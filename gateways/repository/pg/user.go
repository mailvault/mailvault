package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"mailvault/domain/entities"
	"mailvault/domain/user"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
)

type UserRepository struct {
	db DBTX
}

func NewUserRepository(db DBTX) user.Repository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) Create(ctx context.Context, user *entities.User) error {
	query := `
		INSERT INTO users (id, email, auth_provider, auth_provider_id, account_type, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.Exec(ctx, query,
		user.ID,
		user.Email,
		user.AuthProvider,
		user.AuthProviderID,
		user.AccountType,
		user.CreatedAt,
		user.UpdatedAt,
	)

	return err
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*entities.User, error) {
	query := `
		SELECT id, email, auth_provider, auth_provider_id, account_type, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user entities.User
	row := r.db.QueryRow(ctx, query, id)

	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.AuthProvider,
		&user.AuthProviderID,
		&user.AccountType,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*entities.User, error) {
	query := `
		SELECT id, email, auth_provider, auth_provider_id, account_type, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	var user entities.User
	row := r.db.QueryRow(ctx, query, email)

	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.AuthProvider,
		&user.AuthProviderID,
		&user.AccountType,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) GetByAuthProvider(ctx context.Context, provider, providerID string) (*entities.User, error) {
	query := `
		SELECT id, email, auth_provider, auth_provider_id, account_type, created_at, updated_at
		FROM users
		WHERE auth_provider = $1 AND auth_provider_id = $2
	`

	var user entities.User
	row := r.db.QueryRow(ctx, query, provider, providerID)

	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.AuthProvider,
		&user.AuthProviderID,
		&user.AccountType,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) Update(ctx context.Context, user *entities.User) error {
	query := `
		UPDATE users
		SET email = $2, auth_provider = $3, auth_provider_id = $4, account_type = $5, updated_at = $6
		WHERE id = $1
	`

	cmdTag, err := r.db.Exec(ctx, query,
		user.ID,
		user.Email,
		user.AuthProvider,
		user.AuthProviderID,
		user.AccountType,
		user.UpdatedAt,
	)

	if err != nil {
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM users WHERE id = $1`

	cmdTag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *UserRepository) List(ctx context.Context, page, pageSize int) ([]entities.User, int64, error) {
	offset := (page - 1) * pageSize

	// Get total count
	countQuery := `SELECT COUNT(*) FROM users`
	var total int64
	err := r.db.QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get users with pagination
	query := `
		SELECT id, email, auth_provider, auth_provider_id, account_type, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(ctx, query, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []entities.User
	for rows.Next() {
		var user entities.User
		err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.AuthProvider,
			&user.AuthProviderID,
			&user.AccountType,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *UserRepository) SearchUsers(ctx context.Context, page, pageSize int, search, accountType string) ([]entities.User, int64, error) {
	offset := (page - 1) * pageSize
	
	var whereConditions []string
	var args []interface{}
	argIndex := 1

	if search != "" {
		whereConditions = append(whereConditions, "email ILIKE $"+fmt.Sprintf("%d", argIndex))
		args = append(args, "%"+search+"%")
		argIndex++
	}

	if accountType != "" {
		whereConditions = append(whereConditions, "account_type = $"+fmt.Sprintf("%d", argIndex))
		args = append(args, accountType)
		argIndex++
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = "WHERE " + strings.Join(whereConditions, " AND ")
	}

	// Get total count
	countQuery := "SELECT COUNT(*) FROM users " + whereClause
	var total int64
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Add pagination arguments
	args = append(args, pageSize, offset)
	
	// Get users with pagination and filtering
	query := fmt.Sprintf(`
		SELECT id, email, auth_provider, auth_provider_id, account_type, created_at, updated_at
		FROM users
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []entities.User
	for rows.Next() {
		var user entities.User
		err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.AuthProvider,
			&user.AuthProviderID,
			&user.AccountType,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

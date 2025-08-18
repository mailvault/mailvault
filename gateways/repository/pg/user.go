package pg

import (
	"context"
	"database/sql"
	"errors"

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

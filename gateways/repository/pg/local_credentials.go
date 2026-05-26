package pg

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mailvault/mailvault/domain/auth/local"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
)

type LocalCredentialsRepository struct {
	db DBTX
}

func NewLocalCredentialsRepository(db DBTX) local.CredentialsRepo {
	return &LocalCredentialsRepository{db: db}
}

func (r *LocalCredentialsRepository) Create(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	const q = `
		INSERT INTO local_credentials (user_id, password_hash)
		VALUES ($1, $2)
	`
	_, err := r.db.Exec(ctx, q, userID, passwordHash)
	return err
}

func (r *LocalCredentialsRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*local.Credentials, error) {
	const q = `
		SELECT user_id, password_hash, email_confirmed, created_at, updated_at
		FROM local_credentials
		WHERE user_id = $1
	`
	var c local.Credentials
	err := r.db.QueryRow(ctx, q, userID).Scan(
		&c.UserID,
		&c.PasswordHash,
		&c.EmailConfirmed,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return &c, nil
}

func (r *LocalCredentialsRepository) Delete(ctx context.Context, userID uuid.UUID) error {
	const q = `DELETE FROM local_credentials WHERE user_id = $1`
	_, err := r.db.Exec(ctx, q, userID)
	return err
}

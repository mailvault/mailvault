package pg

import (
	"context"
	"database/sql"
	"errors"
	domain "github.com/mailvault/mailvault/domain/domain"
	"github.com/mailvault/mailvault/domain/entities"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
)

type DomainRepository struct {
	db DBTX
}

func NewDomainRepository(db DBTX) domain.Repository {
	return &DomainRepository{
		db: db,
	}
}

func (r *DomainRepository) Create(ctx context.Context, d *entities.Domain) error {
	query := `
		INSERT INTO domains (id, user_id, domain, public_key, api_key, storage_enabled, auto_create_address,
		                     verification_status, verification_token, last_verification_attempt, verification_error, verification_attempts, next_verification_attempt,
		                     created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`

	_, err := r.db.Exec(ctx, query,
		d.ID,
		d.UserID,
		d.Domain,
		d.PublicKey,
		d.APIKey,
		d.StorageEnabled,
		d.AutoCreateAddress,
		d.VerificationStatus,
		d.VerificationToken,
		d.LastVerificationAttempt,
		d.VerificationError,
		d.VerificationAttempts,
		d.NextVerificationAttempt,
		d.CreatedAt,
		d.UpdatedAt,
	)

	return err
}

func (r *DomainRepository) GetByID(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
	query := `
		SELECT id, user_id, domain, public_key, api_key, storage_enabled, auto_create_address,
		       verification_status, verification_token, last_verification_attempt, verification_error, verification_attempts, next_verification_attempt,
		       created_at, updated_at
		FROM domains
		WHERE id = $1
	`

	return r.scanDomain(r.db.QueryRow(ctx, query, id))
}

func (r *DomainRepository) GetByDomain(ctx context.Context, domain string) (*entities.Domain, error) {
	query := `
		SELECT id, user_id, domain, public_key, api_key, storage_enabled, auto_create_address,
		       verification_status, verification_token, last_verification_attempt, verification_error, verification_attempts, next_verification_attempt,
		       created_at, updated_at
		FROM domains
		WHERE domain = $1
	`

	return r.scanDomain(r.db.QueryRow(ctx, query, domain))
}

func (r *DomainRepository) GetByAPIKey(ctx context.Context, apiKey string) (*entities.Domain, error) {
	query := `
		SELECT id, user_id, domain, public_key, api_key, storage_enabled, auto_create_address,
		       verification_status, verification_token, last_verification_attempt, verification_error, verification_attempts, next_verification_attempt,
		       created_at, updated_at
		FROM domains
		WHERE api_key = $1
	`

	return r.scanDomain(r.db.QueryRow(ctx, query, apiKey))
}

func (r *DomainRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*entities.Domain, error) {
	query := `
		SELECT id, user_id, domain, public_key, api_key, storage_enabled, auto_create_address,
		       verification_status, verification_token, last_verification_attempt, verification_error, verification_attempts, next_verification_attempt,
		       created_at, updated_at
		FROM domains
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []*entities.Domain
	for rows.Next() {
		domain, err := r.scanDomainFromRows(rows)
		if err != nil {
			return nil, err
		}
		domains = append(domains, domain)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return domains, nil
}

func (r *DomainRepository) Update(ctx context.Context, d *entities.Domain) error {
	query := `
		UPDATE domains
		SET user_id = $2, domain = $3, public_key = $4, api_key = $5, storage_enabled = $6, auto_create_address = $7,
		    verification_status = $8, verification_token = $9, last_verification_attempt = $10, verification_error = $11, verification_attempts = $12, next_verification_attempt = $13,
		    updated_at = $14
		WHERE id = $1
	`

	cmdTag, err := r.db.Exec(ctx, query,
		d.ID,
		d.UserID,
		d.Domain,
		d.PublicKey,
		d.APIKey,
		d.StorageEnabled,
		d.AutoCreateAddress,
		d.VerificationStatus,
		d.VerificationToken,
		d.LastVerificationAttempt,
		d.VerificationError,
		d.VerificationAttempts,
		d.NextVerificationAttempt,
		d.UpdatedAt,
	)

	if err != nil {
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *DomainRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM domains WHERE id = $1`

	cmdTag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *DomainRepository) scanDomain(row pgx.Row) (*entities.Domain, error) {
	var d entities.Domain
	var lastVerificationAttempt *time.Time
	var nextVerificationAttempt *time.Time
	var verificationError *string
	var verificationToken *string

	err := row.Scan(
		&d.ID,
		&d.UserID,
		&d.Domain,
		&d.PublicKey,
		&d.APIKey,
		&d.StorageEnabled,
		&d.AutoCreateAddress,
		&d.VerificationStatus,
		&verificationToken,
		&lastVerificationAttempt,
		&verificationError,
		&d.VerificationAttempts,
		&nextVerificationAttempt,
		&d.CreatedAt,
		&d.UpdatedAt,
	)

	if lastVerificationAttempt != nil {
		d.LastVerificationAttempt = *lastVerificationAttempt
	}
	if nextVerificationAttempt != nil {
		d.NextVerificationAttempt = *nextVerificationAttempt
	}

	if verificationError != nil {
		d.VerificationError = *verificationError
	}

	if verificationToken != nil {
		d.VerificationToken = *verificationToken
	}

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	return &d, nil
}

func (r *DomainRepository) scanDomainFromRows(rows pgx.Rows) (*entities.Domain, error) {
	var d entities.Domain
	var lastVerificationAttempt *time.Time
	var nextVerificationAttempt *time.Time
	var verificationError *string
	var verificationToken *string

	err := rows.Scan(
		&d.ID,
		&d.UserID,
		&d.Domain,
		&d.PublicKey,
		&d.APIKey,
		&d.StorageEnabled,
		&d.AutoCreateAddress,
		&d.VerificationStatus,
		&verificationToken,
		&lastVerificationAttempt,
		&verificationError,
		&d.VerificationAttempts,
		&nextVerificationAttempt,
		&d.CreatedAt,
		&d.UpdatedAt,
	)

	if lastVerificationAttempt != nil {
		d.LastVerificationAttempt = *lastVerificationAttempt
	}
	if nextVerificationAttempt != nil {
		d.NextVerificationAttempt = *nextVerificationAttempt
	}

	if verificationError != nil {
		d.VerificationError = *verificationError
	}

	if verificationToken != nil {
		d.VerificationToken = *verificationToken
	}

	if err != nil {
		return nil, err
	}

	return &d, nil
}

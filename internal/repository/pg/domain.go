package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"privatemail/domain/entities"
	domain "privatemail/domain/domain"

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
	var webhookConfigJSON []byte
	var err error
	
	if d.WebhookConfig != nil {
		webhookConfigJSON, err = json.Marshal(d.WebhookConfig)
		if err != nil {
			return err
		}
	}
	
	query := `
		INSERT INTO domains (id, user_id, domain, public_key, api_key, verified, webhook_config, storage_enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	
	_, err = r.db.Exec(ctx, query,
		d.ID,
		d.UserID,
		d.Domain,
		d.PublicKey,
		d.APIKey,
		d.Verified,
		webhookConfigJSON,
		d.StorageEnabled,
		d.CreatedAt,
		d.UpdatedAt,
	)
	
	return err
}

func (r *DomainRepository) GetByID(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
	query := `
		SELECT id, user_id, domain, public_key, api_key, verified, webhook_config, storage_enabled, created_at, updated_at
		FROM domains
		WHERE id = $1
	`
	
	return r.scanDomain(r.db.QueryRow(ctx, query, id))
}

func (r *DomainRepository) GetByDomain(ctx context.Context, domain string) (*entities.Domain, error) {
	query := `
		SELECT id, user_id, domain, public_key, api_key, verified, webhook_config, storage_enabled, created_at, updated_at
		FROM domains
		WHERE domain = $1
	`
	
	return r.scanDomain(r.db.QueryRow(ctx, query, domain))
}

func (r *DomainRepository) GetByAPIKey(ctx context.Context, apiKey string) (*entities.Domain, error) {
	query := `
		SELECT id, user_id, domain, public_key, api_key, verified, webhook_config, storage_enabled, created_at, updated_at
		FROM domains
		WHERE api_key = $1
	`
	
	return r.scanDomain(r.db.QueryRow(ctx, query, apiKey))
}

func (r *DomainRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*entities.Domain, error) {
	query := `
		SELECT id, user_id, domain, public_key, api_key, verified, webhook_config, storage_enabled, created_at, updated_at
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
	var webhookConfigJSON []byte
	var err error
	
	if d.WebhookConfig != nil {
		webhookConfigJSON, err = json.Marshal(d.WebhookConfig)
		if err != nil {
			return err
		}
	}
	
	query := `
		UPDATE domains
		SET user_id = $2, domain = $3, public_key = $4, api_key = $5, verified = $6, webhook_config = $7, storage_enabled = $8, updated_at = $9
		WHERE id = $1
	`
	
	cmdTag, err := r.db.Exec(ctx, query,
		d.ID,
		d.UserID,
		d.Domain,
		d.PublicKey,
		d.APIKey,
		d.Verified,
		webhookConfigJSON,
		d.StorageEnabled,
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
	var webhookConfigJSON []byte
	
	err := row.Scan(
		&d.ID,
		&d.UserID,
		&d.Domain,
		&d.PublicKey,
		&d.APIKey,
		&d.Verified,
		&webhookConfigJSON,
		&d.StorageEnabled,
		&d.CreatedAt,
		&d.UpdatedAt,
	)
	
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	
	// Parse webhook config if present
	if len(webhookConfigJSON) > 0 {
		var webhookConfig entities.WebhookConfig
		if err := json.Unmarshal(webhookConfigJSON, &webhookConfig); err != nil {
			return nil, err
		}
		d.WebhookConfig = &webhookConfig
	}
	
	return &d, nil
}

func (r *DomainRepository) scanDomainFromRows(rows pgx.Rows) (*entities.Domain, error) {
	var d entities.Domain
	var webhookConfigJSON []byte
	
	err := rows.Scan(
		&d.ID,
		&d.UserID,
		&d.Domain,
		&d.PublicKey,
		&d.APIKey,
		&d.Verified,
		&webhookConfigJSON,
		&d.StorageEnabled,
		&d.CreatedAt,
		&d.UpdatedAt,
	)
	
	if err != nil {
		return nil, err
	}
	
	// Parse webhook config if present
	if len(webhookConfigJSON) > 0 {
		var webhookConfig entities.WebhookConfig
		if err := json.Unmarshal(webhookConfigJSON, &webhookConfig); err != nil {
			return nil, err
		}
		d.WebhookConfig = &webhookConfig
	}
	
	return &d, nil
}
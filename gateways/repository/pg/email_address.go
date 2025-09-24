package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"mailvault/domain/email"
	"mailvault/domain/entities"
	"mailvault/internal/utils"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
)

type EmailAddressRepository struct {
	db DBTX
}

func NewEmailAddressRepository(db DBTX) email.EmailAddressRepository {
	return &EmailAddressRepository{
		db: db,
	}
}

func (r *EmailAddressRepository) Create(ctx context.Context, emailAddress *entities.EmailAddress) error {
	query := `
		INSERT INTO email_addresses (id, domain_id, local_part, forward_addresses, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.Exec(ctx, query,
		emailAddress.ID,
		emailAddress.DomainID,
		emailAddress.LocalPart,
		emailAddress.ForwardAddresses,
		emailAddress.CreatedAt,
		emailAddress.UpdatedAt,
	)

	return err
}

func (r *EmailAddressRepository) GetByID(ctx context.Context, id uuid.UUID) (*entities.EmailAddress, error) {
	query := `
		SELECT id, domain_id, local_part, forward_addresses, created_at, updated_at
		FROM email_addresses
		WHERE id = $1
	`

	return r.scanEmailAddress(r.db.QueryRow(ctx, query, id))
}

func (r *EmailAddressRepository) GetByDomainID(ctx context.Context, domainID uuid.UUID) ([]*entities.EmailAddress, error) {
	query := `
		SELECT id, domain_id, local_part, forward_addresses, created_at, updated_at
		FROM email_addresses
		WHERE domain_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emailAddresses []*entities.EmailAddress
	for rows.Next() {
		emailAddress, err := r.scanEmailAddressFromRows(rows)
		if err != nil {
			return nil, err
		}
		emailAddresses = append(emailAddresses, emailAddress)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return emailAddresses, nil
}

func (r *EmailAddressRepository) GetByLocalPartAndDomain(ctx context.Context, localPart string, domainID uuid.UUID) (*entities.EmailAddress, error) {
	query := `
		SELECT id, domain_id, local_part, forward_addresses, created_at, updated_at
		FROM email_addresses
		WHERE local_part = $1 AND domain_id = $2
	`

	return r.scanEmailAddress(r.db.QueryRow(ctx, query, localPart, domainID))
}

func (r *EmailAddressRepository) GetByAddress(ctx context.Context, address string) (*entities.EmailAddress, error) {
	// Parse email address to extract local part and domain using safe parsing
	localPart, domainName, err := utils.ParseEmailAddress(address)
	if err != nil {
		return nil, fmt.Errorf("invalid email address format '%s': %w", address, err)
	}

	localPart = strings.ToLower(localPart)
	domainName = strings.ToLower(domainName)

	query := `
		SELECT ea.id, ea.domain_id, ea.local_part, ea.forward_addresses, ea.created_at, ea.updated_at
		FROM email_addresses ea
		JOIN domains d ON ea.domain_id = d.id
		WHERE ea.local_part = $1 AND d.domain = $2
	`

	return r.scanEmailAddress(r.db.QueryRow(ctx, query, localPart, domainName))
}

func (r *EmailAddressRepository) Update(ctx context.Context, emailAddress *entities.EmailAddress) error {
	query := `
		UPDATE email_addresses
		SET domain_id = $2, local_part = $3, forward_addresses = $4, updated_at = $5
		WHERE id = $1
	`

	cmdTag, err := r.db.Exec(ctx, query,
		emailAddress.ID,
		emailAddress.DomainID,
		emailAddress.LocalPart,
		emailAddress.ForwardAddresses,
		emailAddress.UpdatedAt,
	)

	if err != nil {
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *EmailAddressRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM email_addresses WHERE id = $1`

	cmdTag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *EmailAddressRepository) scanEmailAddress(row pgx.Row) (*entities.EmailAddress, error) {
	var e entities.EmailAddress

	err := row.Scan(
		&e.ID,
		&e.DomainID,
		&e.LocalPart,
		&e.ForwardAddresses,
		&e.CreatedAt,
		&e.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	return &e, nil
}

func (r *EmailAddressRepository) scanEmailAddressFromRows(rows pgx.Rows) (*entities.EmailAddress, error) {
	var e entities.EmailAddress

	err := rows.Scan(
		&e.ID,
		&e.DomainID,
		&e.LocalPart,
		&e.ForwardAddresses,
		&e.CreatedAt,
		&e.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &e, nil
}

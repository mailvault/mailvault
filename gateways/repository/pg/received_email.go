package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"mailvault/domain/entities"
	"mailvault/domain/email"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
)

type ReceivedEmailRepository struct {
	db DBTX
}

func NewReceivedEmailRepository(db DBTX) email.ReceivedEmailRepository {
	return &ReceivedEmailRepository{
		db: db,
	}
}

func (r *ReceivedEmailRepository) Create(ctx context.Context, receivedEmail *entities.ReceivedEmail) error {
	query := `
		INSERT INTO received_emails (id, email_address_id, from_address, subject, encrypted_body, received_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING sequence_number
	`
	
	row := r.db.QueryRow(ctx, query,
		receivedEmail.ID,
		receivedEmail.EmailAddressID,
		receivedEmail.FromAddress,
		receivedEmail.Subject,
		receivedEmail.EncryptedBody,
		receivedEmail.ReceivedAt,
	)
	
	return row.Scan(&receivedEmail.SequenceNumber)
}

func (r *ReceivedEmailRepository) GetByID(ctx context.Context, id uuid.UUID) (*entities.ReceivedEmail, error) {
	query := `
		SELECT id, email_address_id, sequence_number, from_address, subject, encrypted_body, received_at
		FROM received_emails
		WHERE id = $1
	`
	
	return r.scanReceivedEmail(r.db.QueryRow(ctx, query, id))
}

func (r *ReceivedEmailRepository) GetByEmailAddressID(ctx context.Context, emailAddressID uuid.UUID, limit, offset int) ([]*entities.ReceivedEmail, error) {
	query := `
		SELECT id, email_address_id, sequence_number, from_address, subject, encrypted_body, received_at
		FROM received_emails
		WHERE email_address_id = $1
		ORDER BY sequence_number DESC
		LIMIT $2 OFFSET $3
	`
	
	rows, err := r.db.Query(ctx, query, emailAddressID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var receivedEmails []*entities.ReceivedEmail
	for rows.Next() {
		receivedEmail, err := r.scanReceivedEmailFromRows(rows)
		if err != nil {
			return nil, err
		}
		receivedEmails = append(receivedEmails, receivedEmail)
	}
	
	if err := rows.Err(); err != nil {
		return nil, err
	}
	
	return receivedEmails, nil
}

func (r *ReceivedEmailRepository) GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int, domain string) ([]*entities.ReceivedEmail, int, error) {
	// Build the query with optional domain filtering
	baseQuery := `
		SELECT re.id, re.email_address_id, re.sequence_number, re.from_address, re.subject, re.encrypted_body, re.received_at
		FROM received_emails re
		JOIN email_addresses ea ON re.email_address_id = ea.id
		JOIN domains d ON ea.domain_id = d.id
		WHERE d.user_id = $1
	`
	
	countQuery := `
		SELECT COUNT(*)
		FROM received_emails re
		JOIN email_addresses ea ON re.email_address_id = ea.id
		JOIN domains d ON ea.domain_id = d.id
		WHERE d.user_id = $1
	`
	
	args := []interface{}{userID}
	argCount := 1
	
	// Add domain filter if provided
	if domain != "" {
		baseQuery += ` AND d.domain = $` + fmt.Sprintf("%d", argCount+1)
		countQuery += ` AND d.domain = $` + fmt.Sprintf("%d", argCount+1)
		args = append(args, domain)
		argCount++
	}
	
	// Add ordering and pagination
	baseQuery += ` ORDER BY re.sequence_number DESC LIMIT $` + fmt.Sprintf("%d", argCount+1) + ` OFFSET $` + fmt.Sprintf("%d", argCount+2)
	args = append(args, limit, offset)
	
	// Get total count first
	var total int
	row := r.db.QueryRow(ctx, countQuery, args[:len(args)-2]...)
	if err := row.Scan(&total); err != nil {
		return nil, 0, err
	}
	
	// Get the emails
	rows, err := r.db.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	var receivedEmails []*entities.ReceivedEmail
	for rows.Next() {
		receivedEmail, err := r.scanReceivedEmailFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		receivedEmails = append(receivedEmails, receivedEmail)
	}
	
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	
	return receivedEmails, total, nil
}

func (r *ReceivedEmailRepository) Count(ctx context.Context, emailAddressID uuid.UUID) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM received_emails
		WHERE email_address_id = $1
	`
	
	var count int64
	row := r.db.QueryRow(ctx, query, emailAddressID)
	err := row.Scan(&count)
	if err != nil {
		return 0, err
	}
	
	return count, nil
}

func (r *ReceivedEmailRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM received_emails WHERE id = $1`
	
	cmdTag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	
	if cmdTag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	
	return nil
}

func (r *ReceivedEmailRepository) scanReceivedEmail(row pgx.Row) (*entities.ReceivedEmail, error) {
	var e entities.ReceivedEmail
	
	err := row.Scan(
		&e.ID,
		&e.EmailAddressID,
		&e.SequenceNumber,
		&e.FromAddress,
		&e.Subject,
		&e.EncryptedBody,
		&e.ReceivedAt,
	)
	
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	
	return &e, nil
}

func (r *ReceivedEmailRepository) scanReceivedEmailFromRows(rows pgx.Rows) (*entities.ReceivedEmail, error) {
	var e entities.ReceivedEmail
	
	err := rows.Scan(
		&e.ID,
		&e.EmailAddressID,
		&e.SequenceNumber,
		&e.FromAddress,
		&e.Subject,
		&e.EncryptedBody,
		&e.ReceivedAt,
	)
	
	if err != nil {
		return nil, err
	}
	
	return &e, nil
}
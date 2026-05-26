package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mailvault/mailvault/domain/email_sending"
	"github.com/mailvault/mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
)

type SentEmailRepository struct {
	db DBTX
}

func NewSentEmailRepository(db DBTX) email_sending.Repository {
	return &SentEmailRepository{
		db: db,
	}
}

func (r *SentEmailRepository) CreateSentEmail(ctx context.Context, sentEmail *entities.SentEmail) error {
	var webhookDataJSON []byte
	var err error

	if sentEmail.WebhookData != nil {
		webhookDataJSON, err = json.Marshal(sentEmail.WebhookData)
		if err != nil {
			return fmt.Errorf("failed to marshal webhook data: %w", err)
		}
	}

	query := `
		INSERT INTO sent_emails (
			id, domain_id, from_address, to_addresses, cc_addresses, bcc_addresses,
			subject, text_body, html_body, message_id, status,
			error_message, retry_count, max_retries,
			created_at, queued_at, sent_at, delivered_at, failed_at, next_retry_at,
			smtp_response, smtp_message_id, webhook_data, webhook_delivered, webhook_attempts
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
			$12, $13, $14, $15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24, $25
		)
	`

	_, err = r.db.Exec(ctx, query,
		sentEmail.ID,
		sentEmail.DomainID,
		sentEmail.FromAddress,
		sentEmail.ToAddresses,
		sentEmail.CCAddresses,
		sentEmail.BCCAddresses,
		sentEmail.Subject,
		sentEmail.TextBody,
		sentEmail.HTMLBody,
		sentEmail.MessageID,
		sentEmail.Status,
		sentEmail.ErrorMessage,
		sentEmail.RetryCount,
		sentEmail.MaxRetries,
		sentEmail.CreatedAt,
		sentEmail.QueuedAt,
		sentEmail.SentAt,
		sentEmail.DeliveredAt,
		sentEmail.FailedAt,
		sentEmail.NextRetryAt,
		sentEmail.SMTPResponse,
		sentEmail.SMTPMessageID,
		webhookDataJSON,
		sentEmail.WebhookDelivered,
		sentEmail.WebhookAttempts,
	)

	return err
}

func (r *SentEmailRepository) GetSentEmail(ctx context.Context, id uuid.UUID) (*entities.SentEmail, error) {
	query := `
		SELECT id, domain_id, from_address, to_addresses, cc_addresses, bcc_addresses,
		       subject, text_body, html_body, message_id, status,
		       error_message, retry_count, max_retries,
		       created_at, queued_at, sent_at, delivered_at, failed_at, next_retry_at,
		       smtp_response, smtp_message_id, webhook_data, webhook_delivered, webhook_attempts
		FROM sent_emails
		WHERE id = $1
	`

	return r.scanSentEmail(r.db.QueryRow(ctx, query, id))
}

func (r *SentEmailRepository) GetSentEmailByMessageID(ctx context.Context, messageID string) (*entities.SentEmail, error) {
	query := `
		SELECT id, domain_id, from_address, to_addresses, cc_addresses, bcc_addresses,
		       subject, text_body, html_body, message_id, status,
		       error_message, retry_count, max_retries,
		       created_at, queued_at, sent_at, delivered_at, failed_at, next_retry_at,
		       smtp_response, smtp_message_id, webhook_data, webhook_delivered, webhook_attempts
		FROM sent_emails
		WHERE message_id = $1
	`

	return r.scanSentEmail(r.db.QueryRow(ctx, query, messageID))
}

func (r *SentEmailRepository) UpdateSentEmail(ctx context.Context, sentEmail *entities.SentEmail) error {
	var webhookDataJSON []byte
	var err error

	if sentEmail.WebhookData != nil {
		webhookDataJSON, err = json.Marshal(sentEmail.WebhookData)
		if err != nil {
			return fmt.Errorf("failed to marshal webhook data: %w", err)
		}
	}

	query := `
		UPDATE sent_emails SET
			domain_id = $2, from_address = $3, to_addresses = $4, cc_addresses = $5, bcc_addresses = $6,
			subject = $7, text_body = $8, html_body = $9, message_id = $10, status = $11,
			error_message = $12, retry_count = $13, max_retries = $14,
			created_at = $15, queued_at = $16, sent_at = $17, delivered_at = $18, failed_at = $19, next_retry_at = $20,
			smtp_response = $21, smtp_message_id = $22, webhook_data = $23, webhook_delivered = $24, webhook_attempts = $25
		WHERE id = $1
	`

	_, err = r.db.Exec(ctx, query,
		sentEmail.ID,
		sentEmail.DomainID,
		sentEmail.FromAddress,
		sentEmail.ToAddresses,
		sentEmail.CCAddresses,
		sentEmail.BCCAddresses,
		sentEmail.Subject,
		sentEmail.TextBody,
		sentEmail.HTMLBody,
		sentEmail.MessageID,
		sentEmail.Status,
		sentEmail.ErrorMessage,
		sentEmail.RetryCount,
		sentEmail.MaxRetries,
		sentEmail.CreatedAt,
		sentEmail.QueuedAt,
		sentEmail.SentAt,
		sentEmail.DeliveredAt,
		sentEmail.FailedAt,
		sentEmail.NextRetryAt,
		sentEmail.SMTPResponse,
		sentEmail.SMTPMessageID,
		webhookDataJSON,
		sentEmail.WebhookDelivered,
		sentEmail.WebhookAttempts,
	)

	return err
}

func (r *SentEmailRepository) UpdateSentEmailStatus(ctx context.Context, id uuid.UUID, status entities.EmailSendStatus, smtpResponse *string, smtpMessageID *string, errorMessage *string) error {
	query := `
		UPDATE sent_emails SET
			status = $2,
			smtp_response = $3,
			smtp_message_id = $4,
			error_message = $5,
			sent_at = CASE WHEN $2 = 'sent'::email_send_status THEN now() ELSE sent_at END,
			delivered_at = CASE WHEN $2 = 'delivered'::email_send_status THEN now() ELSE delivered_at END,
			failed_at = CASE WHEN $2 IN ('failed'::email_send_status, 'bounced'::email_send_status) THEN now() ELSE failed_at END,
			retry_count = CASE WHEN $2 = 'failed'::email_send_status THEN retry_count + 1 ELSE retry_count END
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, id, status, smtpResponse, smtpMessageID, errorMessage)
	return err
}

func (r *SentEmailRepository) DeleteSentEmail(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM sent_emails WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}

func (r *SentEmailRepository) ListSentEmails(ctx context.Context, filters *email_sending.SentEmailFilters) ([]*entities.SentEmail, int64, error) {
	whereClause, args := r.buildWhereClause(filters)

	// Build the count query
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM sent_emails se
		%s
	`, whereClause)

	var total int64
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count sent emails: %w", err)
	}

	// Build the select query
	selectFields := r.buildSelectFields(filters)
	orderClause := r.buildOrderClause(filters)
	limitClause := r.buildLimitClause(filters, len(args))

	query := fmt.Sprintf(`
		SELECT %s
		FROM sent_emails se
		%s
		%s
		%s
	`, selectFields, whereClause, orderClause, limitClause)

	// Add limit and offset to args
	if filters != nil {
		if filters.Limit != nil {
			args = append(args, *filters.Limit)
		}
		if filters.Offset != nil {
			args = append(args, *filters.Offset)
		}
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query sent emails: %w", err)
	}
	defer rows.Close()

	var sentEmails []*entities.SentEmail
	for rows.Next() {
		sentEmail, err := r.scanSentEmail(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan sent email: %w", err)
		}
		sentEmails = append(sentEmails, sentEmail)
	}

	return sentEmails, total, nil
}

func (r *SentEmailRepository) GetSentEmailsPendingSend(ctx context.Context, limit int) ([]*entities.SentEmail, error) {
	query := `
		SELECT id, domain_id, from_address, to_addresses, cc_addresses, bcc_addresses,
		       subject, text_body, html_body, message_id, status,
		       error_message, retry_count, max_retries,
		       created_at, queued_at, sent_at, delivered_at, failed_at, next_retry_at,
		       smtp_response, smtp_message_id, webhook_data, webhook_delivered, webhook_attempts
		FROM sent_emails
		WHERE status IN ('pending', 'queued')
		ORDER BY created_at ASC
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending sent emails: %w", err)
	}
	defer rows.Close()

	var sentEmails []*entities.SentEmail
	for rows.Next() {
		sentEmail, err := r.scanSentEmail(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sent email: %w", err)
		}
		sentEmails = append(sentEmails, sentEmail)
	}

	return sentEmails, nil
}

func (r *SentEmailRepository) GetSentEmailsForRetry(ctx context.Context, limit int) ([]*entities.SentEmail, error) {
	query := `
		SELECT id, domain_id, from_address, to_addresses, cc_addresses, bcc_addresses,
		       subject, text_body, html_body, message_id, status,
		       error_message, retry_count, max_retries,
		       created_at, queued_at, sent_at, delivered_at, failed_at, next_retry_at,
		       smtp_response, smtp_message_id, webhook_data, webhook_delivered, webhook_attempts
		FROM sent_emails
		WHERE status = 'failed'
		  AND retry_count < max_retries
		  AND next_retry_at IS NOT NULL
		  AND next_retry_at <= now()
		ORDER BY next_retry_at ASC
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query retry sent emails: %w", err)
	}
	defer rows.Close()

	var sentEmails []*entities.SentEmail
	for rows.Next() {
		sentEmail, err := r.scanSentEmail(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sent email: %w", err)
		}
		sentEmails = append(sentEmails, sentEmail)
	}

	return sentEmails, nil
}

func (r *SentEmailRepository) GetSentEmailsNeedingWebhook(ctx context.Context, limit int) ([]*entities.SentEmail, error) {
	query := `
		SELECT id, domain_id, from_address, to_addresses, cc_addresses, bcc_addresses,
		       subject, text_body, html_body, message_id, status,
		       error_message, retry_count, max_retries,
		       created_at, queued_at, sent_at, delivered_at, failed_at, next_retry_at,
		       smtp_response, smtp_message_id, webhook_data, webhook_delivered, webhook_attempts
		FROM sent_emails
		WHERE webhook_delivered = false
		  AND status IN ('sent', 'delivered', 'failed', 'bounced', 'cancelled')
		  AND webhook_attempts < 5
		ORDER BY created_at ASC
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query webhook pending sent emails: %w", err)
	}
	defer rows.Close()

	var sentEmails []*entities.SentEmail
	for rows.Next() {
		sentEmail, err := r.scanSentEmail(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sent email: %w", err)
		}
		sentEmails = append(sentEmails, sentEmail)
	}

	return sentEmails, nil
}

func (r *SentEmailRepository) UpdateWebhookDelivery(ctx context.Context, id uuid.UUID, delivered bool, attempts int) error {
	query := `
		UPDATE sent_emails SET
			webhook_delivered = $2,
			webhook_attempts = $3
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, id, delivered, attempts)
	return err
}

func (r *SentEmailRepository) GetSentEmailStats(ctx context.Context, filters *email_sending.SentEmailStatsFilters) (*email_sending.SentEmailStats, error) {
	whereClause, args := r.buildStatsWhereClause(filters)

	query := fmt.Sprintf(`
		SELECT
			COUNT(*) FILTER (WHERE status IN ('sent', 'delivered')) as total_sent,
			COUNT(*) FILTER (WHERE status = 'delivered') as total_delivered,
			COUNT(*) FILTER (WHERE status = 'failed') as total_failed,
			COUNT(*) FILTER (WHERE status = 'bounced') as total_bounced,
			COUNT(*) FILTER (WHERE status = 'pending') as total_pending,
			COUNT(*) FILTER (WHERE status = 'queued') as total_queued
		FROM sent_emails
		%s
	`, whereClause)

	var stats email_sending.SentEmailStats
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&stats.TotalSent,
		&stats.TotalDelivered,
		&stats.TotalFailed,
		&stats.TotalBounced,
		&stats.TotalPending,
		&stats.TotalQueued,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get sent email stats: %w", err)
	}

	// Calculate rates
	totalProcessed := stats.TotalSent + stats.TotalFailed + stats.TotalBounced
	if totalProcessed > 0 {
		stats.DeliveryRate = float64(stats.TotalDelivered) / float64(totalProcessed) * 100
		stats.BounceRate = float64(stats.TotalBounced) / float64(totalProcessed) * 100
	}

	return &stats, nil
}

func (r *SentEmailRepository) GetSentEmailsByDomain(ctx context.Context, domainID uuid.UUID, limit, offset int) ([]*entities.SentEmail, error) {
	query := `
		SELECT id, domain_id, from_address, to_addresses, cc_addresses, bcc_addresses,
		       subject, text_body, html_body, message_id, status,
		       error_message, retry_count, max_retries,
		       created_at, queued_at, sent_at, delivered_at, failed_at, next_retry_at,
		       smtp_response, smtp_message_id, webhook_data, webhook_delivered, webhook_attempts
		FROM sent_emails
		WHERE domain_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, query, domainID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query sent emails by domain: %w", err)
	}
	defer rows.Close()

	var sentEmails []*entities.SentEmail
	for rows.Next() {
		sentEmail, err := r.scanSentEmail(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sent email: %w", err)
		}
		sentEmails = append(sentEmails, sentEmail)
	}

	return sentEmails, nil
}

func (r *SentEmailRepository) CountSentEmailsByDomain(ctx context.Context, domainID uuid.UUID) (int64, error) {
	query := `SELECT COUNT(*) FROM sent_emails WHERE domain_id = $1`

	var count int64
	err := r.db.QueryRow(ctx, query, domainID).Scan(&count)
	return count, err
}

func (r *SentEmailRepository) BulkUpdateStatus(ctx context.Context, emailIDs []uuid.UUID, status entities.EmailSendStatus) error {
	if len(emailIDs) == 0 {
		return nil
	}

	query := `
		UPDATE sent_emails SET
			status = $1,
			sent_at = CASE WHEN $1 = 'sent' THEN now() ELSE sent_at END,
			delivered_at = CASE WHEN $1 = 'delivered' THEN now() ELSE delivered_at END,
			failed_at = CASE WHEN $1 IN ('failed', 'bounced') THEN now() ELSE failed_at END
		WHERE id = ANY($2)
	`

	_, err := r.db.Exec(ctx, query, status, emailIDs)
	return err
}

func (r *SentEmailRepository) GetSentEmailsOlderThan(ctx context.Context, duration time.Duration, limit int) ([]*entities.SentEmail, error) {
	cutoffTime := time.Now().Add(-duration)

	query := `
		SELECT id, domain_id, from_address, to_addresses, cc_addresses, bcc_addresses,
		       subject, text_body, html_body, message_id, status,
		       error_message, retry_count, max_retries,
		       created_at, queued_at, sent_at, delivered_at, failed_at, next_retry_at,
		       smtp_response, smtp_message_id, webhook_data, webhook_delivered, webhook_attempts
		FROM sent_emails
		WHERE created_at < $1
		  AND status IN ('sent', 'delivered', 'failed', 'bounced', 'cancelled')
		ORDER BY created_at ASC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, cutoffTime, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query old sent emails: %w", err)
	}
	defer rows.Close()

	var sentEmails []*entities.SentEmail
	for rows.Next() {
		sentEmail, err := r.scanSentEmail(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sent email: %w", err)
		}
		sentEmails = append(sentEmails, sentEmail)
	}

	return sentEmails, nil
}

func (r *SentEmailRepository) DeleteOldSentEmails(ctx context.Context, olderThan time.Time) (int64, error) {
	query := `
		DELETE FROM sent_emails
		WHERE created_at < $1
		  AND status IN ('sent', 'delivered', 'failed', 'bounced', 'cancelled')
	`

	result, err := r.db.Exec(ctx, query, olderThan)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old sent emails: %w", err)
	}

	return result.RowsAffected(), nil
}

// Helper methods

func (r *SentEmailRepository) scanSentEmail(scanner interface{}) (*entities.SentEmail, error) {
	var sentEmail entities.SentEmail
	var webhookDataJSON sql.NullString
	var toAddresses, ccAddresses, bccAddresses []string

	var err error
	switch s := scanner.(type) {
	case pgx.Row:
		err = s.Scan(
			&sentEmail.ID,
			&sentEmail.DomainID,
			&sentEmail.FromAddress,
			&toAddresses,
			&ccAddresses,
			&bccAddresses,
			&sentEmail.Subject,
			&sentEmail.TextBody,
			&sentEmail.HTMLBody,
			&sentEmail.MessageID,
			&sentEmail.Status,
			&sentEmail.ErrorMessage,
			&sentEmail.RetryCount,
			&sentEmail.MaxRetries,
			&sentEmail.CreatedAt,
			&sentEmail.QueuedAt,
			&sentEmail.SentAt,
			&sentEmail.DeliveredAt,
			&sentEmail.FailedAt,
			&sentEmail.NextRetryAt,
			&sentEmail.SMTPResponse,
			&sentEmail.SMTPMessageID,
			&webhookDataJSON,
			&sentEmail.WebhookDelivered,
			&sentEmail.WebhookAttempts,
		)
	case pgx.Rows:
		err = s.Scan(
			&sentEmail.ID,
			&sentEmail.DomainID,
			&sentEmail.FromAddress,
			&toAddresses,
			&ccAddresses,
			&bccAddresses,
			&sentEmail.Subject,
			&sentEmail.TextBody,
			&sentEmail.HTMLBody,
			&sentEmail.MessageID,
			&sentEmail.Status,
			&sentEmail.ErrorMessage,
			&sentEmail.RetryCount,
			&sentEmail.MaxRetries,
			&sentEmail.CreatedAt,
			&sentEmail.QueuedAt,
			&sentEmail.SentAt,
			&sentEmail.DeliveredAt,
			&sentEmail.FailedAt,
			&sentEmail.NextRetryAt,
			&sentEmail.SMTPResponse,
			&sentEmail.SMTPMessageID,
			&webhookDataJSON,
			&sentEmail.WebhookDelivered,
			&sentEmail.WebhookAttempts,
		)
	default:
		return nil, fmt.Errorf("unsupported scanner type")
	}

	if err != nil {
		if err == pgx.ErrNoRows || err == sql.ErrNoRows {
			return nil, email_sending.ErrSentEmailNotFound
		}
		return nil, err
	}

	// Convert arrays
	sentEmail.ToAddresses = toAddresses
	sentEmail.CCAddresses = ccAddresses
	sentEmail.BCCAddresses = bccAddresses

	// Parse webhook data
	if webhookDataJSON.Valid && webhookDataJSON.String != "" {
		var webhookData map[string]interface{}
		if err := json.Unmarshal([]byte(webhookDataJSON.String), &webhookData); err == nil {
			sentEmail.WebhookData = webhookData
		}
	}

	return &sentEmail, nil
}

func (r *SentEmailRepository) buildWhereClause(filters *email_sending.SentEmailFilters) (string, []interface{}) {
	if filters == nil {
		return "", nil
	}

	var conditions []string
	var args []interface{}
	argIndex := 1

	if filters.DomainID != nil {
		conditions = append(conditions, fmt.Sprintf("se.domain_id = $%d", argIndex))
		args = append(args, *filters.DomainID)
		argIndex++
	}

	if filters.Status != nil {
		conditions = append(conditions, fmt.Sprintf("se.status = $%d", argIndex))
		args = append(args, *filters.Status)
		argIndex++
	}

	if len(filters.Statuses) > 0 {
		conditions = append(conditions, fmt.Sprintf("se.status = ANY($%d)", argIndex))
		args = append(args, filters.Statuses)
		argIndex++
	}

	if filters.FromAddress != nil {
		conditions = append(conditions, fmt.Sprintf("se.from_address ILIKE $%d", argIndex))
		args = append(args, "%"+*filters.FromAddress+"%")
		argIndex++
	}

	if filters.ToAddress != nil {
		conditions = append(conditions, fmt.Sprintf("$%d = ANY(se.to_addresses)", argIndex))
		args = append(args, *filters.ToAddress)
		argIndex++
	}

	if filters.Subject != nil {
		conditions = append(conditions, fmt.Sprintf("se.subject ILIKE $%d", argIndex))
		args = append(args, "%"+*filters.Subject+"%")
		argIndex++
	}

	if filters.MessageID != nil {
		conditions = append(conditions, fmt.Sprintf("se.message_id = $%d", argIndex))
		args = append(args, *filters.MessageID)
		argIndex++
	}

	if filters.CreatedFrom != nil {
		conditions = append(conditions, fmt.Sprintf("se.created_at >= $%d", argIndex))
		args = append(args, *filters.CreatedFrom)
		argIndex++
	}

	if filters.CreatedTo != nil {
		conditions = append(conditions, fmt.Sprintf("se.created_at <= $%d", argIndex))
		args = append(args, *filters.CreatedTo)
		argIndex++
	}

	if filters.SentFrom != nil {
		conditions = append(conditions, fmt.Sprintf("se.sent_at >= $%d", argIndex))
		args = append(args, *filters.SentFrom)
		argIndex++
	}

	if filters.SentTo != nil {
		conditions = append(conditions, fmt.Sprintf("se.sent_at <= $%d", argIndex))
		args = append(args, *filters.SentTo)
	}

	if filters.HasError != nil && *filters.HasError {
		conditions = append(conditions, "se.error_message IS NOT NULL")
	}

	if filters.NeedsRetry != nil && *filters.NeedsRetry {
		conditions = append(conditions, "se.status = 'failed' AND se.retry_count < se.max_retries AND se.next_retry_at <= now()")
	}

	if filters.PendingWebhook != nil && *filters.PendingWebhook {
		conditions = append(conditions, "se.webhook_delivered = false AND se.status IN ('sent', 'delivered', 'failed', 'bounced', 'cancelled')")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	return whereClause, args
}

func (r *SentEmailRepository) buildSelectFields(filters *email_sending.SentEmailFilters) string {
	fields := []string{
		"se.id", "se.domain_id", "se.from_address", "se.to_addresses", "se.cc_addresses", "se.bcc_addresses",
		"se.subject", "se.message_id", "se.status",
		"se.error_message", "se.retry_count", "se.max_retries",
		"se.created_at", "se.queued_at", "se.sent_at", "se.delivered_at", "se.failed_at", "se.next_retry_at",
		"se.smtp_response", "se.smtp_message_id", "se.webhook_delivered", "se.webhook_attempts",
	}

	if filters == nil || filters.IncludeBody {
		fields = append(fields, "se.text_body", "se.html_body")
	} else {
		fields = append(fields, "NULL as text_body", "NULL as html_body")
	}

	if filters == nil || filters.IncludeWebhook {
		fields = append(fields, "se.webhook_data")
	} else {
		fields = append(fields, "NULL as webhook_data")
	}

	return strings.Join(fields, ", ")
}

func (r *SentEmailRepository) buildOrderClause(filters *email_sending.SentEmailFilters) string {
	if filters == nil {
		return "ORDER BY se.created_at DESC"
	}

	orderBy := "created_at"
	if filters.OrderBy != "" {
		switch filters.OrderBy {
		case "created_at", "sent_at", "status", "from_address", "subject":
			orderBy = filters.OrderBy
		}
	}

	orderDir := "DESC"
	if filters.OrderDir == "asc" {
		orderDir = "ASC"
	}

	return fmt.Sprintf("ORDER BY se.%s %s", orderBy, orderDir)
}

func (r *SentEmailRepository) buildLimitClause(filters *email_sending.SentEmailFilters, argOffset int) string {
	if filters == nil {
		return ""
	}

	var clauses []string
	if filters.Limit != nil {
		clauses = append(clauses, fmt.Sprintf("LIMIT $%d", argOffset+1))
	}
	if filters.Offset != nil {
		offsetArg := argOffset + 1
		if filters.Limit != nil {
			offsetArg = argOffset + 2
		}
		clauses = append(clauses, fmt.Sprintf("OFFSET $%d", offsetArg))
	}

	return strings.Join(clauses, " ")
}

func (r *SentEmailRepository) buildStatsWhereClause(filters *email_sending.SentEmailStatsFilters) (string, []interface{}) {
	if filters == nil {
		return "", nil
	}

	var conditions []string
	var args []interface{}
	argIndex := 1

	if filters.DomainID != nil {
		conditions = append(conditions, fmt.Sprintf("domain_id = $%d", argIndex))
		args = append(args, *filters.DomainID)
		argIndex++
	}

	if filters.Since != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIndex))
		args = append(args, *filters.Since)
		argIndex++
	}

	if filters.Until != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIndex))
		args = append(args, *filters.Until)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	return whereClause, args
}
package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"

	"mailvault/domain/email"
	"mailvault/domain/entities"

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
		SELECT re.id, re.email_address_id, re.sequence_number, re.from_address, re.subject, re.encrypted_body, re.received_at,
		       svs.is_quarantined, svs.final_action, svs.spam_score, svs.content_verdict,
		       svs.spf_result, svs.spf_mechanism, svs.dkim_valid, svs.dkim_domain, svs.dkim_selector,
		       svs.dmarc_result, svs.dmarc_policy, svs.dmarc_alignment_spf, svs.dmarc_alignment_dkim,
		       svs.reputation_score, svs.is_blacklisted, svs.sender_ip, svs.sender_domain, svs.verified_at
		FROM received_emails re
		LEFT JOIN email_addresses ea ON re.email_address_id = ea.id
		LEFT JOIN smtp_verification_stats svs ON (
		    svs.email_address_id = ea.id
		    AND svs.from_address = re.from_address
		    AND ABS(EXTRACT(EPOCH FROM (svs.verified_at - re.received_at))) < 300
		)
		WHERE re.id = $1
	`

	return r.scanReceivedEmailWithSMTPStatsFromRow(r.db.QueryRow(ctx, query, id))
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
	// Build the query with optional domain filtering and SMTP verification stats
	baseQuery := `
		SELECT re.id, re.email_address_id, re.sequence_number, re.from_address, re.subject, re.encrypted_body,
		       d.domain as domain_name, CONCAT(ea.local_part, '@', d.domain) as email_address, re.received_at,
		       svs.is_quarantined, svs.final_action, svs.spam_score, svs.content_verdict,
		       svs.spf_result, svs.spf_mechanism, svs.dkim_valid, svs.dkim_domain, svs.dkim_selector,
		       svs.dmarc_result, svs.dmarc_policy, svs.dmarc_alignment_spf, svs.dmarc_alignment_dkim,
		       svs.reputation_score, svs.is_blacklisted, svs.sender_ip, svs.sender_domain, svs.verified_at
		FROM received_emails re
		JOIN email_addresses ea ON re.email_address_id = ea.id
		JOIN domains d ON ea.domain_id = d.id
		LEFT JOIN smtp_verification_stats svs ON (
		    svs.domain_id = d.id
		    AND svs.email_address_id = ea.id
		    AND svs.from_address = re.from_address
		    AND ABS(EXTRACT(EPOCH FROM (svs.verified_at - re.received_at))) < 300
		)
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
		receivedEmail, err := r.scanReceivedEmailWithDetailsAndSMTPStatsFromRows(rows)
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

func (r *ReceivedEmailRepository) scanReceivedEmailWithDetailsFromRows(rows pgx.Rows) (*entities.ReceivedEmail, error) {
	var e entities.ReceivedEmail

	err := rows.Scan(
		&e.ID,
		&e.EmailAddressID,
		&e.SequenceNumber,
		&e.FromAddress,
		&e.Subject,
		&e.EncryptedBody,
		&e.DomainName,
		&e.EmailAddress,
		&e.ReceivedAt,
	)

	if err != nil {
		return nil, err
	}

	return &e, nil
}

func (r *ReceivedEmailRepository) scanReceivedEmailWithDetailsAndSMTPStatsFromRows(rows pgx.Rows) (*entities.ReceivedEmail, error) {
	var e entities.ReceivedEmail
	var smtpStats entities.SMTPVerificationStat

	// Use sql.Null types for optional SMTP verification fields that can be NULL
	var isQuarantined, dkimValid, dmarcAlignmentSpf, dmarcAlignmentDkim, isBlacklisted sql.NullBool
	var finalAction, contentVerdict, spfResult, spfMechanism, dkimDomain, dkimSelector sql.NullString
	var dmarcResult, dmarcPolicy, senderIP, senderDomain sql.NullString
	var spamScore, reputationScore sql.NullFloat64
	var verifiedAt sql.NullTime

	err := rows.Scan(
		&e.ID,
		&e.EmailAddressID,
		&e.SequenceNumber,
		&e.FromAddress,
		&e.Subject,
		&e.EncryptedBody,
		&e.DomainName,
		&e.EmailAddress,
		&e.ReceivedAt,
		// SMTP verification stats fields (can be NULL)
		&isQuarantined,
		&finalAction,
		&spamScore,
		&contentVerdict,
		&spfResult,
		&spfMechanism,
		&dkimValid,
		&dkimDomain,
		&dkimSelector,
		&dmarcResult,
		&dmarcPolicy,
		&dmarcAlignmentSpf,
		&dmarcAlignmentDkim,
		&reputationScore,
		&isBlacklisted,
		&senderIP,
		&senderDomain,
		&verifiedAt,
	)

	if err != nil {
		return nil, err
	}

	// If we have SMTP verification data, populate the struct
	if finalAction.Valid {
		// DomainID will need to be fetched from the JOIN - for now using placeholder
		smtpStats.EmailAddressID = *e.EmailAddressID
		smtpStats.IsQuarantined = isQuarantined.Bool
		smtpStats.FinalAction = finalAction.String
		smtpStats.SpamScore = spamScore.Float64
		smtpStats.ContentVerdict = contentVerdict.String
		smtpStats.SPFResult = spfResult.String
		smtpStats.SPFMechanism = spfMechanism.String
		smtpStats.DKIMValid = dkimValid.Bool
		smtpStats.DKIMDomain = dkimDomain.String
		smtpStats.DKIMSelector = dkimSelector.String
		smtpStats.DMARCResult = dmarcResult.String
		smtpStats.DMARCPolicy = dmarcPolicy.String
		smtpStats.DMARCAlignmentSPF = dmarcAlignmentSpf.Bool
		smtpStats.DMARCAlignmentDKIM = dmarcAlignmentDkim.Bool
		smtpStats.ReputationScore = reputationScore.Float64
		smtpStats.IsBlacklisted = isBlacklisted.Bool
		smtpStats.FromAddress = e.FromAddress
		if senderIP.Valid {
			smtpStats.SenderIP = net.ParseIP(senderIP.String)
		}
		smtpStats.SenderDomain = senderDomain.String
		if verifiedAt.Valid {
			smtpStats.VerifiedAt = verifiedAt.Time
		}

		e.SMTPVerification = &smtpStats
	}

	return &e, nil
}

func (r *ReceivedEmailRepository) scanReceivedEmailWithSMTPStatsFromRow(row pgx.Row) (*entities.ReceivedEmail, error) {
	var e entities.ReceivedEmail
	var smtpStats entities.SMTPVerificationStat

	// Use sql.Null types for optional SMTP verification fields that can be NULL
	var isQuarantined, dkimValid, dmarcAlignmentSpf, dmarcAlignmentDkim, isBlacklisted sql.NullBool
	var finalAction, contentVerdict, spfResult, spfMechanism, dkimDomain, dkimSelector sql.NullString
	var dmarcResult, dmarcPolicy, senderIP, senderDomain sql.NullString
	var spamScore, reputationScore sql.NullFloat64
	var verifiedAt sql.NullTime

	err := row.Scan(
		&e.ID,
		&e.EmailAddressID,
		&e.SequenceNumber,
		&e.FromAddress,
		&e.Subject,
		&e.EncryptedBody,
		&e.ReceivedAt,
		// SMTP verification stats fields (can be NULL)
		&isQuarantined,
		&finalAction,
		&spamScore,
		&contentVerdict,
		&spfResult,
		&spfMechanism,
		&dkimValid,
		&dkimDomain,
		&dkimSelector,
		&dmarcResult,
		&dmarcPolicy,
		&dmarcAlignmentSpf,
		&dmarcAlignmentDkim,
		&reputationScore,
		&isBlacklisted,
		&senderIP,
		&senderDomain,
		&verifiedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	// If we have SMTP verification data, populate the struct
	if finalAction.Valid {
		smtpStats.EmailAddressID = *e.EmailAddressID
		smtpStats.IsQuarantined = isQuarantined.Bool
		smtpStats.FinalAction = finalAction.String
		smtpStats.SpamScore = spamScore.Float64
		smtpStats.ContentVerdict = contentVerdict.String
		smtpStats.SPFResult = spfResult.String
		smtpStats.SPFMechanism = spfMechanism.String
		smtpStats.DKIMValid = dkimValid.Bool
		smtpStats.DKIMDomain = dkimDomain.String
		smtpStats.DKIMSelector = dkimSelector.String
		smtpStats.DMARCResult = dmarcResult.String
		smtpStats.DMARCPolicy = dmarcPolicy.String
		smtpStats.DMARCAlignmentSPF = dmarcAlignmentSpf.Bool
		smtpStats.DMARCAlignmentDKIM = dmarcAlignmentDkim.Bool
		smtpStats.ReputationScore = reputationScore.Float64
		smtpStats.IsBlacklisted = isBlacklisted.Bool
		smtpStats.FromAddress = e.FromAddress
		if senderIP.Valid {
			smtpStats.SenderIP = net.ParseIP(senderIP.String)
		}
		smtpStats.SenderDomain = senderDomain.String
		if verifiedAt.Valid {
			smtpStats.VerifiedAt = verifiedAt.Time
		}

		e.SMTPVerification = &smtpStats
	}

	return &e, nil
}

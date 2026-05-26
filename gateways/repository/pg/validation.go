package pg

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mailvault/mailvault/domain/validation"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
)

type ValidationRepository struct {
	db DBTX
}

func NewValidationRepository(db DBTX) validation.Repository {
	return &ValidationRepository{db: db}
}


// CreateValidationRecord creates a new validation record
func (r *ValidationRepository) CreateValidationRecord(ctx context.Context, record *validation.ValidationRecord) error {
	query := `
		INSERT INTO domain_validation_records (
			id, domain_id, validation_type, status, details,
			started_at, completed_at, error_message, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	detailsJSON, err := json.Marshal(record.Details)
	if err != nil {
		return fmt.Errorf("failed to marshal validation details: %w", err)
	}

	_, err = r.db.Exec(ctx, query,
		record.ID,
		record.DomainID,
		record.ValidationType,
		record.Status,
		detailsJSON,
		record.StartedAt,
		record.CompletedAt,
		record.ErrorMessage,
		record.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create validation record: %w", err)
	}

	return nil
}

// GetValidationRecordByID retrieves a validation record by ID
func (r *ValidationRepository) GetValidationRecordByID(ctx context.Context, id uuid.UUID) (*validation.ValidationRecord, error) {
	query := `
		SELECT id, domain_id, validation_type, status, details,
			   started_at, completed_at, error_message, created_at
		FROM domain_validation_records
		WHERE id = $1
	`

	row := r.db.QueryRow(ctx, query, id)
	return r.scanValidationRecord(row)
}

// GetValidationRecordsByDomainID retrieves all validation records for a domain
func (r *ValidationRepository) GetValidationRecordsByDomainID(ctx context.Context, domainID uuid.UUID) ([]*validation.ValidationRecord, error) {
	query := `
		SELECT id, domain_id, validation_type, status, details,
			   started_at, completed_at, error_message, created_at
		FROM domain_validation_records
		WHERE domain_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, domainID)
	if err != nil {
		return nil, fmt.Errorf("failed to query validation records: %w", err)
	}
	defer rows.Close()

	return r.scanValidationRecords(rows)
}

// GetValidationRecordsByDomainIDAndType retrieves validation records for a domain and type
func (r *ValidationRepository) GetValidationRecordsByDomainIDAndType(ctx context.Context, domainID uuid.UUID, validationType validation.ValidationType) ([]*validation.ValidationRecord, error) {
	query := `
		SELECT id, domain_id, validation_type, status, details,
			   started_at, completed_at, error_message, created_at
		FROM domain_validation_records
		WHERE domain_id = $1 AND validation_type = $2
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, domainID, validationType)
	if err != nil {
		return nil, fmt.Errorf("failed to query validation records: %w", err)
	}
	defer rows.Close()

	return r.scanValidationRecords(rows)
}

// UpdateValidationRecord updates an existing validation record
func (r *ValidationRepository) UpdateValidationRecord(ctx context.Context, record *validation.ValidationRecord) error {
	query := `
		UPDATE domain_validation_records
		SET status = $2, details = $3, completed_at = $4, error_message = $5
		WHERE id = $1
	`

	detailsJSON, err := json.Marshal(record.Details)
	if err != nil {
		return fmt.Errorf("failed to marshal validation details: %w", err)
	}

	_, err = r.db.Exec(ctx, query,
		record.ID,
		record.Status,
		detailsJSON,
		record.CompletedAt,
		record.ErrorMessage,
	)

	if err != nil {
		return fmt.Errorf("failed to update validation record: %w", err)
	}

	return nil
}

// DeleteValidationRecord deletes a validation record
func (r *ValidationRepository) DeleteValidationRecord(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM domain_validation_records WHERE id = $1`

	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete validation record: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("validation record not found")
	}

	return nil
}

// GetLatestValidationRecord gets the most recent validation record for a domain and type
func (r *ValidationRepository) GetLatestValidationRecord(ctx context.Context, domainID uuid.UUID, validationType validation.ValidationType) (*validation.ValidationRecord, error) {
	query := `
		SELECT id, domain_id, validation_type, status, details,
			   started_at, completed_at, error_message, created_at
		FROM domain_validation_records
		WHERE domain_id = $1 AND validation_type = $2
		ORDER BY created_at DESC
		LIMIT 1
	`

	row := r.db.QueryRow(ctx, query, domainID, validationType)
	return r.scanValidationRecord(row)
}

// GetValidationRecordsByTimeRange retrieves validation records within a time range
func (r *ValidationRepository) GetValidationRecordsByTimeRange(ctx context.Context, start, end time.Time) ([]*validation.ValidationRecord, error) {
	query := `
		SELECT id, domain_id, validation_type, status, details,
			   started_at, completed_at, error_message, created_at
		FROM domain_validation_records
		WHERE created_at >= $1 AND created_at <= $2
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query validation records: %w", err)
	}
	defer rows.Close()

	return r.scanValidationRecords(rows)
}

// GetValidationRecordsByStatus retrieves validation records by status
func (r *ValidationRepository) GetValidationRecordsByStatus(ctx context.Context, status validation.ValidationRecordStatus) ([]*validation.ValidationRecord, error) {
	query := `
		SELECT id, domain_id, validation_type, status, details,
			   started_at, completed_at, error_message, created_at
		FROM domain_validation_records
		WHERE status = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to query validation records: %w", err)
	}
	defer rows.Close()

	return r.scanValidationRecords(rows)
}

// CleanupOldValidationRecords deletes validation records older than the specified time
func (r *ValidationRepository) CleanupOldValidationRecords(ctx context.Context, olderThan time.Time) (int64, error) {
	query := `DELETE FROM domain_validation_records WHERE created_at < $1`

	result, err := r.db.Exec(ctx, query, olderThan)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old validation records: %w", err)
	}

	return result.RowsAffected(), nil
}

// UpdateDomainVerificationStatus updates the verification status of a domain
func (r *ValidationRepository) UpdateDomainVerificationStatus(ctx context.Context, domainID uuid.UUID, status validation.VerificationStatus) error {
	query := `
		UPDATE domains
		SET verification_status = $2, updated_at = now()
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, domainID, status)
	if err != nil {
		return fmt.Errorf("failed to update domain verification status: %w", err)
	}

	return nil
}

// UpdateDomainVerificationAttempt updates domain verification attempt information
func (r *ValidationRepository) UpdateDomainVerificationAttempt(ctx context.Context, domainID uuid.UUID, attempts int, lastAttempt time.Time, nextAttempt *time.Time, errorMsg *string) error {
	query := `
		UPDATE domains
		SET verification_attempts = $2,
		    last_verification_attempt = $3,
		    next_verification_attempt = $4,
		    verification_error = $5,
		    updated_at = now()
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, domainID, attempts, lastAttempt, nextAttempt, errorMsg)
	if err != nil {
		return fmt.Errorf("failed to update domain verification attempt: %w", err)
	}

	return nil
}

// GetDomainsNeedingVerification retrieves domains that need verification
func (r *ValidationRepository) GetDomainsNeedingVerification(ctx context.Context, limit int) ([]*validation.DomainValidationInfo, error) {
	query := `
		SELECT id, user_id, domain, verification_status, verification_token,
			   verification_attempts, last_verification_attempt, next_verification_attempt,
			   verification_error, created_at, updated_at
		FROM domains
		WHERE verification_status IN ('pending', 'failed')
		  AND (next_verification_attempt IS NULL OR next_verification_attempt <= now())
		ORDER BY verification_attempts ASC, created_at ASC
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query domains needing verification: %w", err)
	}
	defer rows.Close()

	return r.scanDomainValidationInfos(rows)
}

// GetDomainsPendingVerification retrieves domains with pending verification status
func (r *ValidationRepository) GetDomainsPendingVerification(ctx context.Context, limit int) ([]*validation.DomainValidationInfo, error) {
	query := `
		SELECT id, user_id, domain, verification_status, verification_token,
			   verification_attempts, last_verification_attempt, next_verification_attempt,
			   verification_error, created_at, updated_at
		FROM domains
		WHERE verification_status = 'pending'
		ORDER BY created_at ASC
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending domains: %w", err)
	}
	defer rows.Close()

	return r.scanDomainValidationInfos(rows)
}

// GetDomainsReadyForRetry retrieves domains ready for retry verification
func (r *ValidationRepository) GetDomainsReadyForRetry(ctx context.Context, limit int) ([]*validation.DomainValidationInfo, error) {
	query := `
		SELECT id, user_id, domain, verification_status, verification_token,
			   verification_attempts, last_verification_attempt, next_verification_attempt,
			   verification_error, created_at, updated_at
		FROM domains
		WHERE verification_status IN ('failed', 'expired')
		  AND (next_verification_attempt IS NULL OR next_verification_attempt <= now())
		ORDER BY verification_attempts ASC, next_verification_attempt ASC
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query domains ready for retry: %w", err)
	}
	defer rows.Close()

	return r.scanDomainValidationInfos(rows)
}

// GetValidationStats retrieves validation statistics
func (r *ValidationRepository) GetValidationStats(ctx context.Context, domainID *uuid.UUID, timeRange *validation.TimeRange) (*validation.ValidationStats, error) {
	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argIndex := 1

	if domainID != nil {
		whereClause += fmt.Sprintf(" AND domain_id = $%d", argIndex)
		args = append(args, *domainID)
		argIndex++
	}

	if timeRange != nil {
		whereClause += fmt.Sprintf(" AND created_at >= $%d AND created_at <= $%d", argIndex, argIndex+1)
		args = append(args, timeRange.Start, timeRange.End)
	}

	query := fmt.Sprintf(`
		SELECT
			COUNT(*) as total_attempts,
			COUNT(CASE WHEN status = 'success' THEN 1 END) as successful_attempts,
			COUNT(CASE WHEN status = 'failed' THEN 1 END) as failed_attempts,
			COUNT(CASE WHEN status = 'timeout' THEN 1 END) as timeout_attempts,
			COUNT(CASE WHEN status = 'error' THEN 1 END) as error_attempts,
			AVG(EXTRACT(EPOCH FROM (completed_at - started_at))) as avg_duration_seconds
		FROM domain_validation_records
		%s
	`, whereClause)

	row := r.db.QueryRow(ctx, query, args...)

	var stats validation.ValidationStats
	var totalAttempts, successful, failed, timeout, error int64
	var avgDurationSeconds *float64

	err := row.Scan(&totalAttempts, &successful, &failed, &timeout, &error, &avgDurationSeconds)
	if err != nil {
		return nil, fmt.Errorf("failed to scan validation stats: %w", err)
	}

	stats.TotalAttempts = totalAttempts
	stats.SuccessfulAttempts = successful
	stats.FailedAttempts = failed
	stats.TimeoutAttempts = timeout
	stats.ErrorAttempts = error
	stats.SuccessRate = validation.CalculateSuccessRate(successful, totalAttempts)

	if avgDurationSeconds != nil {
		stats.AverageTime = time.Duration(*avgDurationSeconds * float64(time.Second))
	}

	// Initialize maps
	stats.ByType = make(map[validation.ValidationType]*validation.TypeStats)
	stats.ByStatus = make(map[validation.ValidationRecordStatus]int64)

	// Get stats by type
	typeQuery := fmt.Sprintf(`
		SELECT
			validation_type,
			COUNT(*) as total,
			COUNT(CASE WHEN status = 'success' THEN 1 END) as successful,
			AVG(EXTRACT(EPOCH FROM (completed_at - started_at))) as avg_duration
		FROM domain_validation_records
		%s
		GROUP BY validation_type
	`, whereClause)

	typeRows, err := r.db.Query(ctx, typeQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query type stats: %w", err)
	}
	defer typeRows.Close()

	for typeRows.Next() {
		var validationType validation.ValidationType
		var total, successful int64
		var avgDuration *float64

		err := typeRows.Scan(&validationType, &total, &successful, &avgDuration)
		if err != nil {
			return nil, fmt.Errorf("failed to scan type stats: %w", err)
		}

		typeStats := &validation.TypeStats{
			TotalAttempts:      total,
			SuccessfulAttempts: successful,
			SuccessRate:        validation.CalculateSuccessRate(successful, total),
		}

		if avgDuration != nil {
			typeStats.AverageTime = time.Duration(*avgDuration * float64(time.Second))
		}

		stats.ByType[validationType] = typeStats
	}

	// Get stats by status
	statusQuery := fmt.Sprintf(`
		SELECT status, COUNT(*)
		FROM domain_validation_records
		%s
		GROUP BY status
	`, whereClause)

	statusRows, err := r.db.Query(ctx, statusQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query status stats: %w", err)
	}
	defer statusRows.Close()

	for statusRows.Next() {
		var status validation.ValidationRecordStatus
		var count int64

		err := statusRows.Scan(&status, &count)
		if err != nil {
			return nil, fmt.Errorf("failed to scan status stats: %w", err)
		}

		stats.ByStatus[status] = count
	}

	return &stats, nil
}

// Helper methods

func (r *ValidationRepository) scanValidationRecord(row pgx.Row) (*validation.ValidationRecord, error) {
	var record validation.ValidationRecord
	var detailsJSON []byte

	err := row.Scan(
		&record.ID,
		&record.DomainID,
		&record.ValidationType,
		&record.Status,
		&detailsJSON,
		&record.StartedAt,
		&record.CompletedAt,
		&record.ErrorMessage,
		&record.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("validation record not found")
		}
		return nil, fmt.Errorf("failed to scan validation record: %w", err)
	}

	if len(detailsJSON) > 0 {
		err = json.Unmarshal(detailsJSON, &record.Details)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal validation details: %w", err)
		}
	}

	return &record, nil
}

func (r *ValidationRepository) scanValidationRecords(rows pgx.Rows) ([]*validation.ValidationRecord, error) {
	var records []*validation.ValidationRecord

	for rows.Next() {
		var record validation.ValidationRecord
		var detailsJSON []byte

		err := rows.Scan(
			&record.ID,
			&record.DomainID,
			&record.ValidationType,
			&record.Status,
			&detailsJSON,
			&record.StartedAt,
			&record.CompletedAt,
			&record.ErrorMessage,
			&record.CreatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan validation record: %w", err)
		}

		if len(detailsJSON) > 0 {
			err = json.Unmarshal(detailsJSON, &record.Details)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal validation details: %w", err)
			}
		}

		records = append(records, &record)
	}

	return records, nil
}

func (r *ValidationRepository) scanDomainValidationInfos(rows pgx.Rows) ([]*validation.DomainValidationInfo, error) {
	var domains []*validation.DomainValidationInfo

	for rows.Next() {
		var domain validation.DomainValidationInfo

		err := rows.Scan(
			&domain.ID,
			&domain.UserID,
			&domain.Domain,
			&domain.VerificationStatus,
			&domain.VerificationToken,
			&domain.VerificationAttempts,
			&domain.LastVerificationAttempt,
			&domain.NextVerificationAttempt,
			&domain.VerificationError,
			&domain.CreatedAt,
			&domain.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan domain validation info: %w", err)
		}

		domains = append(domains, &domain)
	}

	return domains, nil
}
package validation

import (
	"context"
	"time"

	"github.com/gofrs/uuid/v5"
)

// Repository defines the interface for validation data operations
type Repository interface {
	// ValidationRecord operations
	CreateValidationRecord(ctx context.Context, record *ValidationRecord) error
	GetValidationRecordByID(ctx context.Context, id uuid.UUID) (*ValidationRecord, error)
	GetValidationRecordsByDomainID(ctx context.Context, domainID uuid.UUID) ([]*ValidationRecord, error)
	GetValidationRecordsByDomainIDAndType(ctx context.Context, domainID uuid.UUID, validationType ValidationType) ([]*ValidationRecord, error)
	UpdateValidationRecord(ctx context.Context, record *ValidationRecord) error
	DeleteValidationRecord(ctx context.Context, id uuid.UUID) error

	// Get latest validation record for a domain and type
	GetLatestValidationRecord(ctx context.Context, domainID uuid.UUID, validationType ValidationType) (*ValidationRecord, error)

	// Get validation records within time range
	GetValidationRecordsByTimeRange(ctx context.Context, start, end time.Time) ([]*ValidationRecord, error)

	// Get validation records by status
	GetValidationRecordsByStatus(ctx context.Context, status ValidationRecordStatus) ([]*ValidationRecord, error)

	// Cleanup old validation records
	CleanupOldValidationRecords(ctx context.Context, olderThan time.Time) (int64, error)

	// Domain validation status operations
	UpdateDomainVerificationStatus(ctx context.Context, domainID uuid.UUID, status VerificationStatus) error
	UpdateDomainVerificationAttempt(ctx context.Context, domainID uuid.UUID, attempts int, lastAttempt time.Time, nextAttempt *time.Time, errorMsg *string) error

	// Get domains needing verification
	GetDomainsNeedingVerification(ctx context.Context, limit int) ([]*DomainValidationInfo, error)
	GetDomainsPendingVerification(ctx context.Context, limit int) ([]*DomainValidationInfo, error)
	GetDomainsReadyForRetry(ctx context.Context, limit int) ([]*DomainValidationInfo, error)

	// Statistics
	GetValidationStats(ctx context.Context, domainID *uuid.UUID, timeRange *TimeRange) (*ValidationStats, error)
}

// DomainValidationInfo contains domain information needed for validation
type DomainValidationInfo struct {
	ID                      uuid.UUID          `json:"id"`
	UserID                  uuid.UUID          `json:"user_id"`
	Domain                  string             `json:"domain"`
	VerificationStatus      VerificationStatus `json:"verification_status"`
	VerificationToken       *string            `json:"verification_token,omitempty"`
	VerificationAttempts    int                `json:"verification_attempts"`
	LastVerificationAttempt *time.Time         `json:"last_verification_attempt,omitempty"`
	NextVerificationAttempt *time.Time         `json:"next_verification_attempt,omitempty"`
	VerificationError       *string            `json:"verification_error,omitempty"`
	CreatedAt               time.Time          `json:"created_at"`
	UpdatedAt               time.Time          `json:"updated_at"`
}

// ValidationStats contains statistics about domain validations
type ValidationStats struct {
	TotalAttempts      int64                            `json:"total_attempts"`
	SuccessfulAttempts int64                            `json:"successful_attempts"`
	FailedAttempts     int64                            `json:"failed_attempts"`
	TimeoutAttempts    int64                            `json:"timeout_attempts"`
	ErrorAttempts      int64                            `json:"error_attempts"`
	SuccessRate        float64                          `json:"success_rate"`
	AverageTime        time.Duration                    `json:"average_time"`
	ByType             map[ValidationType]*TypeStats    `json:"by_type"`
	ByStatus           map[ValidationRecordStatus]int64 `json:"by_status"`
}

// TypeStats contains statistics for a specific validation type
type TypeStats struct {
	TotalAttempts      int64         `json:"total_attempts"`
	SuccessfulAttempts int64         `json:"successful_attempts"`
	SuccessRate        float64       `json:"success_rate"`
	AverageTime        time.Duration `json:"average_time"`
}

// TimeRange represents a time range for queries
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// FilterOptions provides filtering options for validation queries
type FilterOptions struct {
	DomainID       *uuid.UUID              `json:"domain_id,omitempty"`
	ValidationType *ValidationType         `json:"validation_type,omitempty"`
	Status         *ValidationRecordStatus `json:"status,omitempty"`
	TimeRange      *TimeRange              `json:"time_range,omitempty"`
	Limit          int                     `json:"limit,omitempty"`
	Offset         int                     `json:"offset,omitempty"`
	OrderBy        string                  `json:"order_by,omitempty"` // e.g., "created_at DESC"
}

// GetDomainValidationInfo returns basic validation info for a domain
func (dvi *DomainValidationInfo) GetTXTRecord() string {
	if dvi.VerificationToken == nil {
		return ""
	}
	return "mailvault-verification=" + *dvi.VerificationToken
}

// IsVerified returns true if the domain is verified
func (dvi *DomainValidationInfo) IsVerified() bool {
	return dvi.VerificationStatus == VerificationStatusVerified
}

// IsPending returns true if verification is pending
func (dvi *DomainValidationInfo) IsPending() bool {
	return dvi.VerificationStatus == VerificationStatusPending ||
		dvi.VerificationStatus == VerificationStatusValidating
}

// CanRetry returns true if the domain can be retried for verification
func (dvi *DomainValidationInfo) CanRetry() bool {
	if dvi.IsVerified() {
		return false
	}

	if dvi.NextVerificationAttempt == nil {
		return true
	}

	return time.Now().After(*dvi.NextVerificationAttempt)
}

// CalculateSuccessRate calculates the success rate from total and successful attempts
func CalculateSuccessRate(successful, total int64) float64 {
	if total == 0 {
		return 0.0
	}
	return float64(successful) / float64(total) * 100.0
}

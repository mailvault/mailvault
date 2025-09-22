package validation

import (
	"encoding/json"
	"time"

	"mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
)

// Use VerificationStatus from entities package
type VerificationStatus = entities.VerificationStatus

const (
	VerificationStatusPending    = entities.VerificationStatusPending
	VerificationStatusValidating = entities.VerificationStatusValidating
	VerificationStatusVerified   = entities.VerificationStatusVerified
	VerificationStatusFailed     = entities.VerificationStatusFailed
	VerificationStatusExpired    = entities.VerificationStatusExpired
)


// ValidationType represents the type of validation being performed
type ValidationType string

const (
	ValidationTypeMXRecord      ValidationType = "mx_record"
	ValidationTypeTXTRecord     ValidationType = "txt_record"
	ValidationTypeOwnership     ValidationType = "ownership"
	ValidationTypeFullValidation ValidationType = "full_validation"
)

// IsValid checks if the validation type is valid
func (vt ValidationType) IsValid() bool {
	switch vt {
	case ValidationTypeMXRecord, ValidationTypeTXTRecord,
		 ValidationTypeOwnership, ValidationTypeFullValidation:
		return true
	default:
		return false
	}
}

// String returns string representation of ValidationType
func (vt ValidationType) String() string {
	return string(vt)
}

// ValidationRecordStatus represents the status of a specific validation record
type ValidationRecordStatus string

const (
	ValidationRecordStatusPending ValidationRecordStatus = "pending"
	ValidationRecordStatusRunning ValidationRecordStatus = "running"
	ValidationRecordStatusSuccess ValidationRecordStatus = "success"
	ValidationRecordStatusFailed  ValidationRecordStatus = "failed"
	ValidationRecordStatusTimeout ValidationRecordStatus = "timeout"
	ValidationRecordStatusError   ValidationRecordStatus = "error"
)

// IsValid checks if the validation record status is valid
func (vrs ValidationRecordStatus) IsValid() bool {
	switch vrs {
	case ValidationRecordStatusPending, ValidationRecordStatusRunning,
		 ValidationRecordStatusSuccess, ValidationRecordStatusFailed,
		 ValidationRecordStatusTimeout, ValidationRecordStatusError:
		return true
	default:
		return false
	}
}

// String returns string representation of ValidationRecordStatus
func (vrs ValidationRecordStatus) String() string {
	return string(vrs)
}

// ValidationRecord represents a single validation attempt for a domain
type ValidationRecord struct {
	ID              uuid.UUID              `json:"id" db:"id"`
	DomainID        uuid.UUID              `json:"domain_id" db:"domain_id"`
	ValidationType  ValidationType         `json:"validation_type" db:"validation_type"`
	Status          ValidationRecordStatus `json:"status" db:"status"`
	Details         ValidationDetails      `json:"details" db:"details"`
	StartedAt       time.Time              `json:"started_at" db:"started_at"`
	CompletedAt     *time.Time             `json:"completed_at,omitempty" db:"completed_at"`
	ErrorMessage    *string                `json:"error_message,omitempty" db:"error_message"`
	CreatedAt       time.Time              `json:"created_at" db:"created_at"`
}

// ValidationDetails contains detailed information about a validation attempt
type ValidationDetails struct {
	// MX Record validation details
	ExpectedMXServers []string `json:"expected_mx_servers,omitempty"`
	FoundMXRecords    []MXRecord `json:"found_mx_records,omitempty"`
	MXValidationPassed bool     `json:"mx_validation_passed,omitempty"`

	// TXT Record validation details
	ExpectedTXTRecord string   `json:"expected_txt_record,omitempty"`
	FoundTXTRecords   []string `json:"found_txt_records,omitempty"`
	TXTValidationPassed bool   `json:"txt_validation_passed,omitempty"`

	// General validation details
	DNSServer    string        `json:"dns_server,omitempty"`
	QueryTime    time.Duration `json:"query_time,omitempty"`
	RetryCount   int           `json:"retry_count,omitempty"`
	ErrorDetails string        `json:"error_details,omitempty"`
}

// MXRecord represents an MX DNS record
type MXRecord struct {
	Host     string `json:"host"`
	Priority int    `json:"priority"`
}

// ValidationJob represents a validation job to be processed by workers
type ValidationJob struct {
	ID         uuid.UUID      `json:"id"`
	DomainID   uuid.UUID      `json:"domain_id"`
	DomainName string         `json:"domain_name"`
	Type       ValidationType `json:"type"`
	Priority   int            `json:"priority"` // Higher number = higher priority
	CreatedAt  time.Time      `json:"created_at"`
	Attempts   int            `json:"attempts"`
	LastError  string         `json:"last_error,omitempty"`
}

// ValidationConfig holds configuration for domain validation
type ValidationConfig struct {
	// MX validation settings
	ExpectedMXServers []string      `json:"expected_mx_servers"`
	MXCheckTimeout    time.Duration `json:"mx_check_timeout"`

	// TXT validation settings
	TXTRecordPrefix   string        `json:"txt_record_prefix"`
	TXTCheckTimeout   time.Duration `json:"txt_check_timeout"`

	// General settings
	MaxRetries        int           `json:"max_retries"`
	RetryDelay        time.Duration `json:"retry_delay"`
	DNSServer         string        `json:"dns_server"`
	ValidationTimeout time.Duration `json:"validation_timeout"`

	// Token settings
	TokenExpiry       time.Duration `json:"token_expiry"`
}

// DefaultValidationConfig returns a default validation configuration
func DefaultValidationConfig() ValidationConfig {
	return ValidationConfig{
		ExpectedMXServers: []string{"mail.mailvault.sh", "mail2.mailvault.sh"},
		MXCheckTimeout:    30 * time.Second,
		TXTRecordPrefix:   "mailvault-verification",
		TXTCheckTimeout:   30 * time.Second,
		MaxRetries:        3,
		RetryDelay:        5 * time.Minute,
		DNSServer:         "8.8.8.8:53",
		ValidationTimeout: 60 * time.Second,
		TokenExpiry:       24 * time.Hour,
	}
}

// IsComplete returns true if the validation record is completed (success, failed, timeout, or error)
func (vr *ValidationRecord) IsComplete() bool {
	return vr.Status == ValidationRecordStatusSuccess ||
		   vr.Status == ValidationRecordStatusFailed ||
		   vr.Status == ValidationRecordStatusTimeout ||
		   vr.Status == ValidationRecordStatusError
}

// IsSuccessful returns true if the validation record was successful
func (vr *ValidationRecord) IsSuccessful() bool {
	return vr.Status == ValidationRecordStatusSuccess
}

// Duration returns the duration of the validation attempt
func (vr *ValidationRecord) Duration() time.Duration {
	if vr.CompletedAt == nil {
		return time.Since(vr.StartedAt)
	}
	return vr.CompletedAt.Sub(vr.StartedAt)
}

// MarshalJSON custom marshaling for ValidationDetails to handle the embedded JSON in database
func (vd ValidationDetails) MarshalJSON() ([]byte, error) {
	type Alias ValidationDetails
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(&vd),
	})
}

// UnmarshalJSON custom unmarshaling for ValidationDetails
func (vd *ValidationDetails) UnmarshalJSON(data []byte) error {
	type Alias ValidationDetails
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(vd),
	}
	return json.Unmarshal(data, &aux)
}

// CreateValidationJob creates a new validation job
func CreateValidationJob(domainID uuid.UUID, domainName string, validationType ValidationType, priority int) *ValidationJob {
	return &ValidationJob{
		ID:         uuid.Must(uuid.NewV4()),
		DomainID:   domainID,
		DomainName: domainName,
		Type:       validationType,
		Priority:   priority,
		Attempts:   0,
		CreatedAt:  time.Now().UTC(),
	}
}

// CalculateRetryDelay calculates the delay before the next retry attempt
func CalculateRetryDelay(attempt int, baseDelay time.Duration) time.Duration {
	if attempt <= 0 {
		return baseDelay
	}

	// Exponential backoff: 2^(attempt-1) * baseDelay
	// Cap at 24 hours
	multiplier := 1 << (attempt - 1)
	delay := time.Duration(multiplier) * baseDelay

	maxDelay := 24 * time.Hour
	if delay > maxDelay {
		return maxDelay
	}

	return delay
}
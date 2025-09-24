package entities

import (
	"time"

	"github.com/gofrs/uuid/v5"
)

// VerificationStatus represents the status of domain verification
type VerificationStatus string

const (
	VerificationStatusPending    VerificationStatus = "pending"
	VerificationStatusValidating VerificationStatus = "validating"
	VerificationStatusVerified   VerificationStatus = "verified"
	VerificationStatusFailed     VerificationStatus = "failed"
	VerificationStatusExpired    VerificationStatus = "expired"
)

// IsValid checks if the verification status is valid
func (v VerificationStatus) IsValid() bool {
	switch v {
	case VerificationStatusPending, VerificationStatusValidating, VerificationStatusVerified, VerificationStatusFailed, VerificationStatusExpired:
		return true
	default:
		return false
	}
}

type Domain struct {
	ID                  uuid.UUID      `json:"id" db:"id"`
	UserID              uuid.UUID      `json:"user_id" db:"user_id"`
	Domain              string         `json:"domain" db:"domain"`
	PublicKey           string         `json:"public_key" db:"public_key"`
	EncryptedPrivateKey *string        `json:"-" db:"encrypted_private_key"` // Encrypted with user-derived key, never exposed in API
	APIKey              string         `json:"api_key" db:"api_key"`
	WebhookConfig       *WebhookConfig `json:"webhook_config,omitempty" db:"webhook_config"`
	StorageEnabled      bool           `json:"storage_enabled" db:"storage_enabled"`
	AutoCreateAddress   bool           `json:"auto_create_address" db:"auto_create_address"`
	// Validation fields
	VerificationStatus      VerificationStatus `json:"verification_status" db:"verification_status"`
	VerificationToken       string             `json:"verification_token,omitempty" db:"verification_token"`
	LastVerificationAttempt time.Time          `json:"last_verification_attempt,omitempty" db:"last_verification_attempt"`
	VerificationError       string             `json:"verification_error,omitempty" db:"verification_error"`
	VerificationAttempts    int                `json:"verification_attempts" db:"verification_attempts"`
	NextVerificationAttempt time.Time          `json:"next_verification_attempt,omitempty" db:"next_verification_attempt"`
	CreatedAt               time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt               time.Time          `json:"updated_at" db:"updated_at"`
}

func (d *Domain) IsValid() bool {
	return d.Domain != "" && d.PublicKey != "" && d.APIKey != "" && d.UserID != uuid.Nil && d.VerificationStatus.IsValid()
}

func (d *Domain) GetFullDomain() string {
	return d.Domain
}

func (d *Domain) HasWebhook() bool {
	return d.WebhookConfig != nil && d.WebhookConfig.Enabled && d.WebhookConfig.IsValid()
}

func (d *Domain) IsVerified() bool {
	return d.VerificationStatus == VerificationStatusVerified
}

func (d *Domain) IsPendingVerification() bool {
	return d.VerificationStatus == VerificationStatusPending ||
		d.VerificationStatus == VerificationStatusValidating
}

func (d *Domain) IsVerificationFailed() bool {
	return d.VerificationStatus == VerificationStatusFailed ||
		d.VerificationStatus == VerificationStatusExpired
}

func (d *Domain) CanRetryVerification() bool {
	return d.IsVerificationFailed() &&
		(d.NextVerificationAttempt.IsZero() || time.Now().After(d.NextVerificationAttempt))
}

func (d *Domain) GetTXTRecord() string {
	if d.VerificationToken == "" {
		return ""
	}
	return "mailvault-verification=" + d.VerificationToken
}

func (d *Domain) NeedsVerification() bool {
	return !d.IsVerified() && (d.VerificationStatus == VerificationStatusPending || d.CanRetryVerification())
}

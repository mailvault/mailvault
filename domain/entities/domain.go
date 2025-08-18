package entities

import (
	"time"

	"github.com/gofrs/uuid/v5"
)

type Domain struct {
	ID               uuid.UUID      `json:"id" db:"id"`
	UserID           uuid.UUID      `json:"user_id" db:"user_id"`
	Domain           string         `json:"domain" db:"domain"`
	PublicKey        string         `json:"public_key" db:"public_key"`
	EncryptedPrivateKey *string     `json:"-" db:"encrypted_private_key"` // Encrypted with user-derived key, never exposed in API
	APIKey           string         `json:"api_key" db:"api_key"`
	Verified         bool           `json:"verified" db:"verified"`
	WebhookConfig    *WebhookConfig `json:"webhook_config,omitempty" db:"webhook_config"`
	StorageEnabled   bool           `json:"storage_enabled" db:"storage_enabled"`
	AutoCreateAddress bool          `json:"auto_create_address" db:"auto_create_address"`
	CreatedAt        time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at" db:"updated_at"`
}

func (d *Domain) IsValid() bool {
	return d.Domain != "" && d.PublicKey != "" && d.APIKey != "" && d.UserID != uuid.Nil
}

func (d *Domain) GetFullDomain() string {
	return d.Domain
}

func (d *Domain) HasWebhook() bool {
	return d.WebhookConfig != nil && d.WebhookConfig.Enabled && d.WebhookConfig.IsValid()
}

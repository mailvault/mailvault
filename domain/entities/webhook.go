package entities

import (
	"encoding/json"
	"time"

	"github.com/gofrs/uuid/v5"
)

type WebhookConfig struct {
	URL     string            `json:"url"`
	Secret  string            `json:"secret,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Enabled bool              `json:"enabled"`
}

func (w *WebhookConfig) IsValid() bool {
	return w.URL != ""
}

func (w *WebhookConfig) ToJSON() ([]byte, error) {
	return json.Marshal(w)
}

func WebhookConfigFromJSON(data []byte) (*WebhookConfig, error) {
	var config WebhookConfig
	err := json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// WebhookEvent represents a webhook event received from email providers
type WebhookEvent struct {
	ID               uuid.UUID              `json:"id"`
	SentEmailID      uuid.UUID              `json:"sent_email_id"`
	ProviderID       uuid.UUID              `json:"provider_id"`
	EventType        string                 `json:"event_type"`
	Status           string                 `json:"status"`
	Recipient        string                 `json:"recipient"`
	Reason           string                 `json:"reason,omitempty"`
	Description      string                 `json:"description,omitempty"`
	ProviderData     map[string]interface{} `json:"provider_data,omitempty"`
	WebhookTimestamp time.Time              `json:"webhook_timestamp"`
	ProcessedAt      time.Time              `json:"processed_at"`
	CreatedAt        time.Time              `json:"created_at"`
}

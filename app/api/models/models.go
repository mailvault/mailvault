package models

import (
	"time"
)

// User represents a user in the system
// @Description User account information
type User struct {
	ID           string    `json:"id" example:"123e4567-e89b-12d3-a456-426614174000"`
	Email        string    `json:"email" example:"user@example.com"`
	AuthProvider string    `json:"auth_provider" example:"supabase"`
	CreatedAt    time.Time `json:"created_at" example:"2023-01-01T00:00:00Z"`
	UpdatedAt    time.Time `json:"updated_at" example:"2023-01-01T00:00:00Z"`
}

// Domain represents a domain configuration
// @Description Domain configuration for email services
type Domain struct {
	ID             string               `json:"id" example:"123e4567-e89b-12d3-a456-426614174000"`
	UserID         string               `json:"user_id" example:"123e4567-e89b-12d3-a456-426614174000"`
	Domain         string               `json:"domain" example:"example.com"`
	PublicKey      string               `json:"public_key" example:"-----BEGIN PUBLIC KEY-----...-----END PUBLIC KEY-----"`
	APIKey         string               `json:"api_key" example:"sk_test_1234567890"`
	Verified       bool                 `json:"verified" example:"true"`
	WebhookConfig  *WebhookConfigResult `json:"webhook_config,omitempty"`
	StorageEnabled bool                 `json:"storage_enabled" example:"true"`
	CreatedAt      time.Time            `json:"created_at" example:"2023-01-01T00:00:00Z"`
	UpdatedAt      time.Time            `json:"updated_at" example:"2023-01-01T00:00:00Z"`
}

// EmailAddress represents an email address configuration
// @Description Email address configuration within a domain
type EmailAddress struct {
	ID               string    `json:"id" example:"123e4567-e89b-12d3-a456-426614174000"`
	DomainID         string    `json:"domain_id" example:"123e4567-e89b-12d3-a456-426614174000"`
	LocalPart        string    `json:"local_part" example:"support"`
	FullAddress      string    `json:"full_address" example:"support@example.com"`
	IsCatchAll       bool      `json:"is_catch_all" example:"false"`
	ForwardAddresses []string  `json:"forward_addresses" example:"[\"user@company.com\", \"admin@company.com\"]"`
	CreatedAt        time.Time `json:"created_at" example:"2023-01-01T00:00:00Z"`
	UpdatedAt        time.Time `json:"updated_at" example:"2023-01-01T00:00:00Z"`
}

// ReceivedEmail represents a received email
// @Description Email received by the system
type ReceivedEmail struct {
	ID             string    `json:"id" example:"123e4567-e89b-12d3-a456-426614174000"`
	SequenceNumber int       `json:"sequence_number" example:"1"`
	FromAddress    string    `json:"from_address" example:"sender@external.com"`
	Subject        string    `json:"subject" example:"Important Message"`
	EncryptedBody  string    `json:"encrypted_body" example:"encrypted_content_hash"`
	ReceivedAt     time.Time `json:"received_at" example:"2023-01-01T00:00:00Z"`
}

// Authentication types
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email" example:"user@example.com"`
	Password string `json:"password" validate:"required,min=8" example:"password123"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email" example:"user@example.com"`
	Password string `json:"password" validate:"required" example:"password123"`
}

type UserResult struct {
	ID           string `json:"id" example:"123e4567-e89b-12d3-a456-426614174000"`
	Email        string `json:"email" example:"user@example.com"`
	AuthProvider string `json:"auth_provider" example:"supabase"`
	CreatedAt    string `json:"created_at" example:"2023-01-01T00:00:00Z"`
}

type AuthResponse struct {
	Token string      `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	User  *UserResult `json:"user"`
}

// Domain management types
type CreateDomainRequest struct {
	Domain         string                `json:"domain" validate:"required" example:"example.com"`
	PublicKey      string                `json:"public_key" validate:"required" example:"-----BEGIN PUBLIC KEY-----...-----END PUBLIC KEY-----"`
	WebhookConfig  *WebhookConfigRequest `json:"webhook_config,omitempty"`
	StorageEnabled *bool                 `json:"storage_enabled,omitempty" example:"true"`
}

type UpdateDomainRequest struct {
	PublicKey      *string               `json:"public_key,omitempty" example:"-----BEGIN PUBLIC KEY-----...-----END PUBLIC KEY-----"`
	Verified       *bool                 `json:"verified,omitempty" example:"true"`
	WebhookConfig  *WebhookConfigRequest `json:"webhook_config,omitempty"`
	StorageEnabled *bool                 `json:"storage_enabled,omitempty" example:"false"`
}

type DomainResult struct {
	ID             string               `json:"id" example:"123e4567-e89b-12d3-a456-426614174000"`
	Domain         string               `json:"domain" example:"example.com"`
	PublicKey      string               `json:"public_key" example:"-----BEGIN PUBLIC KEY-----...-----END PUBLIC KEY-----"`
	APIKey         string               `json:"api_key" example:"sk_test_1234567890"`
	Verified       bool                 `json:"verified" example:"true"`
	WebhookConfig  *WebhookConfigResult `json:"webhook_config,omitempty"`
	StorageEnabled bool                 `json:"storage_enabled" example:"true"`
	CreatedAt      string               `json:"created_at" example:"2023-01-01T00:00:00Z"`
	UpdatedAt      string               `json:"updated_at" example:"2023-01-01T00:00:00Z"`
}

// Email address management types
type CreateEmailRequest struct {
	LocalPart        string   `json:"local_part" validate:"required" example:"info"`
	IsCatchAll       bool     `json:"is_catch_all" example:"false"`
	ForwardAddresses []string `json:"forward_addresses,omitempty" example:"[\"forward@example.com\"]"`
}

type UpdateEmailRequest struct {
	IsCatchAll       *bool    `json:"is_catch_all,omitempty" example:"true"`
	ForwardAddresses []string `json:"forward_addresses,omitempty" example:"[\"forward1@example.com\", \"forward2@example.com\"]"`
}

type EmailAddressResult struct {
	ID               string   `json:"id" example:"123e4567-e89b-12d3-a456-426614174000"`
	DomainID         string   `json:"domain_id" example:"123e4567-e89b-12d3-a456-426614174001"`
	LocalPart        string   `json:"local_part" example:"info"`
	FullAddress      string   `json:"full_address" example:"info@example.com"`
	IsCatchAll       bool     `json:"is_catch_all" example:"false"`
	ForwardAddresses []string `json:"forward_addresses" example:"[\"forward@example.com\"]"`
	CreatedAt        string   `json:"created_at" example:"2023-01-01T00:00:00Z"`
	UpdatedAt        string   `json:"updated_at" example:"2023-01-01T00:00:00Z"`
}

type ReceivedEmailResult struct {
	ID             string `json:"id" example:"123e4567-e89b-12d3-a456-426614174000"`
	SequenceNumber int    `json:"sequence_number" example:"1"`
	FromAddress    string `json:"from_address" example:"sender@external.com"`
	Subject        string `json:"subject" example:"Important Message"`
	EncryptedBody  string `json:"encrypted_body" example:"encrypted_content_hash"`
	ReceivedAt     string `json:"received_at" example:"2023-01-01T00:00:00Z"`
}

// Webhook configuration types
type WebhookConfigRequest struct {
	URL     string            `json:"url" validate:"required" example:"https://api.example.com/webhook"`
	Secret  string            `json:"secret,omitempty" example:"webhook_secret_key"`
	Headers map[string]string `json:"headers,omitempty" example:"{\"X-Custom-Header\": \"value\"}"`
	Enabled bool              `json:"enabled" example:"true"`
}

type WebhookConfigResult struct {
	URL     string            `json:"url" example:"https://api.example.com/webhook"`
	Secret  string            `json:"secret,omitempty" example:"webhook_secret_key"`
	Headers map[string]string `json:"headers,omitempty" example:"{\"X-Custom-Header\": \"value\"}"`
	Enabled bool              `json:"enabled" example:"true"`
}

// Send email types
type SendEmailRequest struct {
	From     string   `json:"from" validate:"required,email" example:"sender@example.com"`
	To       []string `json:"to" validate:"required,min=1" example:"[\"recipient@example.com\"]"`
	CC       []string `json:"cc,omitempty" example:"[\"cc@example.com\"]"`
	BCC      []string `json:"bcc,omitempty" example:"[\"bcc@example.com\"]"`
	Subject  string   `json:"subject" validate:"required" example:"Hello from MailVault"`
	TextBody string   `json:"text_body,omitempty" example:"Plain text email body"`
	HTMLBody string   `json:"html_body,omitempty" example:"<p>HTML email body</p>"`
}

type SendEmailResponse struct {
	MessageID string `json:"message_id" example:"pm_1234567890abcdef"`
	Status    string `json:"status" example:"queued"`
}

// Common response types
type ErrorResponseBody struct {
	Error string `json:"error" example:"Invalid request"`
}

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Pagination struct {
		Limit  int `json:"limit" example:"10"`
		Offset int `json:"offset" example:"0"`
		Total  int `json:"total,omitempty" example:"100"`
	} `json:"pagination"`
}
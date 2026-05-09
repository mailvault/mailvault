package models

import (
	"time"

	"github.com/gofrs/uuid/v5"
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
	ID                string               `json:"id" example:"123e4567-e89b-12d3-a456-426614174000"`
	UserID            string               `json:"user_id" example:"123e4567-e89b-12d3-a456-426614174000"`
	Domain            string               `json:"domain" example:"example.com"`
	PublicKey         string               `json:"public_key" example:"-----BEGIN PUBLIC KEY-----...-----END PUBLIC KEY-----"`
	APIKey            string               `json:"api_key" example:"sk_test_1234567890"`
	Verified          bool                 `json:"verified" example:"true"`
	WebhookConfig     *WebhookConfigResult `json:"webhook_config,omitempty"`
	StorageEnabled    bool                 `json:"storage_enabled" example:"true"`
	AutoCreateAddress bool                 `json:"auto_create_address" example:"false"`
	CreatedAt         time.Time            `json:"created_at" example:"2023-01-01T00:00:00Z"`
	UpdatedAt         time.Time            `json:"updated_at" example:"2023-01-01T00:00:00Z"`
}

// EmailAddress represents an email address configuration
// @Description Email address configuration within a domain
type EmailAddress struct {
	ID                string    `json:"id" example:"123e4567-e89b-12d3-a456-426614174000"`
	DomainID          string    `json:"domain_id" example:"123e4567-e89b-12d3-a456-426614174000"`
	LocalPart         string    `json:"local_part" example:"support"`
	FullAddress       string    `json:"full_address" example:"support@example.com"`
	ForwardAddresses  []string  `json:"forward_addresses" example:"[\"user@company.com\", \"admin@company.com\"]"`
	ForwardingEnabled bool      `json:"forwarding_enabled" example:"false"`
	CreatedAt         time.Time `json:"created_at" example:"2023-01-01T00:00:00Z"`
	UpdatedAt         time.Time `json:"updated_at" example:"2023-01-01T00:00:00Z"`
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
	Email    string `json:"email" validate:"required,email,max=254" example:"user@example.com"`
	Password string `json:"password" validate:"required,min=8,max=128" example:"password123"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email,max=254" example:"user@example.com"`
	Password string `json:"password" validate:"required,max=128" example:"password123"`
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
	Domain            string                `json:"domain" validate:"required,domain,min=1,max=253" example:"example.com"`
	PublicKey         string                `json:"public_key" validate:"required,public_key,min=100" example:"-----BEGIN PUBLIC KEY-----...-----END PUBLIC KEY-----"`
	WebhookConfig     *WebhookConfigRequest `json:"webhook_config,omitempty"`
	StorageEnabled    *bool                 `json:"storage_enabled,omitempty" example:"true"`
	AutoCreateAddress *bool                 `json:"auto_create_address,omitempty" example:"false"`
}

type UpdateDomainRequest struct {
	PublicKey         *string               `json:"public_key,omitempty" example:"-----BEGIN PUBLIC KEY-----...-----END PUBLIC KEY-----"`
	Verified          *bool                 `json:"verified,omitempty" example:"true"`
	WebhookConfig     *WebhookConfigRequest `json:"webhook_config,omitempty"`
	StorageEnabled    *bool                 `json:"storage_enabled,omitempty" example:"false"`
	AutoCreateAddress *bool                 `json:"auto_create_address,omitempty" example:"false"`
}

type DomainResult struct {
	ID                string               `json:"id" example:"123e4567-e89b-12d3-a456-426614174000"`
	Domain            string               `json:"domain" example:"example.com"`
	PublicKey         string               `json:"public_key" example:"-----BEGIN PUBLIC KEY-----...-----END PUBLIC KEY-----"`
	APIKey            string               `json:"api_key" example:"sk_test_1234567890"`
	Verified          bool                 `json:"verified" example:"true"`
	WebhookConfig     *WebhookConfigResult `json:"webhook_config,omitempty"`
	StorageEnabled    bool                 `json:"storage_enabled" example:"true"`
	AutoCreateAddress bool                 `json:"auto_create_address" example:"false"`
	CreatedAt         string               `json:"created_at" example:"2023-01-01T00:00:00Z"`
	UpdatedAt         string               `json:"updated_at" example:"2023-01-01T00:00:00Z"`
}

// Email address management types
type CreateEmailRequest struct {
	LocalPart         string   `json:"local_part" validate:"required,min=1,max=64,safe_string" example:"info"`
	ForwardAddresses  []string `json:"forward_addresses,omitempty" validate:"omitempty,email_list,max=10" example:"[\"forward@example.com\"]"`
	ForwardingEnabled bool     `json:"forwarding_enabled" example:"false"`
}

type UpdateEmailRequest struct {
	ForwardAddresses  []string `json:"forward_addresses,omitempty" validate:"omitempty,email_list,max=10" example:"[\"forward1@example.com\", \"forward2@example.com\"]"`
	ForwardingEnabled *bool    `json:"forwarding_enabled,omitempty" example:"true"`
}

type EmailAddressResult struct {
	ID                string   `json:"id" example:"123e4567-e89b-12d3-a456-426614174000"`
	DomainID          string   `json:"domain_id" example:"123e4567-e89b-12d3-a456-426614174001"`
	LocalPart         string   `json:"local_part" example:"info"`
	FullAddress       string   `json:"full_address" example:"info@example.com"`
	ForwardAddresses  []string `json:"forward_addresses" example:"[\"forward@example.com\"]"`
	ForwardingEnabled bool     `json:"forwarding_enabled" example:"false"`
	CreatedAt         string   `json:"created_at" example:"2023-01-01T00:00:00Z"`
	UpdatedAt         string   `json:"updated_at" example:"2023-01-01T00:00:00Z"`
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
	URL     string            `json:"url" validate:"required,url,max=2048" example:"https://api.example.com/webhook"`
	Secret  string            `json:"secret,omitempty" validate:"omitempty,min=16,max=256,safe_string" example:"webhook_secret_key"`
	Headers map[string]string `json:"headers,omitempty" validate:"omitempty,dive,keys,safe_string,endkeys,safe_string" example:"{\"X-Custom-Header\": \"value\"}"`
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
	From     string   `json:"from" validate:"required,email,max=254" example:"sender@example.com"`
	To       []string `json:"to" validate:"required,min=1,email_list,max=100" example:"[\"recipient@example.com\"]"`
	CC       []string `json:"cc,omitempty" validate:"omitempty,email_list,max=50" example:"[\"cc@example.com\"]"`
	BCC      []string `json:"bcc,omitempty" validate:"omitempty,email_list,max=50" example:"[\"bcc@example.com\"]"`
	Subject  string   `json:"subject" validate:"required,min=1,max=998" example:"Hello from MailVault"`
	TextBody string   `json:"text_body,omitempty" validate:"omitempty,max=1048576" example:"Plain text email body"`
	HTMLBody string   `json:"html_body,omitempty" validate:"omitempty,max=1048576" example:"<p>HTML email body</p>"`
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

// Domain validation types

// DomainValidationResponse represents the response for domain validation requests
// @Description Domain validation response with instructions and status
type DomainValidationResponse struct {
	DomainID          uuid.UUID              `json:"domain_id" example:"123e4567-e89b-12d3-a456-426614174000"`
	DomainName        string                 `json:"domain_name" example:"example.com"`
	Status            string                 `json:"status" example:"pending"`
	Instructions      ValidationInstructions `json:"instructions"`
	VerificationToken string                 `json:"verification_token,omitempty" example:"abc123def456"`
	LastAttempt       time.Time              `json:"last_attempt,omitempty" example:"2023-01-01T00:00:00Z"`
	NextAttempt       time.Time              `json:"next_attempt,omitempty" example:"2023-01-01T01:00:00Z"`
	Attempts          int                    `json:"attempts" example:"1"`
	Error             string                 `json:"error,omitempty" example:"DNS records not found"`
}

// DomainValidationStatusResponse represents the validation status response
// @Description Detailed domain validation status with history
type DomainValidationStatusResponse struct {
	DomainID          uuid.UUID          `json:"domain_id" example:"123e4567-e89b-12d3-a456-426614174000"`
	DomainName        string             `json:"domain_name" example:"example.com"`
	Status            string             `json:"status" example:"verified"`
	VerificationToken string             `json:"verification_token,omitempty" example:"abc123def456"`
	LastAttempt       time.Time          `json:"last_attempt,omitempty" example:"2023-01-01T00:00:00Z"`
	NextAttempt       time.Time          `json:"next_attempt,omitempty" example:"2023-01-01T01:00:00Z"`
	Attempts          int                `json:"attempts" example:"2"`
	Error             string             `json:"error,omitempty" example:"TXT record validation failed"`
	History           []ValidationRecord `json:"history"`
	IsVerified        bool               `json:"is_verified" example:"true"`
	CanRetry          bool               `json:"can_retry" example:"false"`
	TXTRecord         string             `json:"txt_record" example:"mailvault-verification=abc123def456"`
}

// ValidationInstructionsResponse represents validation setup instructions
// @Description Complete validation instructions for domain setup
type ValidationInstructionsResponse struct {
	DomainID            uuid.UUID              `json:"domain_id" example:"123e4567-e89b-12d3-a456-426614174000"`
	DomainName          string                 `json:"domain_name" example:"example.com"`
	Status              string                 `json:"status" example:"pending"`
	Instructions        ValidationInstructions `json:"instructions"`
	VerificationSteps   []string               `json:"verification_steps"`
	TroubleshootingTips []string               `json:"troubleshooting_tips"`
	VerificationToken   string                 `json:"verification_token,omitempty" example:"abc123def456"`
}

// ValidationInstructions provides DNS setup instructions
// @Description DNS setup instructions for domain validation
type ValidationInstructions struct {
	MXRecords MXRecordInstructions  `json:"mx_records"`
	TXTRecord TXTRecordInstructions `json:"txt_record"`
}

// MXRecordInstructions provides MX record setup instructions
// @Description MX record setup instructions
type MXRecordInstructions struct {
	RequiredRecords []MXRecordInfo `json:"required_records"`
	Instructions    string         `json:"instructions" example:"Add these MX records to your DNS configuration"`
	Example         string         `json:"example,omitempty"`
}

// TXTRecordInstructions provides TXT record setup instructions
// @Description TXT record setup instructions for domain verification
type TXTRecordInstructions struct {
	RecordName   string `json:"record_name" example:"example.com"`
	RecordValue  string `json:"record_value" example:"mailvault-verification=abc123def456"`
	Instructions string `json:"instructions" example:"Add this TXT record to verify domain ownership"`
	Example      string `json:"example,omitempty"`
}

// MXRecordInfo represents an MX record configuration
// @Description MX record information
type MXRecordInfo struct {
	Host     string `json:"host" example:"mail.mailvault.sh"`
	Priority int    `json:"priority" example:"10"`
}

// ValidationRecord represents a validation attempt record
// @Description Individual validation attempt record
type ValidationRecord struct {
	ID             uuid.UUID         `json:"id" example:"123e4567-e89b-12d3-a456-426614174000"`
	ValidationType string            `json:"validation_type" example:"full_validation"`
	Status         string            `json:"status" example:"success"`
	Details        ValidationDetails `json:"details"`
	StartedAt      time.Time         `json:"started_at" example:"2023-01-01T00:00:00Z"`
	CompletedAt    time.Time         `json:"completed_at,omitempty" example:"2023-01-01T00:00:30Z"`
	ErrorMessage   string            `json:"error_message,omitempty" example:"DNS timeout"`
	CreatedAt      time.Time         `json:"created_at" example:"2023-01-01T00:00:00Z"`
}

// ValidationDetails contains detailed validation results
// @Description Detailed validation results including DNS records found
type ValidationDetails struct {
	ExpectedMXServers   []string       `json:"expected_mx_servers,omitempty" example:"[\"mail.mailvault.sh\", \"mail2.mailvault.sh\"]"`
	FoundMXRecords      []MXRecordInfo `json:"found_mx_records,omitempty"`
	MXValidationPassed  bool           `json:"mx_validation_passed,omitempty" example:"true"`
	ExpectedTXTRecord   string         `json:"expected_txt_record,omitempty" example:"mailvault-verification=abc123def456"`
	FoundTXTRecords     []string       `json:"found_txt_records,omitempty" example:"[\"mailvault-verification=abc123def456\"]"`
	TXTValidationPassed bool           `json:"txt_validation_passed,omitempty" example:"true"`
	DNSServer           string         `json:"dns_server,omitempty" example:"8.8.8.8:53"`
	QueryTime           string         `json:"query_time,omitempty" example:"250ms"`
	RetryCount          int            `json:"retry_count,omitempty" example:"0"`
	ErrorDetails        string         `json:"error_details,omitempty" example:"Connection timeout"`
}

package mailvault

// APIError represents an error returned by the MailVault API
type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	return e.Message
}

// ErrorResponse represents the standard error response format
type ErrorResponse struct {
	Error string `json:"error"`
}

// User represents a MailVault user
type User struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	AuthProvider string `json:"auth_provider"`
	CreatedAt    string `json:"created_at"`
}

// Domain represents a MailVault domain
type Domain struct {
	ID             string         `json:"id"`
	Domain         string         `json:"domain"`
	PublicKey      string         `json:"public_key"`
	APIKey         string         `json:"api_key"`
	Verified       bool           `json:"verified"`
	WebhookConfig  *WebhookConfig `json:"webhook_config,omitempty"`
	StorageEnabled bool           `json:"storage_enabled"`
	CreatedAt      string         `json:"created_at"`
	UpdatedAt      string         `json:"updated_at"`
}

// WebhookConfig represents webhook configuration for a domain
type WebhookConfig struct {
	URL     string            `json:"url"`
	Secret  string            `json:"secret,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Enabled bool              `json:"enabled"`
}

// EmailAddress represents an email address configuration
type EmailAddress struct {
	ID               string   `json:"id"`
	DomainID         string   `json:"domain_id"`
	LocalPart        string   `json:"local_part"`
	FullAddress      string   `json:"full_address"`
	ForwardAddresses []string `json:"forward_addresses"`
	IsCatchAll       bool     `json:"is_catch_all"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
}

// ReceivedEmail represents a received email
type ReceivedEmail struct {
	ID             string `json:"id"`
	FromAddress    string `json:"from_address"`
	Subject        string `json:"subject"`
	EncryptedBody  string `json:"encrypted_body"`
	SequenceNumber int    `json:"sequence_number"`
	ReceivedAt     string `json:"received_at"`
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Pagination Pagination  `json:"pagination"`
}

// Pagination contains pagination metadata
type Pagination struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// AuthResponse represents authentication response
type AuthResponse struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}

// SendEmailResponse represents email sending response
type SendEmailResponse struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
}

// Request types

// RegisterRequest represents user registration request
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// LoginRequest represents user login request
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// CreateDomainRequest represents domain creation request
type CreateDomainRequest struct {
	Domain         string         `json:"domain" validate:"required"`
	PublicKey      string         `json:"public_key" validate:"required"`
	WebhookConfig  *WebhookConfig `json:"webhook_config,omitempty"`
	StorageEnabled *bool          `json:"storage_enabled,omitempty"`
}

// UpdateDomainRequest represents domain update request
type UpdateDomainRequest struct {
	PublicKey      *string        `json:"public_key,omitempty"`
	Verified       *bool          `json:"verified,omitempty"`
	WebhookConfig  *WebhookConfig `json:"webhook_config,omitempty"`
	StorageEnabled *bool          `json:"storage_enabled,omitempty"`
}

// CreateEmailRequest represents email address creation request
type CreateEmailRequest struct {
	LocalPart        string   `json:"local_part" validate:"required"`
	ForwardAddresses []string `json:"forward_addresses"`
	IsCatchAll       bool     `json:"is_catch_all"`
}

// UpdateEmailRequest represents email address update request
type UpdateEmailRequest struct {
	ForwardAddresses []string `json:"forward_addresses"`
	IsCatchAll       *bool    `json:"is_catch_all,omitempty"`
}

// SendEmailRequest represents email sending request
type SendEmailRequest struct {
	From     string   `json:"from" validate:"required"`
	To       []string `json:"to" validate:"required,min=1"`
	CC       []string `json:"cc,omitempty"`
	BCC      []string `json:"bcc,omitempty"`
	Subject  string   `json:"subject" validate:"required"`
	TextBody string   `json:"text_body,omitempty"`
	HTMLBody string   `json:"html_body,omitempty"`
}

// GetReceivedEmailsOptions represents options for getting received emails
type GetReceivedEmailsOptions struct {
	Limit  *int `json:"limit,omitempty"`
	Offset *int `json:"offset,omitempty"`
}

// HealthStatus represents the API health status
type HealthStatus struct {
	Status string `json:"status"`
}
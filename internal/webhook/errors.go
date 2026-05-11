package webhook

import "errors"

// Webhook validation errors
var (
	ErrInvalidEventType          = errors.New("invalid event type")
	ErrMissingEventID            = errors.New("missing event ID")
	ErrMissingEmailID            = errors.New("missing email ID")
	ErrMissingFromAddress        = errors.New("missing from address")
	ErrMissingEncryptedBody      = errors.New("missing encrypted body")
	ErrMissingRecipientAddress   = errors.New("missing recipient address")
	ErrMissingRecipientAddressID = errors.New("missing recipient address ID")
	ErrMissingDomainID           = errors.New("missing domain ID")
	ErrMissingDomainName         = errors.New("missing domain name")
)

// Webhook delivery errors
var (
	ErrWebhookNotConfigured = errors.New("webhook not configured for domain")
	ErrWebhookDisabled      = errors.New("webhook is disabled for domain")
	ErrInvalidWebhookURL    = errors.New("invalid webhook URL")
	ErrWebhookTimeout       = errors.New("webhook request timed out")
	ErrWebhookFailed        = errors.New("webhook delivery failed")
	ErrMaxRetriesExceeded   = errors.New("maximum webhook retries exceeded")
)

package entities

import (
	"fmt"
	"time"

	"github.com/gofrs/uuid/v5"
)

// EmailSendStatus represents the status of an email in the sending pipeline
type EmailSendStatus string

const (
	EmailSendStatusPending   EmailSendStatus = "pending"   // Email queued but not yet processed
	EmailSendStatusQueued    EmailSendStatus = "queued"    // Email in worker queue for sending
	EmailSendStatusSending   EmailSendStatus = "sending"   // Email currently being sent
	EmailSendStatusSent      EmailSendStatus = "sent"      // Email successfully sent to SMTP server
	EmailSendStatusDelivered EmailSendStatus = "delivered" // Email successfully delivered (if tracking available)
	EmailSendStatusBounced   EmailSendStatus = "bounced"   // Email bounced back
	EmailSendStatusFailed    EmailSendStatus = "failed"    // Email sending failed
	EmailSendStatusCancelled EmailSendStatus = "cancelled" // Email sending cancelled
)

// IsValid checks if the email send status is valid
func (s EmailSendStatus) IsValid() bool {
	switch s {
	case EmailSendStatusPending, EmailSendStatusQueued, EmailSendStatusSending,
		EmailSendStatusSent, EmailSendStatusDelivered, EmailSendStatusBounced,
		EmailSendStatusFailed, EmailSendStatusCancelled:
		return true
	default:
		return false
	}
}

// IsFinal returns true if the status represents a final state
func (s EmailSendStatus) IsFinal() bool {
	switch s {
	case EmailSendStatusDelivered, EmailSendStatusBounced, EmailSendStatusFailed, EmailSendStatusCancelled:
		return true
	default:
		return false
	}
}

// IsError returns true if the status represents an error state
func (s EmailSendStatus) IsError() bool {
	switch s {
	case EmailSendStatusBounced, EmailSendStatusFailed:
		return true
	default:
		return false
	}
}

// CanRetry returns true if the email can be retried
func (s EmailSendStatus) CanRetry() bool {
	switch s {
	case EmailSendStatusFailed:
		return true
	default:
		return false
	}
}

// SentEmail represents an email that has been sent or is being sent via the API
type SentEmail struct {
	ID       uuid.UUID `json:"id" db:"id"`
	DomainID uuid.UUID `json:"domain_id" db:"domain_id"`

	// Email addressing
	FromAddress  string   `json:"from_address" db:"from_address"`
	ToAddresses  []string `json:"to_addresses" db:"to_addresses"`
	CCAddresses  []string `json:"cc_addresses,omitempty" db:"cc_addresses"`
	BCCAddresses []string `json:"bcc_addresses,omitempty" db:"bcc_addresses"`

	// Email content
	Subject  string  `json:"subject" db:"subject"`
	TextBody *string `json:"text_body,omitempty" db:"text_body"`
	HTMLBody *string `json:"html_body,omitempty" db:"html_body"`

	// Tracking and metadata
	MessageID string          `json:"message_id" db:"message_id"`
	Status    EmailSendStatus `json:"status" db:"status"`

	// Error handling and retries
	ErrorMessage *string `json:"error_message,omitempty" db:"error_message"`
	RetryCount   int     `json:"retry_count" db:"retry_count"`
	MaxRetries   int     `json:"max_retries" db:"max_retries"`

	// Timestamps
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	QueuedAt    *time.Time `json:"queued_at,omitempty" db:"queued_at"`
	SentAt      *time.Time `json:"sent_at,omitempty" db:"sent_at"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty" db:"delivered_at"`
	FailedAt    *time.Time `json:"failed_at,omitempty" db:"failed_at"`
	NextRetryAt *time.Time `json:"next_retry_at,omitempty" db:"next_retry_at"`

	// SMTP delivery details
	SMTPResponse  *string `json:"smtp_response,omitempty" db:"smtp_response"`
	SMTPMessageID *string `json:"smtp_message_id,omitempty" db:"smtp_message_id"`

	// Email provider tracking
	ProviderID           *uuid.UUID `json:"provider_id,omitempty" db:"provider_id"`
	ProviderName         *string    `json:"provider_name,omitempty" db:"provider_name"`
	ProviderAttemptCount int        `json:"provider_attempt_count" db:"provider_attempt_count"`
	LastProviderError    *string    `json:"last_provider_error,omitempty" db:"last_provider_error"`

	// Webhook and event data
	WebhookData      map[string]interface{} `json:"webhook_data,omitempty" db:"webhook_data"`
	WebhookDelivered bool                   `json:"webhook_delivered" db:"webhook_delivered"`
	WebhookAttempts  int                    `json:"webhook_attempts" db:"webhook_attempts"`
}

// IsValid checks if the sent email entity has valid data
func (se *SentEmail) IsValid() bool {
	if se.DomainID == uuid.Nil {
		return false
	}
	if se.FromAddress == "" {
		return false
	}
	if len(se.ToAddresses) == 0 {
		return false
	}
	if se.Subject == "" {
		return false
	}
	if se.TextBody == nil && se.HTMLBody == nil {
		return false
	}
	if se.MessageID == "" {
		return false
	}
	if !se.Status.IsValid() {
		return false
	}
	return true
}

// GetAllRecipients returns all recipient addresses (to, cc, bcc combined)
func (se *SentEmail) GetAllRecipients() []string {
	var recipients []string
	recipients = append(recipients, se.ToAddresses...)
	recipients = append(recipients, se.CCAddresses...)
	recipients = append(recipients, se.BCCAddresses...)
	return recipients
}

// GetRecipientCount returns the total number of recipients
func (se *SentEmail) GetRecipientCount() int {
	return len(se.ToAddresses) + len(se.CCAddresses) + len(se.BCCAddresses)
}

// MarkAsQueued updates the status to queued and sets queued timestamp
func (se *SentEmail) MarkAsQueued() {
	se.Status = EmailSendStatusQueued
	now := time.Now()
	se.QueuedAt = &now
}

// MarkAsSending updates the status to sending
func (se *SentEmail) MarkAsSending() {
	se.Status = EmailSendStatusSending
}

// MarkAsSent updates the status to sent and sets sent timestamp
func (se *SentEmail) MarkAsSent(smtpMessageID *string, smtpResponse *string) {
	se.Status = EmailSendStatusSent
	now := time.Now()
	se.SentAt = &now
	se.SMTPMessageID = smtpMessageID
	se.SMTPResponse = smtpResponse
	// Clear any previous error
	se.ErrorMessage = nil
	se.LastProviderError = nil
}

// MarkAsDelivered updates the status to delivered and sets delivered timestamp
func (se *SentEmail) MarkAsDelivered() {
	se.Status = EmailSendStatusDelivered
	now := time.Now()
	se.DeliveredAt = &now
}

// MarkAsFailed updates the status to failed and sets error information
func (se *SentEmail) MarkAsFailed(errorMessage string) {
	se.Status = EmailSendStatusFailed
	se.ErrorMessage = &errorMessage
	now := time.Now()
	se.FailedAt = &now
	se.RetryCount++

	// Calculate next retry time if retries are available
	if se.CanRetry() {
		nextRetry := se.calculateNextRetryTime()
		se.NextRetryAt = &nextRetry
	}
}

// MarkAsBounced updates the status to bounced
func (se *SentEmail) MarkAsBounced(bounceReason string) {
	se.Status = EmailSendStatusBounced
	se.ErrorMessage = &bounceReason
	now := time.Now()
	se.FailedAt = &now
	// Bounces are typically final - no retry
	se.NextRetryAt = nil
}

// MarkAsCancelled updates the status to cancelled
func (se *SentEmail) MarkAsCancelled() {
	se.Status = EmailSendStatusCancelled
	// Clear retry time
	se.NextRetryAt = nil
}

// CanRetry checks if this email can be retried
func (se *SentEmail) CanRetry() bool {
	return se.Status.CanRetry() && se.RetryCount < se.MaxRetries
}

// IsReadyForRetry checks if this email is ready to be retried
func (se *SentEmail) IsReadyForRetry() bool {
	if !se.CanRetry() {
		return false
	}
	if se.NextRetryAt == nil {
		return false
	}
	return time.Now().After(*se.NextRetryAt)
}

// calculateNextRetryTime calculates when this email should be retried using exponential backoff
func (se *SentEmail) calculateNextRetryTime() time.Time {
	// Exponential backoff: 5min, 15min, 45min, 2h15min, etc.
	baseDelay := 5 * time.Minute
	delay := time.Duration(1)

	// Calculate exponential backoff (3^retry_count * base_delay)
	for i := 0; i < se.RetryCount; i++ {
		delay *= 3
	}

	return time.Now().Add(delay * baseDelay)
}

// GetBody returns the email body, preferring HTML over text
func (se *SentEmail) GetBody() string {
	if se.HTMLBody != nil {
		return *se.HTMLBody
	}
	if se.TextBody != nil {
		return *se.TextBody
	}
	return ""
}

// HasHTMLBody returns true if the email has HTML content
func (se *SentEmail) HasHTMLBody() bool {
	return se.HTMLBody != nil && *se.HTMLBody != ""
}

// HasTextBody returns true if the email has text content
func (se *SentEmail) HasTextBody() bool {
	return se.TextBody != nil && *se.TextBody != ""
}

// GetStatusDisplay returns a human-readable status description
func (se *SentEmail) GetStatusDisplay() string {
	switch se.Status {
	case EmailSendStatusPending:
		return "Pending"
	case EmailSendStatusQueued:
		return "Queued for sending"
	case EmailSendStatusSending:
		return "Sending..."
	case EmailSendStatusSent:
		return "Sent successfully"
	case EmailSendStatusDelivered:
		return "Delivered"
	case EmailSendStatusBounced:
		return "Bounced"
	case EmailSendStatusFailed:
		if se.CanRetry() {
			return fmt.Sprintf("Failed (retry %d/%d)", se.RetryCount, se.MaxRetries)
		}
		return "Failed (no more retries)"
	case EmailSendStatusCancelled:
		return "Cancelled"
	default:
		return string(se.Status)
	}
}

// NeedsWebhookDelivery returns true if this email needs webhook notification
func (se *SentEmail) NeedsWebhookDelivery() bool {
	return !se.WebhookDelivered && se.Status.IsFinal()
}

// MarkWebhookDelivered marks the webhook as successfully delivered
func (se *SentEmail) MarkWebhookDelivered() {
	se.WebhookDelivered = true
}

// IncrementWebhookAttempts increments the webhook delivery attempts
func (se *SentEmail) IncrementWebhookAttempts() {
	se.WebhookAttempts++
}

// Email Provider Methods

// SetProvider sets the email provider for this email
func (se *SentEmail) SetProvider(providerID uuid.UUID, providerName string) {
	se.ProviderID = &providerID
	se.ProviderName = &providerName
}

// ClearProvider clears the current provider assignment
func (se *SentEmail) ClearProvider() {
	se.ProviderID = nil
	se.ProviderName = nil
}

// GetProviderID returns the provider ID if set
func (se *SentEmail) GetProviderID() uuid.UUID {
	if se.ProviderID != nil {
		return *se.ProviderID
	}
	return uuid.Nil
}

// GetProviderName returns the provider name if set
func (se *SentEmail) GetProviderName() string {
	if se.ProviderName != nil {
		return *se.ProviderName
	}
	return ""
}

// HasProvider returns true if a provider is assigned
func (se *SentEmail) HasProvider() bool {
	return se.ProviderID != nil
}

// IncrementProviderAttempts increments the number of provider attempts
func (se *SentEmail) IncrementProviderAttempts() {
	se.ProviderAttemptCount++
}

// SetProviderError sets the last provider error
func (se *SentEmail) SetProviderError(errorMessage string) {
	se.LastProviderError = &errorMessage
}

// ClearProviderError clears the last provider error
func (se *SentEmail) ClearProviderError() {
	se.LastProviderError = nil
}

// GetProviderError returns the last provider error if any
func (se *SentEmail) GetProviderError() string {
	if se.LastProviderError != nil {
		return *se.LastProviderError
	}
	return ""
}

// HasProviderError returns true if there's a provider error
func (se *SentEmail) HasProviderError() bool {
	return se.LastProviderError != nil
}

// CanTryNextProvider returns true if more providers can be tried
func (se *SentEmail) CanTryNextProvider(maxProviders int) bool {
	return se.ProviderAttemptCount < maxProviders && se.Status == EmailSendStatusFailed
}

// GetProviderDisplayInfo returns formatted provider information for display
func (se *SentEmail) GetProviderDisplayInfo() string {
	if !se.HasProvider() {
		return "No provider"
	}

	providerName := se.GetProviderName()
	if se.ProviderAttemptCount > 1 {
		return fmt.Sprintf("%s (attempt %d)", providerName, se.ProviderAttemptCount)
	}
	return providerName
}

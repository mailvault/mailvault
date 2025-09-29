package email_sending

import (
	"context"
	"time"

	"mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
)

//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/sent_email_repository.go . Repository

// Repository defines the interface for sent email data access
type Repository interface {
	// Basic CRUD operations
	CreateSentEmail(ctx context.Context, sentEmail *entities.SentEmail) error
	GetSentEmail(ctx context.Context, id uuid.UUID) (*entities.SentEmail, error)
	GetSentEmailByMessageID(ctx context.Context, messageID string) (*entities.SentEmail, error)
	UpdateSentEmail(ctx context.Context, sentEmail *entities.SentEmail) error
	UpdateSentEmailStatus(ctx context.Context, id uuid.UUID, status entities.EmailSendStatus, smtpResponse *string, smtpMessageID *string, errorMessage *string) error
	DeleteSentEmail(ctx context.Context, id uuid.UUID) error

	// Query operations
	ListSentEmails(ctx context.Context, filters *SentEmailFilters) ([]*entities.SentEmail, int64, error)
	GetSentEmailsByDomain(ctx context.Context, domainID uuid.UUID, limit, offset int) ([]*entities.SentEmail, error)
	CountSentEmailsByDomain(ctx context.Context, domainID uuid.UUID) (int64, error)

	// Queue operations
	GetSentEmailsPendingSend(ctx context.Context, limit int) ([]*entities.SentEmail, error)
	GetSentEmailsForRetry(ctx context.Context, limit int) ([]*entities.SentEmail, error)

	// Webhook operations
	GetSentEmailsNeedingWebhook(ctx context.Context, limit int) ([]*entities.SentEmail, error)
	UpdateWebhookDelivery(ctx context.Context, id uuid.UUID, delivered bool, attempts int) error

	// Statistics
	GetSentEmailStats(ctx context.Context, filters *SentEmailStatsFilters) (*SentEmailStats, error)

	// Bulk operations
	BulkUpdateStatus(ctx context.Context, emailIDs []uuid.UUID, status entities.EmailSendStatus) error

	// Cleanup operations
	GetSentEmailsOlderThan(ctx context.Context, duration time.Duration, limit int) ([]*entities.SentEmail, error)
	DeleteOldSentEmails(ctx context.Context, olderThan time.Time) (int64, error)
}

// Filter structures

type SentEmailFilters struct {
	DomainID        *uuid.UUID                `json:"domain_id,omitempty"`
	Status          *entities.EmailSendStatus `json:"status,omitempty"`
	Statuses        []entities.EmailSendStatus `json:"statuses,omitempty"`
	FromAddress     *string                   `json:"from_address,omitempty"`
	ToAddress       *string                   `json:"to_address,omitempty"`
	Subject         *string                   `json:"subject,omitempty"`
	MessageID       *string                   `json:"message_id,omitempty"`
	CreatedFrom     *time.Time                `json:"created_from,omitempty"`
	CreatedTo       *time.Time                `json:"created_to,omitempty"`
	SentFrom        *time.Time                `json:"sent_from,omitempty"`
	SentTo          *time.Time                `json:"sent_to,omitempty"`
	HasError        *bool                     `json:"has_error,omitempty"`
	NeedsRetry      *bool                     `json:"needs_retry,omitempty"`
	PendingWebhook  *bool                     `json:"pending_webhook,omitempty"`
	IncludeBody     bool                      `json:"include_body"`
	IncludeWebhook  bool                      `json:"include_webhook"`
	OrderBy         string                    `json:"order_by"`
	OrderDir        string                    `json:"order_dir"`
	Limit           *int                      `json:"limit,omitempty"`
	Offset          *int                      `json:"offset,omitempty"`
}

type SentEmailStatsFilters struct {
	DomainID *uuid.UUID `json:"domain_id,omitempty"`
	Since    *time.Time `json:"since,omitempty"`
	Until    *time.Time `json:"until,omitempty"`
}

// Response structures

type SentEmailStats struct {
	TotalSent      int64   `json:"total_sent"`
	TotalDelivered int64   `json:"total_delivered"`
	TotalFailed    int64   `json:"total_failed"`
	TotalBounced   int64   `json:"total_bounced"`
	TotalPending   int64   `json:"total_pending"`
	TotalQueued    int64   `json:"total_queued"`
	DeliveryRate   float64 `json:"delivery_rate"`
	BounceRate     float64 `json:"bounce_rate"`
}

// Errors
var (
	ErrSentEmailNotFound = &SentEmailError{Code: "SENT_EMAIL_NOT_FOUND", Message: "Sent email not found"}
	ErrInvalidStatus     = &SentEmailError{Code: "INVALID_STATUS", Message: "Invalid email status"}
	ErrEmailNotSendable  = &SentEmailError{Code: "EMAIL_NOT_SENDABLE", Message: "Email is not in a sendable state"}
)

type SentEmailError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *SentEmailError) Error() string {
	return e.Message
}
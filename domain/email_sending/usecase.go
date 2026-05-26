package email_sending

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mailvault/mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
)

// UseCase orchestrates outbound mail: persist a sent-email row, hand the
// message to the configured Sender (a local SMTP relay by default), record
// the outcome.
type UseCase struct {
	repo   Repository
	sender Sender
	logger *slog.Logger
}

// NewUseCase builds the use case.
func NewUseCase(repo Repository, sender Sender, logger *slog.Logger) *UseCase {
	return &UseCase{repo: repo, sender: sender, logger: logger}
}

// SendEmail validates the request, persists a sent-email row, dispatches
// through the configured Sender, and updates the row with the outcome.
func (uc *UseCase) SendEmail(ctx context.Context, req SendEmailRequest) (*SendEmailResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid send email request: %w", err)
	}

	sentEmail := &entities.SentEmail{
		ID:           uuid.Must(uuid.NewV4()),
		DomainID:     req.DomainID,
		FromAddress:  req.From,
		ToAddresses:  req.ToAddresses,
		CCAddresses:  req.CCAddresses,
		BCCAddresses: req.BCCAddresses,
		Subject:      req.Subject,
		TextBody:     req.TextBody,
		HTMLBody:     req.HTMLBody,
		MessageID:    req.MessageID,
		Status:       entities.EmailSendStatusPending,
		RetryCount:   0,
		MaxRetries:   req.MaxRetries,
		CreatedAt:    time.Now(),
		WebhookData:  req.WebhookData,
	}

	if !sentEmail.IsValid() {
		return nil, fmt.Errorf("invalid sent email entity")
	}

	if err := uc.repo.CreateSentEmail(ctx, sentEmail); err != nil {
		return nil, fmt.Errorf("failed to create sent email: %w", err)
	}

	messageID, err := uc.sender.Send(ctx, sentEmail)
	if err != nil {
		sentEmail.MarkAsFailed(err.Error())
		if updateErr := uc.repo.UpdateSentEmail(ctx, sentEmail); updateErr != nil {
			uc.logger.Error("failed to update sent email after failure",
				"sent_email_id", sentEmail.ID,
				"error", updateErr)
		}
		return nil, err
	}

	if messageID == "" {
		messageID = sentEmail.MessageID
	}
	emptyResponse := ""
	sentEmail.MarkAsSent(&messageID, &emptyResponse)
	if updateErr := uc.repo.UpdateSentEmail(ctx, sentEmail); updateErr != nil {
		uc.logger.Error("failed to update sent email after success",
			"sent_email_id", sentEmail.ID,
			"error", updateErr)
	}

	return &SendEmailResponse{
		ID:        sentEmail.ID,
		MessageID: sentEmail.MessageID,
		Status:    string(sentEmail.Status),
		CreatedAt: sentEmail.CreatedAt,
		SentAt:    sentEmail.SentAt,
	}, nil
}

// ResendEmail re-attempts a previously-failed send.
func (uc *UseCase) ResendEmail(ctx context.Context, emailID uuid.UUID) (*SendEmailResponse, error) {
	sentEmail, err := uc.repo.GetSentEmail(ctx, emailID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sent email: %w", err)
	}
	if !sentEmail.CanRetry() {
		return nil, fmt.Errorf("email cannot be resent: max retries exceeded or not in failed state")
	}
	sentEmail.Status = entities.EmailSendStatusPending

	messageID, err := uc.sender.Send(ctx, sentEmail)
	if err != nil {
		sentEmail.MarkAsFailed(err.Error())
		if updateErr := uc.repo.UpdateSentEmail(ctx, sentEmail); updateErr != nil {
			uc.logger.Error("failed to update sent email after resend failure",
				"sent_email_id", sentEmail.ID,
				"error", updateErr)
		}
		return nil, err
	}

	if messageID == "" {
		messageID = sentEmail.MessageID
	}
	emptyResponse := ""
	sentEmail.MarkAsSent(&messageID, &emptyResponse)
	if updateErr := uc.repo.UpdateSentEmail(ctx, sentEmail); updateErr != nil {
		uc.logger.Error("failed to update sent email after resend success",
			"sent_email_id", sentEmail.ID,
			"error", updateErr)
	}

	return &SendEmailResponse{
		ID:        sentEmail.ID,
		MessageID: sentEmail.MessageID,
		Status:    string(sentEmail.Status),
		CreatedAt: sentEmail.CreatedAt,
		SentAt:    sentEmail.SentAt,
	}, nil
}

// GetSentEmail retrieves a sent email by ID.
func (uc *UseCase) GetSentEmail(ctx context.Context, emailID uuid.UUID) (*entities.SentEmail, error) {
	return uc.repo.GetSentEmail(ctx, emailID)
}

// ListSentEmails lists sent emails with filtering.
func (uc *UseCase) ListSentEmails(ctx context.Context, filters *SentEmailFilters) ([]*entities.SentEmail, int64, error) {
	return uc.repo.ListSentEmails(ctx, filters)
}

// GetSentEmailStats retrieves sending statistics.
func (uc *UseCase) GetSentEmailStats(ctx context.Context, filters *SentEmailStatsFilters) (*SentEmailStats, error) {
	return uc.repo.GetSentEmailStats(ctx, filters)
}

// ProcessRetryQueue processes emails ready for retry.
func (uc *UseCase) ProcessRetryQueue(ctx context.Context, limit int) error {
	emails, err := uc.repo.GetSentEmailsForRetry(ctx, limit)
	if err != nil {
		return fmt.Errorf("failed to get emails for retry: %w", err)
	}
	for _, email := range emails {
		uc.logger.Info("processing email retry",
			"sent_email_id", email.ID,
			"retry_count", email.RetryCount,
			"max_retries", email.MaxRetries)
		if _, err := uc.ResendEmail(ctx, email.ID); err != nil {
			uc.logger.Error("failed to retry email",
				"sent_email_id", email.ID,
				"error", err)
		}
	}
	return nil
}

// Request and response structures

type SendEmailRequest struct {
	DomainID  uuid.UUID `json:"domain_id"`
	MessageID string    `json:"message_id"`

	From    string `json:"from"`
	ReplyTo string `json:"reply_to,omitempty"`

	ToAddresses  []string `json:"to_addresses"`
	CCAddresses  []string `json:"cc_addresses,omitempty"`
	BCCAddresses []string `json:"bcc_addresses,omitempty"`

	Subject  string  `json:"subject"`
	TextBody *string `json:"text_body,omitempty"`
	HTMLBody *string `json:"html_body,omitempty"`

	MaxRetries int `json:"max_retries,omitempty"`

	WebhookData map[string]interface{} `json:"webhook_data,omitempty"`
}

func (r *SendEmailRequest) Validate() error {
	if r.DomainID == uuid.Nil {
		return fmt.Errorf("domain_id is required")
	}
	if r.From == "" {
		return fmt.Errorf("from address is required")
	}
	if len(r.ToAddresses) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}
	if r.Subject == "" {
		return fmt.Errorf("subject is required")
	}
	if r.TextBody == nil && r.HTMLBody == nil {
		return fmt.Errorf("either text_body or html_body is required")
	}
	if r.MessageID == "" {
		return fmt.Errorf("message_id is required")
	}
	if r.MaxRetries < 0 {
		return fmt.Errorf("max_retries must be non-negative")
	}
	return nil
}

type SendEmailResponse struct {
	ID        uuid.UUID  `json:"id"`
	MessageID string     `json:"message_id"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	SentAt    *time.Time `json:"sent_at,omitempty"`
}

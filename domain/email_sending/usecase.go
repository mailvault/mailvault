package email_sending

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"mailvault/domain/email_provider"
	"mailvault/domain/entities"
	"mailvault/internal/providers"

	"github.com/gofrs/uuid/v5"
)

// UseCase defines the business logic for email sending
type UseCase struct {
	repo              Repository
	providerRepo      email_provider.Repository
	providerLogRepo   email_provider.LogRepository
	emailSender       providers.EmailSender
	logger            *slog.Logger
}

// NewUseCase creates a new email sending use case
func NewUseCase(
	repo Repository,
	providerRepo email_provider.Repository,
	providerLogRepo email_provider.LogRepository,
	emailSender providers.EmailSender,
	logger *slog.Logger,
) *UseCase {
	return &UseCase{
		repo:            repo,
		providerRepo:    providerRepo,
		providerLogRepo: providerLogRepo,
		emailSender:     emailSender,
		logger:          logger,
	}
}

// SendEmail sends an email using the best available provider
func (uc *UseCase) SendEmail(ctx context.Context, req SendEmailRequest) (*SendEmailResponse, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid send email request: %w", err)
	}

	// Create sent email entity
	sentEmail := &entities.SentEmail{
		ID:                   uuid.Must(uuid.NewV4()),
		DomainID:             req.DomainID,
		FromAddress:          req.From,
		ToAddresses:          req.ToAddresses,
		CCAddresses:          req.CCAddresses,
		BCCAddresses:         req.BCCAddresses,
		Subject:              req.Subject,
		TextBody:             req.TextBody,
		HTMLBody:             req.HTMLBody,
		MessageID:            req.MessageID,
		Status:               entities.EmailSendStatusPending,
		RetryCount:           0,
		MaxRetries:           req.MaxRetries,
		ProviderAttemptCount: 1,
		CreatedAt:            time.Now(),
		WebhookData:          req.WebhookData,
	}

	// Validate sent email
	if !sentEmail.IsValid() {
		return nil, fmt.Errorf("invalid sent email entity")
	}

	// Save to database
	if err := uc.repo.CreateSentEmail(ctx, sentEmail); err != nil {
		return nil, fmt.Errorf("failed to create sent email: %w", err)
	}

	// Build provider request
	providerReq := uc.buildProviderRequest(req)

	// Try to send with available providers
	response, err := uc.sendWithRetry(ctx, sentEmail, providerReq)
	if err != nil {
		// Mark as failed and save
		sentEmail.MarkAsFailed(err.Error())
		if updateErr := uc.repo.UpdateSentEmail(ctx, sentEmail); updateErr != nil {
			uc.logger.Error("failed to update sent email after failure",
				"sent_email_id", sentEmail.ID,
				"error", updateErr)
		}
		return nil, err
	}

	// Update sent email with success
	sentEmail.MarkAsSent(&response.ProviderMessageID, &response.ProviderResponse)
	if updateErr := uc.repo.UpdateSentEmail(ctx, sentEmail); updateErr != nil {
		uc.logger.Error("failed to update sent email after success",
			"sent_email_id", sentEmail.ID,
			"error", updateErr)
	}

	return &SendEmailResponse{
		ID:                sentEmail.ID,
		MessageID:         sentEmail.MessageID,
		Status:            string(sentEmail.Status),
		ProviderID:        sentEmail.GetProviderID(),
		ProviderName:      sentEmail.GetProviderName(),
		ProviderMessageID: response.ProviderMessageID,
		CreatedAt:         sentEmail.CreatedAt,
		SentAt:            sentEmail.SentAt,
	}, nil
}

// sendWithRetry attempts to send email with multiple providers
func (uc *UseCase) sendWithRetry(ctx context.Context, sentEmail *entities.SentEmail, req providers.SendEmailRequest) (*providers.SendEmailResponse, error) {
	var lastError error
	var excludeProviderIDs []uuid.UUID

	maxProviderAttempts := 3 // Max different providers to try

	for attempt := 1; attempt <= maxProviderAttempts; attempt++ {
		// Get next available provider
		provider, err := uc.providerRepo.GetNextAvailableProvider(ctx, sentEmail.DomainID, excludeProviderIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to get available provider: %w", err)
		}
		if provider == nil {
			break // No more providers available
		}

		// Update sent email with current provider
		sentEmail.SetProvider(provider.ID, provider.Name)
		sentEmail.IncrementProviderAttempts()

		uc.logger.Info("attempting to send email with provider",
			"sent_email_id", sentEmail.ID,
			"provider_id", provider.ID,
			"provider_type", provider.Type,
			"provider_name", provider.Name,
			"attempt", attempt)

		// Try to send with this provider
		response, err := uc.emailSender.SendEmailWithProvider(ctx, provider.ID, req)
		if err != nil {
			lastError = err
			excludeProviderIDs = append(excludeProviderIDs, provider.ID)

			// Log the failure
			uc.logger.Warn("provider send failed",
				"sent_email_id", sentEmail.ID,
				"provider_id", provider.ID,
				"provider_type", provider.Type,
				"error", err)

			// Set provider error on sent email
			sentEmail.SetProviderError(err.Error())

			// Check if this is a retryable error
			if providerErr, ok := err.(*providers.ProviderError); ok {
				if !providerErr.IsTemporary() {
					// Non-retryable error, don't try other providers
					return nil, err
				}
			}

			continue
		}

		// Success!
		uc.logger.Info("email sent successfully",
			"sent_email_id", sentEmail.ID,
			"provider_id", provider.ID,
			"provider_type", provider.Type,
			"provider_message_id", response.ProviderMessageID)

		return response, nil
	}

	// All providers failed
	if lastError != nil {
		return nil, fmt.Errorf("all providers failed, last error: %w", lastError)
	}

	return nil, email_provider.ErrNoHealthyProviders
}

// ResendEmail resends a failed email
func (uc *UseCase) ResendEmail(ctx context.Context, emailID uuid.UUID) (*SendEmailResponse, error) {
	// Get the sent email
	sentEmail, err := uc.repo.GetSentEmail(ctx, emailID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sent email: %w", err)
	}

	// Check if email can be resent
	if !sentEmail.CanRetry() {
		return nil, fmt.Errorf("email cannot be resent: max retries exceeded or not in failed state")
	}

	// Reset provider assignment and errors
	sentEmail.ClearProvider()
	sentEmail.ClearProviderError()
	sentEmail.Status = entities.EmailSendStatusPending
	sentEmail.ProviderAttemptCount = 1

	// Build provider request from sent email
	providerReq := uc.buildProviderRequestFromSentEmail(sentEmail)

	// Try to send again
	response, err := uc.sendWithRetry(ctx, sentEmail, providerReq)
	if err != nil {
		// Mark as failed and save
		sentEmail.MarkAsFailed(err.Error())
		if updateErr := uc.repo.UpdateSentEmail(ctx, sentEmail); updateErr != nil {
			uc.logger.Error("failed to update sent email after resend failure",
				"sent_email_id", sentEmail.ID,
				"error", updateErr)
		}
		return nil, err
	}

	// Update sent email with success
	sentEmail.MarkAsSent(&response.ProviderMessageID, &response.ProviderResponse)
	if updateErr := uc.repo.UpdateSentEmail(ctx, sentEmail); updateErr != nil {
		uc.logger.Error("failed to update sent email after resend success",
			"sent_email_id", sentEmail.ID,
			"error", updateErr)
	}

	return &SendEmailResponse{
		ID:                sentEmail.ID,
		MessageID:         sentEmail.MessageID,
		Status:            string(sentEmail.Status),
		ProviderID:        sentEmail.GetProviderID(),
		ProviderName:      sentEmail.GetProviderName(),
		ProviderMessageID: response.ProviderMessageID,
		CreatedAt:         sentEmail.CreatedAt,
		SentAt:            sentEmail.SentAt,
	}, nil
}

// GetSentEmail retrieves a sent email by ID
func (uc *UseCase) GetSentEmail(ctx context.Context, emailID uuid.UUID) (*entities.SentEmail, error) {
	return uc.repo.GetSentEmail(ctx, emailID)
}

// ListSentEmails lists sent emails with filtering
func (uc *UseCase) ListSentEmails(ctx context.Context, filters *SentEmailFilters) ([]*entities.SentEmail, int64, error) {
	return uc.repo.ListSentEmails(ctx, filters)
}

// GetSentEmailStats retrieves sending statistics
func (uc *UseCase) GetSentEmailStats(ctx context.Context, filters *SentEmailStatsFilters) (*SentEmailStats, error) {
	return uc.repo.GetSentEmailStats(ctx, filters)
}

// ProcessRetryQueue processes emails ready for retry
func (uc *UseCase) ProcessRetryQueue(ctx context.Context, limit int) error {
	// Get emails ready for retry
	emails, err := uc.repo.GetSentEmailsForRetry(ctx, limit)
	if err != nil {
		return fmt.Errorf("failed to get emails for retry: %w", err)
	}

	for _, email := range emails {
		uc.logger.Info("processing email retry",
			"sent_email_id", email.ID,
			"retry_count", email.RetryCount,
			"max_retries", email.MaxRetries)

		// Try to resend
		_, err := uc.ResendEmail(ctx, email.ID)
		if err != nil {
			uc.logger.Error("failed to retry email",
				"sent_email_id", email.ID,
				"error", err)
		}
	}

	return nil
}

// Helper methods

func (uc *UseCase) buildProviderRequest(req SendEmailRequest) providers.SendEmailRequest {
	providerReq := providers.SendEmailRequest{
		MessageID:       req.MessageID,
		From:            req.From,
		FromName:        req.FromName,
		ReplyTo:         req.ReplyTo,
		Subject:         req.Subject,
		TextBody:        req.TextBody,
		HTMLBody:        req.HTMLBody,
		TrackOpens:      req.TrackOpens,
		TrackClicks:     req.TrackClicks,
		Tags:            req.Tags,
		Metadata:        req.Metadata,
		ProviderOptions: req.ProviderOptions,
	}

	// Convert to addresses
	for _, addr := range req.ToAddresses {
		providerReq.To = append(providerReq.To, providers.EmailAddress{
			Email: addr,
		})
	}

	for _, addr := range req.CCAddresses {
		providerReq.CC = append(providerReq.CC, providers.EmailAddress{
			Email: addr,
		})
	}

	for _, addr := range req.BCCAddresses {
		providerReq.BCC = append(providerReq.BCC, providers.EmailAddress{
			Email: addr,
		})
	}

	return providerReq
}

func (uc *UseCase) buildProviderRequestFromSentEmail(sentEmail *entities.SentEmail) providers.SendEmailRequest {
	providerReq := providers.SendEmailRequest{
		MessageID: sentEmail.MessageID,
		From:      sentEmail.FromAddress,
		Subject:   sentEmail.Subject,
		TextBody:  sentEmail.TextBody,
		HTMLBody:  sentEmail.HTMLBody,
	}

	// Convert to addresses
	for _, addr := range sentEmail.ToAddresses {
		providerReq.To = append(providerReq.To, providers.EmailAddress{
			Email: addr,
		})
	}

	for _, addr := range sentEmail.CCAddresses {
		providerReq.CC = append(providerReq.CC, providers.EmailAddress{
			Email: addr,
		})
	}

	for _, addr := range sentEmail.BCCAddresses {
		providerReq.BCC = append(providerReq.BCC, providers.EmailAddress{
			Email: addr,
		})
	}

	return providerReq
}

// Request and response structures

type SendEmailRequest struct {
	DomainID   uuid.UUID `json:"domain_id"`
	MessageID  string    `json:"message_id"`

	// Sender information
	From     string `json:"from"`
	FromName string `json:"from_name,omitempty"`
	ReplyTo  string `json:"reply_to,omitempty"`

	// Recipients
	ToAddresses  []string `json:"to_addresses"`
	CCAddresses  []string `json:"cc_addresses,omitempty"`
	BCCAddresses []string `json:"bcc_addresses,omitempty"`

	// Email content
	Subject  string  `json:"subject"`
	TextBody *string `json:"text_body,omitempty"`
	HTMLBody *string `json:"html_body,omitempty"`

	// Tracking and options
	TrackOpens  bool              `json:"track_opens,omitempty"`
	TrackClicks bool              `json:"track_clicks,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`

	// Retry configuration
	MaxRetries int `json:"max_retries,omitempty"`

	// Webhook data
	WebhookData map[string]interface{} `json:"webhook_data,omitempty"`

	// Provider-specific options
	ProviderOptions map[string]interface{} `json:"provider_options,omitempty"`
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
	ID                uuid.UUID  `json:"id"`
	MessageID         string     `json:"message_id"`
	Status            string     `json:"status"`
	ProviderID        uuid.UUID  `json:"provider_id,omitempty"`
	ProviderName      string     `json:"provider_name,omitempty"`
	ProviderMessageID string     `json:"provider_message_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	SentAt            *time.Time `json:"sent_at,omitempty"`
}
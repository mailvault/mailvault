package webhook

import (
	"context"

	"github.com/mailvault/mailvault/app/smtp/verification"
	"github.com/mailvault/mailvault/domain/email"
	"github.com/mailvault/mailvault/domain/entities"
)

// NotificationServiceAdapter adapts IncomingEmailNotificationService to the email.WebhookNotifier interface
type NotificationServiceAdapter struct {
	service *IncomingEmailNotificationService
}

// NewNotificationServiceAdapter creates a new adapter
func NewNotificationServiceAdapter(service *IncomingEmailNotificationService) *NotificationServiceAdapter {
	return &NotificationServiceAdapter{
		service: service,
	}
}

// NotifyIncomingEmail implements email.WebhookNotifier interface
func (a *NotificationServiceAdapter) NotifyIncomingEmail(
	ctx context.Context,
	receivedEmail *entities.ReceivedEmail,
	domain *entities.Domain,
	emailAddress *entities.EmailAddress,
	verificationResult *verification.VerificationResult,
	autoCreated bool,
) error {
	return a.service.NotifyIncomingEmail(
		ctx,
		receivedEmail,
		domain,
		emailAddress,
		verificationResult,
		autoCreated,
	)
}

// Ensure adapter implements the interface
var _ email.WebhookNotifier = (*NotificationServiceAdapter)(nil)

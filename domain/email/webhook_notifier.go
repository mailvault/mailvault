package email

import (
	"context"

	"mailvault/app/smtp/verification"
	"mailvault/domain/entities"
)

// WebhookNotifier defines the interface for sending webhook notifications for incoming emails
type WebhookNotifier interface {
	// NotifyIncomingEmail sends a webhook notification for an incoming email
	NotifyIncomingEmail(
		ctx context.Context,
		receivedEmail *entities.ReceivedEmail,
		domain *entities.Domain,
		emailAddress *entities.EmailAddress,
		verificationResult *verification.VerificationResult,
		autoCreated bool,
	) error
}
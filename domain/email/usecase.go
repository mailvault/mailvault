package email

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"mailvault/app/smtp/verification"
	"mailvault/domain/entities"
	"mailvault/internal/utils"

	"github.com/gofrs/uuid/v5"
)

type UseCase struct {
	emailRepo         EmailAddressRepository
	receivedEmailRepo ReceivedEmailRepository
	domainRepo        DomainRepository
	webhookNotifier   WebhookNotifier
	emailForwarder    EmailForwarder
}

func NewUseCase(emailRepo EmailAddressRepository, receivedEmailRepo ReceivedEmailRepository, domainRepo DomainRepository, webhookNotifier WebhookNotifier) *UseCase {
	return &UseCase{
		emailRepo:         emailRepo,
		receivedEmailRepo: receivedEmailRepo,
		domainRepo:        domainRepo,
		webhookNotifier:   webhookNotifier,
	}
}

// SetEmailForwarder configures the email forwarder used to relay incoming emails.
// It is set separately to avoid circular dependencies between the SMTP layer and the use case.
func (uc *UseCase) SetEmailForwarder(forwarder EmailForwarder) {
	uc.emailForwarder = forwarder
}

type CreateEmailAddressInput struct {
	DomainID          uuid.UUID `json:"domain_id"`
	LocalPart         string    `json:"local_part"`
	ForwardAddresses  []string  `json:"forward_addresses,omitempty"`
	ForwardingEnabled bool      `json:"forwarding_enabled"`
}

type UpdateEmailAddressInput struct {
	ForwardAddresses  []string `json:"forward_addresses,omitempty"`
	ForwardingEnabled *bool    `json:"forwarding_enabled,omitempty"`
}

type ProcessIncomingEmailInput struct {
	EmailAddressID      uuid.UUID                       `json:"email_address_id"`
	FromAddress         string                          `json:"from_address"`
	Subject             string                          `json:"subject"`
	Body                string                          `json:"body"`
	DomainID            uuid.UUID                       `json:"domain_id"`
	VerificationResults *verification.VerificationResult `json:"verification_results,omitempty"`
	IsQuarantined       bool                            `json:"is_quarantined"`
	Domain              *entities.Domain                `json:"domain,omitempty"`
	EmailAddress        *entities.EmailAddress          `json:"email_address,omitempty"`
	AutoCreated         bool                            `json:"auto_created"`
}

type GetReceivedEmailsFilter struct {
	SortBy         string  // "received_at", "sequence_number", "from_address", "subject"
	SortOrder      string  // "asc", "desc"
	EmailAddress   string  // Filter by recipient email address
	FromAddress    string  // Filter by sender email address
	DateFrom       string  // ISO date format (YYYY-MM-DD)
	DateTo         string  // ISO date format (YYYY-MM-DD)
	SpamMin        float64 // Minimum spam score (0-1)
	SpamMax        float64 // Maximum spam score (0-1)
	SecurityStatus string  // "clean", "suspicious", "quarantined", "high_risk"
	Search         string  // Full-text search in subject and from address
	Domain         string  // Filter by domain (existing functionality)
}

func (uc *UseCase) CreateEmailAddress(ctx context.Context, emailAddress *entities.EmailAddress) error {
	if !emailAddress.IsValid() {
		return fmt.Errorf("invalid email address data")
	}

	if err := uc.emailRepo.Create(ctx, emailAddress); err != nil {
		return fmt.Errorf("failed to create email address: %w", err)
	}

	return nil
}

func (uc *UseCase) CreateEmailAddressFromInput(ctx context.Context, req CreateEmailAddressInput) (*entities.EmailAddress, error) {
	if req.DomainID == uuid.Nil {
		return nil, fmt.Errorf("domain ID is required")
	}

	if req.LocalPart == "" {
		return nil, fmt.Errorf("local part is required")
	}

	// Validate local part format
	if !isValidLocalPart(req.LocalPart) {
		return nil, fmt.Errorf("invalid local part format: %s", req.LocalPart)
	}

	// Normalize local part (lowercase)
	normalizedLocalPart := strings.ToLower(req.LocalPart)

	// Check if email address already exists
	existing, err := uc.emailRepo.GetByLocalPartAndDomain(ctx, normalizedLocalPart, req.DomainID)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("email address %s already exists for this domain", normalizedLocalPart)
	}

	// Validate forward addresses
	for _, addr := range req.ForwardAddresses {
		if addr == "" {
			continue
		}
		if !isValidEmail(addr) {
			return nil, fmt.Errorf("invalid forward address: %s", addr)
		}
	}

	emailAddress := &entities.EmailAddress{
		ID:                uuid.Must(uuid.NewV4()),
		DomainID:          req.DomainID,
		LocalPart:         normalizedLocalPart,
		ForwardAddresses:  req.ForwardAddresses,
		ForwardingEnabled: req.ForwardingEnabled,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}

	if !emailAddress.IsValid() {
		return nil, fmt.Errorf("invalid email address data")
	}

	if err := uc.emailRepo.Create(ctx, emailAddress); err != nil {
		return nil, fmt.Errorf("failed to create email address: %w", err)
	}

	return emailAddress, nil
}

func (uc *UseCase) GetEmailAddressesByDomainID(ctx context.Context, domainID uuid.UUID) ([]*entities.EmailAddress, error) {
	if domainID == uuid.Nil {
		return nil, fmt.Errorf("domain ID is required")
	}

	addresses, err := uc.emailRepo.GetByDomainID(ctx, domainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get email addresses: %w", err)
	}

	return addresses, nil
}

func (uc *UseCase) GetEmailAddressByID(ctx context.Context, id uuid.UUID) (*entities.EmailAddress, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("email address ID is required")
	}

	emailAddress, err := uc.emailRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get email address: %w", err)
	}

	return emailAddress, nil
}

func (uc *UseCase) UpdateEmailAddress(ctx context.Context, id uuid.UUID, req UpdateEmailAddressInput) (*entities.EmailAddress, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("email address ID is required")
	}

	emailAddress, err := uc.emailRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get email address: %w", err)
	}

	// Update fields if provided
	if req.ForwardAddresses != nil {
		// Validate forward addresses
		for _, addr := range req.ForwardAddresses {
			if !isValidEmail(addr) {
				return nil, fmt.Errorf("invalid forward address: %s", addr)
			}
		}
		emailAddress.ForwardAddresses = req.ForwardAddresses
	}

	if req.ForwardingEnabled != nil {
		emailAddress.ForwardingEnabled = *req.ForwardingEnabled
	}

	emailAddress.UpdatedAt = time.Now().UTC()

	if !emailAddress.IsValid() {
		return nil, fmt.Errorf("invalid email address data after update")
	}

	if err := uc.emailRepo.Update(ctx, emailAddress); err != nil {
		return nil, fmt.Errorf("failed to update email address: %w", err)
	}

	return emailAddress, nil
}

func (uc *UseCase) GetEmailAddressByAddress(ctx context.Context, fullAddress string) (*entities.EmailAddress, error) {
	// Parse email address using safe parsing
	localPart, domainName, err := utils.ParseEmailAddress(fullAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid email address format '%s': %w", fullAddress, err)
	}

	localPart = strings.ToLower(localPart)
	domainName = strings.ToLower(domainName)

	// Get domain
	domain, err := uc.domainRepo.GetByDomain(ctx, domainName)
	if err != nil {
		return nil, fmt.Errorf("domain not found: %w", err)
	}

	// Try to find specific email address
	emailAddress, err := uc.emailRepo.GetByLocalPartAndDomain(ctx, localPart, domain.ID)
	if err != nil {
		return nil, fmt.Errorf("email address not found: %w", err)
	}

	return emailAddress, nil
}

func (uc *UseCase) DeleteEmailAddress(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return fmt.Errorf("email address ID is required")
	}

	if err := uc.emailRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete email address: %w", err)
	}

	return nil
}

func (uc *UseCase) ProcessIncomingEmail(ctx context.Context, req ProcessIncomingEmailInput) error {
	if req.EmailAddressID == uuid.Nil {
		return fmt.Errorf("email address ID is required")
	}

	if req.FromAddress == "" {
		return fmt.Errorf("from address is required")
	}

	if req.Body == "" {
		return fmt.Errorf("body is required")
	}

	// Create received email record
	var subject *string
	if req.Subject != "" {
		subject = &req.Subject
	}

	receivedEmail := &entities.ReceivedEmail{
		ID:             uuid.Must(uuid.NewV4()),
		EmailAddressID: &req.EmailAddressID,
		FromAddress:    req.FromAddress,
		Subject:        subject,
		EncryptedBody:  req.Body,
		ReceivedAt:     time.Now().UTC(),
	}

	// Set additional fields for webhook payload
	if req.Domain != nil {
		receivedEmail.DomainName = req.Domain.Domain
	}
	if req.EmailAddress != nil {
		receivedEmail.EmailAddress = req.EmailAddress.LocalPart + "@" + receivedEmail.DomainName
	}

	if !receivedEmail.IsValid() {
		return fmt.Errorf("invalid received email data")
	}

	if err := uc.receivedEmailRepo.Create(ctx, receivedEmail); err != nil {
		return fmt.Errorf("failed to store received email: %w", err)
	}

	// Trigger webhook if configured (don't block email processing on webhook failures)
	if uc.webhookNotifier != nil && req.Domain != nil && req.EmailAddress != nil {
		go func() {
			// Use a separate context with timeout for webhook delivery
			webhookCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := uc.webhookNotifier.NotifyIncomingEmail(
				webhookCtx,
				receivedEmail,
				req.Domain,
				req.EmailAddress,
				req.VerificationResults,
				req.AutoCreated,
			); err != nil {
				// Log webhook failure but don't fail email processing
				// The webhook system should handle retries internally
			}
		}()
	}

	// Forward email if the destination address has forwarding enabled
	if uc.emailForwarder != nil && req.EmailAddress != nil && req.EmailAddress.HasForwarding() {
		go func() {
			fwdCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			recipientAddr := ""
			if req.Domain != nil {
				recipientAddr = req.EmailAddress.LocalPart + "@" + req.Domain.Domain
			}

			if err := uc.emailForwarder.ForwardEmail(
				fwdCtx,
				req.FromAddress,
				recipientAddr,
				req.Subject,
				req.EmailAddress.ForwardAddresses,
			); err != nil {
				// Log forwarding failure but never affect the original email storage
				slog.Warn("Failed to forward email",
					"error", err,
					"from", req.FromAddress,
					"recipient", recipientAddr,
					"forward_count", len(req.EmailAddress.ForwardAddresses),
				)
			}
		}()
	}

	return nil
}

func (uc *UseCase) GetReceivedEmails(ctx context.Context, emailAddressID uuid.UUID, limit, offset int) ([]*entities.ReceivedEmail, error) {
	if emailAddressID == uuid.Nil {
		return nil, fmt.Errorf("email address ID is required")
	}

	if limit <= 0 {
		limit = 50 // Default limit
	}
	if limit > 1000 {
		limit = 1000 // Maximum limit
	}

	emails, err := uc.receivedEmailRepo.GetByEmailAddressID(ctx, emailAddressID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get received emails: %w", err)
	}

	return emails, nil
}

func (uc *UseCase) GetReceivedEmailByID(ctx context.Context, receivedEmailID uuid.UUID, userID uuid.UUID) (*entities.ReceivedEmail, error) {
	if receivedEmailID == uuid.Nil {
		return nil, fmt.Errorf("received email ID is required")
	}

	if userID == uuid.Nil {
		return nil, fmt.Errorf("user ID is required")
	}

	// Get the received email
	receivedEmail, err := uc.receivedEmailRepo.GetByID(ctx, receivedEmailID)
	if err != nil {
		return nil, fmt.Errorf("failed to get received email: %w", err)
	}

	// Verify that this email belongs to an email address owned by the user
	if receivedEmail.EmailAddressID == nil {
		return nil, fmt.Errorf("received email has no associated email address")
	}

	emailAddress, err := uc.emailRepo.GetByID(ctx, *receivedEmail.EmailAddressID)
	if err != nil {
		return nil, fmt.Errorf("failed to get email address: %w", err)
	}

	// Get the domain to verify ownership
	domain, err := uc.domainRepo.GetByID(ctx, emailAddress.DomainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get domain: %w", err)
	}

	// Check if the user owns this domain
	if domain.UserID != userID {
		return nil, fmt.Errorf("access denied: email does not belong to user")
	}

	receivedEmail.DomainName = domain.Domain
	receivedEmail.EmailAddress = emailAddress.LocalPart + "@" + domain.Domain

	return receivedEmail, nil
}

func (uc *UseCase) DeleteReceivedEmail(ctx context.Context, receivedEmailID uuid.UUID, userID uuid.UUID) error {
	if receivedEmailID == uuid.Nil {
		return fmt.Errorf("received email ID is required")
	}

	if userID == uuid.Nil {
		return fmt.Errorf("user ID is required")
	}

	// Fetch the received email to verify ownership via its email address → domain → user
	receivedEmail, err := uc.receivedEmailRepo.GetByID(ctx, receivedEmailID)
	if err != nil {
		return fmt.Errorf("failed to get received email: %w", err)
	}

	if receivedEmail.EmailAddressID == nil {
		return fmt.Errorf("received email has no associated email address")
	}

	emailAddress, err := uc.emailRepo.GetByID(ctx, *receivedEmail.EmailAddressID)
	if err != nil {
		return fmt.Errorf("failed to get email address: %w", err)
	}

	domain, err := uc.domainRepo.GetByID(ctx, emailAddress.DomainID)
	if err != nil {
		return fmt.Errorf("failed to get domain: %w", err)
	}

	if domain.UserID != userID {
		return fmt.Errorf("access denied: email does not belong to user")
	}

	if err := uc.receivedEmailRepo.Delete(ctx, receivedEmailID); err != nil {
		return fmt.Errorf("failed to delete received email: %w", err)
	}

	return nil
}

func (uc *UseCase) GetReceivedEmailsByUser(ctx context.Context, userID uuid.UUID, limit, offset int, filter GetReceivedEmailsFilter) ([]*entities.ReceivedEmail, int, error) {
	if userID == uuid.Nil {
		return nil, 0, fmt.Errorf("user ID is required")
	}

	if limit <= 0 {
		limit = 50 // Default limit
	}
	if limit > 1000 {
		limit = 1000 // Maximum limit
	}

	// Set default sort if not specified
	if filter.SortBy == "" {
		filter.SortBy = "received_at"
	}
	if filter.SortOrder == "" {
		filter.SortOrder = "desc"
	}

	// Validate sort options
	validSortFields := map[string]bool{
		"received_at":     true,
		"sequence_number": true,
		"from_address":    true,
		"subject":         true,
	}
	if !validSortFields[filter.SortBy] {
		return nil, 0, fmt.Errorf("invalid sort field: %s", filter.SortBy)
	}

	validSortOrders := map[string]bool{
		"asc":  true,
		"desc": true,
	}
	if !validSortOrders[filter.SortOrder] {
		return nil, 0, fmt.Errorf("invalid sort order: %s", filter.SortOrder)
	}

	// Validate security status if provided
	if filter.SecurityStatus != "" {
		validSecurityStatus := map[string]bool{
			"clean":       true,
			"suspicious":  true,
			"quarantined": true,
			"high_risk":   true,
		}
		if !validSecurityStatus[filter.SecurityStatus] {
			return nil, 0, fmt.Errorf("invalid security status: %s", filter.SecurityStatus)
		}
	}

	// Get all emails for the user with filters
	emails, total, err := uc.receivedEmailRepo.GetByUserIDWithFilter(ctx, userID, limit, offset, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get received emails: %w", err)
	}

	return emails, total, nil
}

func (uc *UseCase) GetDomainByID(ctx context.Context, domainID uuid.UUID) (*entities.Domain, error) {
	if domainID == uuid.Nil {
		return nil, fmt.Errorf("domain ID is required")
	}

	domain, err := uc.domainRepo.GetByID(ctx, domainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get domain: %w", err)
	}

	return domain, nil
}

// Validation helpers
func isValidLocalPart(localPart string) bool {
	// Basic local part validation (simplified)
	if len(localPart) == 0 || len(localPart) > 64 {
		return false
	}

	// Must not start or end with dot
	if localPart[0] == '.' || localPart[len(localPart)-1] == '.' {
		return false
	}

	// Basic character validation
	localPartRegex := regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	return localPartRegex.MatchString(localPart)
}

func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}


package email

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"mailsafe/domain/entities"

	"github.com/gofrs/uuid/v5"
)

type UseCase struct {
	emailRepo         EmailAddressRepository
	receivedEmailRepo ReceivedEmailRepository
	domainRepo        DomainRepository
}

func NewUseCase(emailRepo EmailAddressRepository, receivedEmailRepo ReceivedEmailRepository, domainRepo DomainRepository) *UseCase {
	return &UseCase{
		emailRepo:         emailRepo,
		receivedEmailRepo: receivedEmailRepo,
		domainRepo:        domainRepo,
	}
}

type CreateEmailAddressInput struct {
	DomainID         uuid.UUID `json:"domain_id"`
	LocalPart        string    `json:"local_part"`
	IsCatchAll       bool      `json:"is_catch_all"`
	ForwardAddresses []string  `json:"forward_addresses,omitempty"`
}

type UpdateEmailAddressInput struct {
	IsCatchAll       *bool    `json:"is_catch_all,omitempty"`
	ForwardAddresses []string `json:"forward_addresses,omitempty"`
}

type ProcessIncomingEmailInput struct {
	EmailAddressID uuid.UUID `json:"email_address_id"`
	FromAddress    string    `json:"from_address"`
	Subject        string    `json:"subject"`
	Body           string    `json:"body"`
	DomainID       uuid.UUID `json:"domain_id"`
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

	// If this is a catch-all, ensure there isn't already one for this domain
	if req.IsCatchAll {
		existingCatchAll, err := uc.emailRepo.GetCatchAllByDomainID(ctx, req.DomainID)
		if err == nil && existingCatchAll != nil {
			return nil, fmt.Errorf("domain already has a catch-all email address")
		}
	}

	// Validate forward addresses
	for _, addr := range req.ForwardAddresses {
		if !isValidEmail(addr) {
			return nil, fmt.Errorf("invalid forward address: %s", addr)
		}
	}

	emailAddress := &entities.EmailAddress{
		ID:               uuid.Must(uuid.NewV4()),
		DomainID:         req.DomainID,
		LocalPart:        normalizedLocalPart,
		IsCatchAll:       req.IsCatchAll,
		ForwardAddresses: req.ForwardAddresses,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
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
	if req.IsCatchAll != nil {
		// If setting as catch-all, ensure there isn't already one for this domain
		if *req.IsCatchAll && !emailAddress.IsCatchAll {
			existingCatchAll, err := uc.emailRepo.GetCatchAllByDomainID(ctx, emailAddress.DomainID)
			if err == nil && existingCatchAll != nil && existingCatchAll.ID != emailAddress.ID {
				return nil, fmt.Errorf("domain already has a catch-all email address")
			}
		}
		emailAddress.IsCatchAll = *req.IsCatchAll
	}

	if req.ForwardAddresses != nil {
		// Validate forward addresses
		for _, addr := range req.ForwardAddresses {
			if !isValidEmail(addr) {
				return nil, fmt.Errorf("invalid forward address: %s", addr)
			}
		}
		emailAddress.ForwardAddresses = req.ForwardAddresses
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
	// Parse email address
	parts := strings.Split(fullAddress, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid email address format: %s", fullAddress)
	}

	localPart := strings.ToLower(parts[0])
	domainName := strings.ToLower(parts[1])

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
		EncryptedBody:  req.Body, // For now, store as-is. TODO: Add encryption
		ReceivedAt:     time.Now().UTC(),
	}

	if !receivedEmail.IsValid() {
		return fmt.Errorf("invalid received email data")
	}

	if err := uc.receivedEmailRepo.Create(ctx, receivedEmail); err != nil {
		return fmt.Errorf("failed to store received email: %w", err)
	}

	// TODO: Trigger webhook if configured
	// TODO: Forward email if configured

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

func isValidURL(url string) bool {
	urlRegex := regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)
	return urlRegex.MatchString(url)
}

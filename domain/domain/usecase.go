package domain

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"mailvault/domain/entities"
	"mailvault/domain/user"
	"mailvault/internal/encryption"

	"github.com/gofrs/uuid/v5"
)

type UseCase struct {
	repo     Repository
	userRepo user.Repository
}

func NewUseCase(repo Repository, userRepo user.Repository) *UseCase {
	return &UseCase{
		repo:     repo,
		userRepo: userRepo,
	}
}

type CreateDomainInput struct {
	UserID            uuid.UUID              `json:"user_id"`
	Domain            string                 `json:"domain"`
	PublicKey         string                 `json:"public_key"`
	StorageEnabled    *bool                  `json:"storage_enabled,omitempty"`
	AutoCreateAddress *bool                  `json:"auto_create_address,omitempty"`
	WebhookConfig     *entities.WebhookConfig `json:"webhook_config,omitempty"`
}

type UpdateDomainInput struct {
	PublicKey         *string `json:"public_key,omitempty"`
	StorageEnabled    *bool   `json:"storage_enabled,omitempty"`
	AutoCreateAddress *bool   `json:"auto_create_address,omitempty"`
}

func (uc *UseCase) CreateDomain(ctx context.Context, req CreateDomainInput) (*entities.Domain, error) {
	if req.UserID == uuid.Nil {
		return nil, fmt.Errorf("user ID is required")
	}

	if req.Domain == "" {
		return nil, fmt.Errorf("domain is required")
	}

	if req.PublicKey == "" {
		return nil, fmt.Errorf("public key is required")
	}

	// Validate public key format
	_, err := encryption.ParsePublicKey(req.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid public key format: %w", err)
	}

	// Validate domain format
	if !isValidDomain(req.Domain) {
		return nil, fmt.Errorf("invalid domain format: %s", req.Domain)
	}

	// Normalize domain (lowercase)
	normalizedDomain := strings.ToLower(req.Domain)

	// Get user to check account type and domain limits
	user, err := uc.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Check domain limit based on user plan
	userDomains, err := uc.repo.GetByUserID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user domains: %w", err)
	}

	domainLimit := user.GetDomainLimit()
	if domainLimit > 0 && len(userDomains) >= domainLimit {
		return nil, fmt.Errorf("domain limit exceeded: %s plan can have maximum %d domain(s), you currently have %d",
			user.UserPlan, domainLimit, len(userDomains))
	}

	// Check if domain already exists
	existing, err := uc.repo.GetByDomain(ctx, normalizedDomain)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("domain %s already exists", normalizedDomain)
	}

	// Default storage enabled to true if not specified
	storageEnabled := true
	if req.StorageEnabled != nil {
		storageEnabled = *req.StorageEnabled
	}

	// Default auto create address to false if not specified
	autoCreateAddress := false
	if req.AutoCreateAddress != nil {
		autoCreateAddress = *req.AutoCreateAddress
	}

	// Generate API key
	apiKey, err := generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	// Generate verification token immediately
	verificationToken, err := generateVerificationToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate verification token: %w", err)
	}

	domain := &entities.Domain{
		ID:                 uuid.Must(uuid.NewV4()),
		UserID:             req.UserID,
		Domain:             normalizedDomain,
		PublicKey:          req.PublicKey, // Use user-provided public key
		APIKey:             apiKey,
		StorageEnabled:     storageEnabled,
		AutoCreateAddress:  autoCreateAddress,
		WebhookConfig:      req.WebhookConfig,
		VerificationStatus: entities.VerificationStatusPending,
		VerificationToken:  verificationToken,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
	}

	if !domain.IsValid() {
		return nil, fmt.Errorf("invalid domain data")
	}

	if err := uc.repo.Create(ctx, domain); err != nil {
		return nil, fmt.Errorf("failed to create domain: %w", err)
	}

	return domain, nil
}

func (uc *UseCase) GetDomainByID(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("domain ID is required")
	}

	domain, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get domain: %w", err)
	}

	return domain, nil
}

func (uc *UseCase) GetDomainsByUserID(ctx context.Context, userID uuid.UUID) ([]*entities.Domain, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("user ID is required")
	}

	domains, err := uc.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get domains: %w", err)
	}

	return domains, nil
}

func (uc *UseCase) GetDomainByAPIKey(ctx context.Context, apiKey string) (*entities.Domain, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	domain, err := uc.repo.GetByAPIKey(ctx, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get domain by API key: %w", err)
	}

	return domain, nil
}

func (uc *UseCase) GetDomainByName(ctx context.Context, domainName string) (*entities.Domain, error) {
	if domainName == "" {
		return nil, fmt.Errorf("domain name is required")
	}

	domain, err := uc.repo.GetByDomain(ctx, strings.ToLower(domainName))
	if err != nil {
		return nil, fmt.Errorf("failed to get domain by name: %w", err)
	}

	return domain, nil
}

func (uc *UseCase) UpdateDomain(ctx context.Context, id uuid.UUID, req UpdateDomainInput) (*entities.Domain, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("domain ID is required")
	}

	domain, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get domain: %w", err)
	}

	// Update fields if provided
	if req.PublicKey != nil {
		domain.PublicKey = *req.PublicKey
	}
	if req.StorageEnabled != nil {
		domain.StorageEnabled = *req.StorageEnabled
	}
	if req.AutoCreateAddress != nil {
		domain.AutoCreateAddress = *req.AutoCreateAddress
	}

	domain.UpdatedAt = time.Now().UTC()

	if !domain.IsValid() {
		return nil, fmt.Errorf("invalid domain data after update")
	}

	if err := uc.repo.Update(ctx, domain); err != nil {
		return nil, fmt.Errorf("failed to update domain: %w", err)
	}

	return domain, nil
}

func (uc *UseCase) DeleteDomain(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	if id == uuid.Nil {
		return fmt.Errorf("domain ID is required")
	}

	if userID == uuid.Nil {
		return fmt.Errorf("user ID is required")
	}

	// Verify domain belongs to user
	domain, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get domain: %w", err)
	}

	if domain.UserID != userID {
		return fmt.Errorf("unauthorized: domain does not belong to user")
	}

	if err := uc.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete domain: %w", err)
	}

	return nil
}

// generateAPIKey generates a random API key
func generateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "pm_" + hex.EncodeToString(bytes), nil
}

// generateVerificationToken generates a random verification token
func generateVerificationToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// isValidDomain validates domain format
func isValidDomain(domain string) bool {
	// Basic domain validation regex
	domainRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)

	if !domainRegex.MatchString(domain) {
		return false
	}

	// Check length
	if len(domain) > 253 {
		return false
	}

	// Must contain at least one dot
	if !strings.Contains(domain, ".") {
		return false
	}

	return true
}

package domains

import (
	"context"
	"log/slog"

	billingdomain "mailvault/domain/billing"
	domainpkg "mailvault/domain/domain"
	"mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
)

//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/usecase.go . UseCase

// UseCase defines the behavior required by this package from the domain use case.
type UseCase interface {
	CreateDomain(ctx context.Context, req domainpkg.CreateDomainInput) (*entities.Domain, error)
	GetDomainsByUserID(ctx context.Context, userID uuid.UUID) ([]*entities.Domain, error)
	GetDomainByID(ctx context.Context, id uuid.UUID) (*entities.Domain, error)
	UpdateDomain(ctx context.Context, id uuid.UUID, req domainpkg.UpdateDomainInput) (*entities.Domain, error)
	DeleteDomain(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	GetDomainByAPIKey(ctx context.Context, apiKey string) (*entities.Domain, error)
	GetDomainByName(ctx context.Context, domainName string) (*entities.Domain, error)
}

// BillingUseCase defines the billing operations required by domain handlers.
type BillingUseCase interface {
	CheckLimit(ctx context.Context, userID uuid.UUID, metric entities.UsageMetric) (*billingdomain.CheckLimitResult, error)
	IncrementUsage(ctx context.Context, userID uuid.UUID, metric entities.UsageMetric, amount int64) error
}

// DomainsHandlers contains domain-related endpoints
type DomainsHandlers struct {
	domainUseCase  UseCase
	billingUseCase BillingUseCase
	logger         *slog.Logger
}

func NewDomainsHandlers(domainUseCase UseCase, billingUseCase BillingUseCase, logger *slog.Logger) *DomainsHandlers {
	return &DomainsHandlers{
		domainUseCase:  domainUseCase,
		billingUseCase: billingUseCase,
		logger:         logger,
	}
}

// WebhookConfigResult represents webhook configuration in a response
type WebhookConfigResult struct {
	URL     string            `json:"url"`
	Secret  string            `json:"secret,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Enabled bool              `json:"enabled"`
}

// DomainResult represents domain data in responses
type DomainResult struct {
	ID                string               `json:"id"`
	Domain            string               `json:"domain"`
	PublicKey         string               `json:"public_key"`
	APIKey            string               `json:"api_key"`
	StorageEnabled    bool                 `json:"storage_enabled"`
	AutoCreateAddress bool                 `json:"auto_create_address"`
	WebhookConfig     *WebhookConfigResult `json:"webhook_config,omitempty"`
	// Verification fields
	VerificationStatus      string `json:"verification_status"`
	VerificationToken       string `json:"verification_token,omitempty"`
	LastVerificationAttempt string `json:"last_verification_attempt,omitempty"`
	VerificationError       string `json:"verification_error,omitempty"`
	VerificationAttempts    int    `json:"verification_attempts"`
	NextVerificationAttempt string `json:"next_verification_attempt,omitempty"`
	CreatedAt               string `json:"created_at"`
	UpdatedAt               string `json:"updated_at"`
}

// mapDomainToResult converts domain entity to API result
func (h *DomainsHandlers) mapDomainToResult(domain *entities.Domain) *DomainResult {
	result := &DomainResult{
		ID:                domain.ID.String(),
		Domain:            domain.Domain,
		PublicKey:         domain.PublicKey,
		APIKey:            domain.APIKey,
		StorageEnabled:    domain.StorageEnabled,
		AutoCreateAddress: domain.AutoCreateAddress,
		// Verification fields
		VerificationStatus:   string(domain.VerificationStatus),
		VerificationToken:    domain.VerificationToken,
		VerificationError:    domain.VerificationError,
		VerificationAttempts: domain.VerificationAttempts,
		CreatedAt:            domain.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:            domain.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if domain.WebhookConfig != nil {
		result.WebhookConfig = &WebhookConfigResult{
			URL:     domain.WebhookConfig.URL,
			Secret:  domain.WebhookConfig.Secret,
			Headers: domain.WebhookConfig.Headers,
			Enabled: domain.WebhookConfig.Enabled,
		}
	}

	// Format verification attempt timestamps
	if !domain.LastVerificationAttempt.IsZero() {
		formatted := domain.LastVerificationAttempt.Format("2006-01-02T15:04:05Z07:00")
		result.LastVerificationAttempt = formatted
	}
	if !domain.NextVerificationAttempt.IsZero() {
		formatted := domain.NextVerificationAttempt.Format("2006-01-02T15:04:05Z07:00")
		result.NextVerificationAttempt = formatted
	}

	return result
}

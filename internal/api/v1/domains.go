package v1

import (
	"context"
	"encoding/json"
	"net/http"

	domainpkg "mailsafe/domain/domain"
	"mailsafe/domain/entities"

	"github.com/go-chi/chi/v5"
	"github.com/gofrs/uuid/v5"
)

//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/domain_usecase.go . DomainUseCase

// DomainUseCase defines the behavior required by this package from the domain use case.
type DomainUseCase interface {
	CreateDomain(ctx context.Context, req domainpkg.CreateDomainInput) (*entities.Domain, error)
	GetDomainsByUserID(ctx context.Context, userID uuid.UUID) ([]*entities.Domain, error)
	GetDomainByID(ctx context.Context, id uuid.UUID) (*entities.Domain, error)
	UpdateDomain(ctx context.Context, id uuid.UUID, req domainpkg.UpdateDomainInput) (*entities.Domain, error)
	DeleteDomain(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	GetDomainByAPIKey(ctx context.Context, apiKey string) (*entities.Domain, error)
	GetDomainByName(ctx context.Context, domainName string) (*entities.Domain, error)
}

// DomainsHandlers contains domain-related endpoints
type DomainsHandlers struct {
	domainUseCase DomainUseCase
}

func NewDomainsHandlers(domainUseCase DomainUseCase) *DomainsHandlers {
	return &DomainsHandlers{
		domainUseCase: domainUseCase,
	}
}

// CreateDomainRequest represents domain creation request
type CreateDomainRequest struct {
	Domain         string                `json:"domain" validate:"required"`
	PublicKey      string                `json:"public_key" validate:"required"`
	WebhookConfig  *WebhookConfigRequest `json:"webhook_config,omitempty"`
	StorageEnabled *bool                 `json:"storage_enabled,omitempty"`
}

// UpdateDomainRequest represents domain update request
type UpdateDomainRequest struct {
	PublicKey      *string               `json:"public_key,omitempty"`
	Verified       *bool                 `json:"verified,omitempty"`
	WebhookConfig  *WebhookConfigRequest `json:"webhook_config,omitempty"`
	StorageEnabled *bool                 `json:"storage_enabled,omitempty"`
}

// WebhookConfigRequest represents webhook configuration in requests
type WebhookConfigRequest struct {
	URL     string            `json:"url" validate:"required"`
	Secret  string            `json:"secret,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Enabled bool              `json:"enabled"`
}

// DomainResult represents domain data in responses
type DomainResult struct {
	ID             string               `json:"id"`
	Domain         string               `json:"domain"`
	PublicKey      string               `json:"public_key"`
	APIKey         string               `json:"api_key"`
	Verified       bool                 `json:"verified"`
	WebhookConfig  *WebhookConfigResult `json:"webhook_config,omitempty"`
	StorageEnabled bool                 `json:"storage_enabled"`
	CreatedAt      string               `json:"created_at"`
	UpdatedAt      string               `json:"updated_at"`
}

// WebhookConfigResult represents webhook configuration in responses
type WebhookConfigResult struct {
	URL     string            `json:"url"`
	Secret  string            `json:"secret,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Enabled bool              `json:"enabled"`
}

// CreateDomain creates a new domain
func (h *DomainsHandlers) CreateDomain(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r)
	if err != nil {
		errorResponse(w, r, http.StatusUnauthorized, err)
		return
	}

	var req CreateDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Convert webhook config
	var webhookConfig *entities.WebhookConfig
	if req.WebhookConfig != nil {
		webhookConfig = &entities.WebhookConfig{
			URL:     req.WebhookConfig.URL,
			Secret:  req.WebhookConfig.Secret,
			Headers: req.WebhookConfig.Headers,
			Enabled: req.WebhookConfig.Enabled,
		}
	}

	// Create domain
	domainEntity, err := h.domainUseCase.CreateDomain(r.Context(), domainpkg.CreateDomainInput{
		UserID:         userID,
		Domain:         req.Domain,
		PublicKey:      req.PublicKey,
		WebhookConfig:  webhookConfig,
		StorageEnabled: req.StorageEnabled,
	})
	if err != nil {
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	result := h.mapDomainToResult(domainEntity)
	createdResponse(w, r, result)
}

// GetDomains gets all domains for the authenticated user
func (h *DomainsHandlers) GetDomains(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r)
	if err != nil {
		errorResponse(w, r, http.StatusUnauthorized, err)
		return
	}

	domains, err := h.domainUseCase.GetDomainsByUserID(r.Context(), userID)
	if err != nil {
		errorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	results := make([]*DomainResult, len(domains))
	for i, domain := range domains {
		results[i] = h.mapDomainToResult(domain)
	}

	successResponse(w, r, results)
}

// GetDomain gets a specific domain by ID
func (h *DomainsHandlers) GetDomain(w http.ResponseWriter, r *http.Request) {
	domainIDStr := chi.URLParam(r, "id")
	domainID, err := parseUUID(domainIDStr)
	if err != nil {
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	userID, err := getUserIDFromContext(r)
	if err != nil {
		errorResponse(w, r, http.StatusUnauthorized, err)
		return
	}

	domain, err := h.domainUseCase.GetDomainByID(r.Context(), domainID)
	if err != nil {
		errorResponse(w, r, http.StatusNotFound, err)
		return
	}

	// Verify domain belongs to user
	if domain.UserID != userID {
		errorResponse(w, r, http.StatusForbidden, ErrUnauthorized)
		return
	}

	result := h.mapDomainToResult(domain)
	successResponse(w, r, result)
}

// UpdateDomain updates an existing domain
func (h *DomainsHandlers) UpdateDomain(w http.ResponseWriter, r *http.Request) {
	domainIDStr := chi.URLParam(r, "id")
	domainID, err := parseUUID(domainIDStr)
	if err != nil {
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	userID, err := getUserIDFromContext(r)
	if err != nil {
		errorResponse(w, r, http.StatusUnauthorized, err)
		return
	}

	var req UpdateDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Verify domain belongs to user
	domain, err := h.domainUseCase.GetDomainByID(r.Context(), domainID)
	if err != nil {
		errorResponse(w, r, http.StatusNotFound, err)
		return
	}

	if domain.UserID != userID {
		errorResponse(w, r, http.StatusForbidden, ErrUnauthorized)
		return
	}

	// Convert webhook config
	var webhookConfig *entities.WebhookConfig
	if req.WebhookConfig != nil {
		webhookConfig = &entities.WebhookConfig{
			URL:     req.WebhookConfig.URL,
			Secret:  req.WebhookConfig.Secret,
			Headers: req.WebhookConfig.Headers,
			Enabled: req.WebhookConfig.Enabled,
		}
	}

	// Update domain
	updatedDomain, err := h.domainUseCase.UpdateDomain(r.Context(), domainID, domainpkg.UpdateDomainInput{
		PublicKey:      req.PublicKey,
		Verified:       req.Verified,
		WebhookConfig:  webhookConfig,
		StorageEnabled: req.StorageEnabled,
	})
	if err != nil {
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	result := h.mapDomainToResult(updatedDomain)
	successResponse(w, r, result)
}

// DeleteDomain deletes a domain
func (h *DomainsHandlers) DeleteDomain(w http.ResponseWriter, r *http.Request) {
	domainIDStr := chi.URLParam(r, "id")
	domainID, err := parseUUID(domainIDStr)
	if err != nil {
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	userID, err := getUserIDFromContext(r)
	if err != nil {
		errorResponse(w, r, http.StatusUnauthorized, err)
		return
	}

	err = h.domainUseCase.DeleteDomain(r.Context(), domainID, userID)
	if err != nil {
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	noContentResponse(w, r)
}

// mapDomainToResult converts domain entity to API result
func (h *DomainsHandlers) mapDomainToResult(domain *entities.Domain) *DomainResult {
	result := &DomainResult{
		ID:             domain.ID.String(),
		Domain:         domain.Domain,
		PublicKey:      domain.PublicKey,
		APIKey:         domain.APIKey,
		Verified:       domain.Verified,
		StorageEnabled: domain.StorageEnabled,
		CreatedAt:      domain.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      domain.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if domain.WebhookConfig != nil {
		result.WebhookConfig = &WebhookConfigResult{
			URL:     domain.WebhookConfig.URL,
			Secret:  domain.WebhookConfig.Secret,
			Headers: domain.WebhookConfig.Headers,
			Enabled: domain.WebhookConfig.Enabled,
		}
	}

	return result
}

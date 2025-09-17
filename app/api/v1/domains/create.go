package domains

import (
	"encoding/json"
	"mailvault/app/api"
	domainpkg "mailvault/domain/domain"
	"mailvault/domain/entities"
	"net/http"
)

// CreateDomainRequest represents domain creation request
type CreateDomainRequest struct {
	Domain           string                `json:"domain" validate:"required,domain,min=1,max=253"`
	PublicKey        string                `json:"public_key" validate:"required,public_key,min=100"`
	WebhookConfig    *WebhookConfigRequest `json:"webhook_config,omitempty"`
	StorageEnabled   *bool                 `json:"storage_enabled,omitempty"`
	AutoCreateAddress *bool                `json:"auto_create_address,omitempty"`
}

// CreateDomain creates a new domain
// @Summary Create a new domain
// @Description Register a new domain for the authenticated user with user-provided encryption public key and optional webhook configuration
// @Tags Domains
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateDomainRequest true "Domain creation details"
// @Success 201 {object} DomainResult "Domain created successfully"
// @Failure 400 {object} models.ErrorResponseBody "Bad request"
// @Failure 401 {object} models.ErrorResponseBody "Unauthorized"
// @Router /domains [post]
func (h *DomainsHandlers) CreateDomain(w http.ResponseWriter, r *http.Request) {
	userID, err := api.GetUserIDFromContext(r)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusUnauthorized, err)
		return
	}

	var req CreateDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
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
		UserID:           userID,
		Domain:           req.Domain,
		PublicKey:        req.PublicKey,
		WebhookConfig:    webhookConfig,
		StorageEnabled:   req.StorageEnabled,
		AutoCreateAddress: req.AutoCreateAddress,
	})
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	result := h.mapDomainToResult(domainEntity)
	api.CreatedResponse(w, r, result)
}

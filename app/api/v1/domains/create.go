package domains

import (
	"encoding/json"
	"fmt"
	"mailvault/app/api"
	domainpkg "mailvault/domain/domain"
	"mailvault/domain/entities"
	"net/http"

	"github.com/go-chi/render"
)

// WebhookConfigRequest represents webhook configuration in a request
type WebhookConfigRequest struct {
	URL     string            `json:"url"`
	Secret  string            `json:"secret,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Enabled bool              `json:"enabled"`
}

// CreateDomainRequest represents domain creation request
type CreateDomainRequest struct {
	Domain            string                `json:"domain" validate:"required,domain,min=1,max=253"`
	PublicKey         string                `json:"public_key" validate:"required,public_key,min=100"`
	StorageEnabled    *bool                 `json:"storage_enabled,omitempty"`
	AutoCreateAddress *bool                 `json:"auto_create_address,omitempty"`
	WebhookConfig     *WebhookConfigRequest `json:"webhook_config,omitempty"`
}

// planLimitExceededResponse is the 402 response body for plan limit violations.
type planLimitExceededResponse struct {
	Error      string `json:"error"`
	Message    string `json:"message"`
	Current    int64  `json:"current"`
	Limit      int    `json:"limit"`
	UpgradeURL string `json:"upgrade_url"`
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
// @Failure 402 {object} planLimitExceededResponse "Plan limit exceeded"
// @Router /domains [post]
func (h *DomainsHandlers) CreateDomain(w http.ResponseWriter, r *http.Request) {
	userID, err := api.GetUserIDFromContext(r)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusUnauthorized, err)
		return
	}

	// Check domain creation limit before proceeding.
	limitResult, err := h.billingUseCase.CheckLimit(r.Context(), userID, entities.UsageMetricDomains)
	if err != nil {
		h.logger.Error("failed to check domain limit", "user_id", userID, "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}
	if !limitResult.Allowed {
		render.Status(r, http.StatusPaymentRequired)
		render.JSON(w, r, planLimitExceededResponse{
			Error:      "plan_limit_exceeded",
			Message:    fmt.Sprintf("domain limit reached (%d/%d). upgrade your plan to add more domains", limitResult.Current, limitResult.Limit),
			Current:    limitResult.Current,
			Limit:      limitResult.Limit,
			UpgradeURL: "/billing",
		})
		return
	}

	var req CreateDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Build webhook config if provided
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
		UserID:            userID,
		Domain:            req.Domain,
		PublicKey:         req.PublicKey,
		StorageEnabled:    req.StorageEnabled,
		AutoCreateAddress: req.AutoCreateAddress,
		WebhookConfig:     webhookConfig,
	})
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Increment domain usage counter after successful creation.
	if err := h.billingUseCase.IncrementUsage(r.Context(), userID, entities.UsageMetricDomains, 1); err != nil {
		h.logger.Error("failed to increment domain usage", "user_id", userID, "error", err)
		// Non-fatal: domain was created successfully, log and continue.
	}

	result := h.mapDomainToResult(domainEntity)
	api.CreatedResponse(w, r, result)
}

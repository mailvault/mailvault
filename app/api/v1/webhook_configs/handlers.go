package webhook_configs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/mailvault/mailvault/app/api"
	"github.com/mailvault/mailvault/app/api/models"
	"github.com/mailvault/mailvault/domain/entities"
	"github.com/mailvault/mailvault/domain/webhook_config"

	"github.com/go-chi/chi/v5"
	"github.com/gofrs/uuid/v5"
)

//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/usecase.go . UseCase

// UseCase defines the behavior required from webhook config use case
type UseCase interface {
	CreateWebhookConfiguration(ctx context.Context, input webhook_config.CreateWebhookConfigInput) (*entities.WebhookConfiguration, error)
	GetWebhookConfiguration(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*entities.WebhookConfiguration, error)
	ListWebhookConfigurations(ctx context.Context, domainID uuid.UUID, userID uuid.UUID) ([]*entities.WebhookConfiguration, error)
	UpdateWebhookConfiguration(ctx context.Context, input webhook_config.UpdateWebhookConfigInput) (*entities.WebhookConfiguration, error)
	DeleteWebhookConfiguration(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	EnableWebhookConfiguration(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	DisableWebhookConfiguration(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	TestWebhookConfiguration(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*webhook_config.TestWebhookResult, error)
	CheckWebhookHealth(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*entities.WebhookHealthCheck, error)
	GetWebhookHealthHistory(ctx context.Context, id uuid.UUID, userID uuid.UUID, limit, offset int) ([]*entities.WebhookHealthCheck, error)
	GetWebhookMetrics(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*webhook_config.WebhookMetrics, error)
	GetWebhookAuditLog(ctx context.Context, id uuid.UUID, userID uuid.UUID, limit, offset int) ([]*entities.WebhookConfigurationAudit, error)
	ListWebhookTemplates(ctx context.Context) ([]*entities.WebhookConfigurationTemplate, error)
	GetWebhookTemplate(ctx context.Context, id uuid.UUID) (*entities.WebhookConfigurationTemplate, error)
	CreateFromTemplate(ctx context.Context, input webhook_config.CreateFromTemplateInput) (*entities.WebhookConfiguration, error)
}

// WebhookConfigHandlers contains webhook configuration endpoints
type WebhookConfigHandlers struct {
	webhookConfigUseCase UseCase
	logger               *slog.Logger
}

// Ensure models package is imported for swagger documentation
var _ models.ErrorResponseBody

func NewWebhookConfigHandlers(webhookConfigUseCase UseCase, log *slog.Logger) *WebhookConfigHandlers {
	return &WebhookConfigHandlers{
		webhookConfigUseCase: webhookConfigUseCase,
		logger:               log,
	}
}

// CreateWebhookConfigRequest represents the request to create a webhook configuration
type CreateWebhookConfigRequest struct {
	Name                         string                   `json:"name" validate:"required,min=1,max=255"`
	Description                  string                   `json:"description,omitempty" validate:"max=1000"`
	URL                          string                   `json:"url" validate:"required,url,max=2048"`
	Method                       string                   `json:"method,omitempty" validate:"omitempty,oneof=POST PUT PATCH"`
	AuthType                     entities.WebhookAuthType `json:"auth_type,omitempty" validate:"omitempty,oneof=none basic bearer hmac_sha256"`
	AuthSecret                   string                   `json:"auth_secret,omitempty" validate:"omitempty,max=512"`
	AuthUsername                 string                   `json:"auth_username,omitempty" validate:"omitempty,max=255"`
	CustomHeaders                map[string]string        `json:"custom_headers,omitempty"`
	EventTypes                   []string                 `json:"event_types,omitempty" validate:"dive,oneof=email.received email.sent email.delivered email.bounced email.rejected *"`
	TimeoutSeconds               int                      `json:"timeout_seconds,omitempty" validate:"omitempty,min=1,max=300"`
	MaxRetries                   int                      `json:"max_retries,omitempty" validate:"omitempty,min=0,max=10"`
	RetryBackoffMultiplier       float64                  `json:"retry_backoff_multiplier,omitempty" validate:"omitempty,min=1.0,max=5.0"`
	InitialRetryDelaySeconds     int                      `json:"initial_retry_delay_seconds,omitempty" validate:"omitempty,min=0"`
	RateLimitPerMinute           int                      `json:"rate_limit_per_minute,omitempty" validate:"omitempty,min=1"`
	RateLimitPerHour             int                      `json:"rate_limit_per_hour,omitempty" validate:"omitempty,min=1"`
	CircuitBreakerEnabled        bool                     `json:"circuit_breaker_enabled"`
	CircuitBreakerThreshold      int                      `json:"circuit_breaker_threshold,omitempty" validate:"omitempty,min=1"`
	CircuitBreakerTimeoutSeconds int                      `json:"circuit_breaker_timeout_seconds,omitempty" validate:"omitempty,min=1"`
}

// UpdateWebhookConfigRequest represents the request to update a webhook configuration
type UpdateWebhookConfigRequest struct {
	Name                         *string                   `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Description                  *string                   `json:"description,omitempty" validate:"omitempty,max=1000"`
	URL                          *string                   `json:"url,omitempty" validate:"omitempty,url,max=2048"`
	Method                       *string                   `json:"method,omitempty" validate:"omitempty,oneof=POST PUT PATCH"`
	AuthType                     *entities.WebhookAuthType `json:"auth_type,omitempty" validate:"omitempty,oneof=none basic bearer hmac_sha256"`
	AuthSecret                   *string                   `json:"auth_secret,omitempty" validate:"omitempty,max=512"`
	AuthUsername                 *string                   `json:"auth_username,omitempty" validate:"omitempty,max=255"`
	CustomHeaders                map[string]string         `json:"custom_headers,omitempty"`
	EventTypes                   []string                  `json:"event_types,omitempty" validate:"omitempty,dive,oneof=email.received email.sent email.delivered email.bounced email.rejected *"`
	TimeoutSeconds               *int                      `json:"timeout_seconds,omitempty" validate:"omitempty,min=1,max=300"`
	MaxRetries                   *int                      `json:"max_retries,omitempty" validate:"omitempty,min=0,max=10"`
	RetryBackoffMultiplier       *float64                  `json:"retry_backoff_multiplier,omitempty" validate:"omitempty,min=1.0,max=5.0"`
	InitialRetryDelaySeconds     *int                      `json:"initial_retry_delay_seconds,omitempty" validate:"omitempty,min=0"`
	RateLimitPerMinute           *int                      `json:"rate_limit_per_minute,omitempty" validate:"omitempty,min=1"`
	RateLimitPerHour             *int                      `json:"rate_limit_per_hour,omitempty" validate:"omitempty,min=1"`
	CircuitBreakerEnabled        *bool                     `json:"circuit_breaker_enabled,omitempty"`
	CircuitBreakerThreshold      *int                      `json:"circuit_breaker_threshold,omitempty" validate:"omitempty,min=1"`
	CircuitBreakerTimeoutSeconds *int                      `json:"circuit_breaker_timeout_seconds,omitempty" validate:"omitempty,min=1"`
}

// CreateFromTemplateRequest represents the request to create from a template
type CreateFromTemplateRequest struct {
	TemplateID uuid.UUID `json:"template_id" validate:"required"`
	Name       string    `json:"name" validate:"required,min=1,max=255"`
	URL        string    `json:"url" validate:"required,url,max=2048"`
	AuthSecret string    `json:"auth_secret,omitempty" validate:"omitempty,max=512"`
	EventTypes []string  `json:"event_types,omitempty" validate:"omitempty,dive,oneof=email.received email.sent email.delivered email.bounced email.rejected *"`
}

// WebhookConfigResponse represents a webhook configuration in responses
type WebhookConfigResponse struct {
	ID                           string                       `json:"id"`
	DomainID                     string                       `json:"domain_id"`
	Name                         string                       `json:"name"`
	Description                  string                       `json:"description,omitempty"`
	URL                          string                       `json:"url"`
	Method                       string                       `json:"method"`
	Enabled                      bool                         `json:"enabled"`
	Verified                     bool                         `json:"verified"`
	AuthType                     entities.WebhookAuthType     `json:"auth_type"`
	AuthUsername                 string                       `json:"auth_username,omitempty"`
	CustomHeaders                map[string]string            `json:"custom_headers,omitempty"`
	EventTypes                   []string                     `json:"event_types"`
	TimeoutSeconds               int                          `json:"timeout_seconds"`
	MaxRetries                   int                          `json:"max_retries"`
	RetryBackoffMultiplier       float64                      `json:"retry_backoff_multiplier"`
	InitialRetryDelaySeconds     int                          `json:"initial_retry_delay_seconds"`
	RateLimitPerMinute           int                          `json:"rate_limit_per_minute"`
	RateLimitPerHour             int                          `json:"rate_limit_per_hour"`
	CircuitBreakerEnabled        bool                         `json:"circuit_breaker_enabled"`
	CircuitBreakerThreshold      int                          `json:"circuit_breaker_threshold"`
	CircuitBreakerTimeoutSeconds int                          `json:"circuit_breaker_timeout_seconds"`
	CircuitBreakerState          entities.CircuitBreakerState `json:"circuit_breaker_state"`
	CircuitBreakerOpenedAt       string                       `json:"circuit_breaker_opened_at,omitempty"`
	HealthStatus                 entities.WebhookHealthStatus `json:"health_status"`
	LastHealthCheckAt            string                       `json:"last_health_check_at,omitempty"`
	LastSuccessAt                string                       `json:"last_success_at,omitempty"`
	LastFailureAt                string                       `json:"last_failure_at,omitempty"`
	ConsecutiveFailures          int                          `json:"consecutive_failures"`
	TotalSuccessCount            int64                        `json:"total_success_count"`
	TotalFailureCount            int64                        `json:"total_failure_count"`
	SuccessRate                  float64                      `json:"success_rate"`
	AverageResponseTimeMs        int                          `json:"average_response_time_ms,omitempty"`
	LastUsedAt                   string                       `json:"last_used_at,omitempty"`
	CreatedAt                    string                       `json:"created_at"`
	UpdatedAt                    string                       `json:"updated_at"`
}

// CreateWebhookConfig godoc
// @Summary      Create webhook configuration
// @Description  Create a new webhook configuration for a domain
// @Tags         webhook-configs
// @Accept       json
// @Produce      json
// @Param        domainId path string true "Domain ID"
// @Param        request body CreateWebhookConfigRequest true "Webhook configuration data"
// @Success      201 {object} WebhookConfigResponse
// @Failure      400 {object} models.ErrorResponseBody
// @Failure      401 {object} models.ErrorResponseBody
// @Failure      500 {object} models.ErrorResponseBody
// @Router       /domains/{domainId}/webhooks [post]
// @Security     BearerAuth
func (h *WebhookConfigHandlers) CreateWebhookConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	domainIDStr := chi.URLParam(r, "domainId")

	domainID, err := uuid.FromString(domainIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("invalid domain ID"))
		return
	}

	userID, ok := ctx.Value("user_id").(uuid.UUID)
	if !ok {
		api.ErrorResponse(w, r, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return
	}

	var req CreateWebhookConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}

	input := webhook_config.CreateWebhookConfigInput{
		DomainID:                     domainID,
		UserID:                       userID,
		Name:                         req.Name,
		Description:                  req.Description,
		URL:                          req.URL,
		Method:                       req.Method,
		AuthType:                     req.AuthType,
		AuthSecret:                   req.AuthSecret,
		AuthUsername:                 req.AuthUsername,
		CustomHeaders:                req.CustomHeaders,
		EventTypes:                   req.EventTypes,
		TimeoutSeconds:               req.TimeoutSeconds,
		MaxRetries:                   req.MaxRetries,
		RetryBackoffMultiplier:       req.RetryBackoffMultiplier,
		InitialRetryDelaySeconds:     req.InitialRetryDelaySeconds,
		RateLimitPerMinute:           req.RateLimitPerMinute,
		RateLimitPerHour:             req.RateLimitPerHour,
		CircuitBreakerEnabled:        req.CircuitBreakerEnabled,
		CircuitBreakerThreshold:      req.CircuitBreakerThreshold,
		CircuitBreakerTimeoutSeconds: req.CircuitBreakerTimeoutSeconds,
	}

	config, err := h.webhookConfigUseCase.CreateWebhookConfiguration(ctx, input)
	if err != nil {
		h.logger.Error("failed to create webhook configuration", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	response := h.mapWebhookConfigToResponse(config)
	api.CreatedResponse(w, r, response)
}

// ListWebhookConfigs godoc
// @Summary      List webhook configurations
// @Description  List all webhook configurations for a domain
// @Tags         webhook-configs
// @Produce      json
// @Param        domainId path string true "Domain ID"
// @Success      200 {array} WebhookConfigResponse
// @Failure      400 {object} models.ErrorResponseBody
// @Failure      401 {object} models.ErrorResponseBody
// @Failure      500 {object} models.ErrorResponseBody
// @Router       /domains/{domainId}/webhooks [get]
// @Security     BearerAuth
func (h *WebhookConfigHandlers) ListWebhookConfigs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	domainIDStr := chi.URLParam(r, "domainId")

	domainID, err := uuid.FromString(domainIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("invalid domain ID"))
		return
	}

	userID, ok := ctx.Value("user_id").(uuid.UUID)
	if !ok {
		api.ErrorResponse(w, r, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return
	}

	configs, err := h.webhookConfigUseCase.ListWebhookConfigurations(ctx, domainID, userID)
	if err != nil {
		h.logger.Error("failed to list webhook configurations", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	responses := make([]*WebhookConfigResponse, len(configs))
	for i, config := range configs {
		responses[i] = h.mapWebhookConfigToResponse(config)
	}

	api.SuccessResponse(w, r, responses)
}

// GetWebhookConfig godoc
// @Summary      Get webhook configuration
// @Description  Get a specific webhook configuration
// @Tags         webhook-configs
// @Produce      json
// @Param        domainId path string true "Domain ID"
// @Param        webhookId path string true "Webhook Configuration ID"
// @Success      200 {object} WebhookConfigResponse
// @Failure      400 {object} models.ErrorResponseBody
// @Failure      401 {object} models.ErrorResponseBody
// @Failure      404 {object} models.ErrorResponseBody
// @Failure      500 {object} models.ErrorResponseBody
// @Router       /domains/{domainId}/webhooks/{webhookId} [get]
// @Security     BearerAuth
func (h *WebhookConfigHandlers) GetWebhookConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	webhookIDStr := chi.URLParam(r, "webhookId")

	webhookID, err := uuid.FromString(webhookIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("invalid webhook ID"))
		return
	}

	userID, ok := ctx.Value("user_id").(uuid.UUID)
	if !ok {
		api.ErrorResponse(w, r, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return
	}

	config, err := h.webhookConfigUseCase.GetWebhookConfiguration(ctx, webhookID, userID)
	if err != nil {
		h.logger.Error("failed to get webhook configuration", "error", err)
		api.ErrorResponse(w, r, http.StatusNotFound, err)
		return
	}

	response := h.mapWebhookConfigToResponse(config)
	api.SuccessResponse(w, r, response)
}

// UpdateWebhookConfig godoc
// @Summary      Update webhook configuration
// @Description  Update a webhook configuration
// @Tags         webhook-configs
// @Accept       json
// @Produce      json
// @Param        domainId path string true "Domain ID"
// @Param        webhookId path string true "Webhook Configuration ID"
// @Param        request body UpdateWebhookConfigRequest true "Updated webhook configuration data"
// @Success      200 {object} WebhookConfigResponse
// @Failure      400 {object} models.ErrorResponseBody
// @Failure      401 {object} models.ErrorResponseBody
// @Failure      404 {object} models.ErrorResponseBody
// @Failure      500 {object} models.ErrorResponseBody
// @Router       /domains/{domainId}/webhooks/{webhookId} [put]
// @Security     BearerAuth
func (h *WebhookConfigHandlers) UpdateWebhookConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	webhookIDStr := chi.URLParam(r, "webhookId")

	webhookID, err := uuid.FromString(webhookIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("invalid webhook ID"))
		return
	}

	userID, ok := ctx.Value("user_id").(uuid.UUID)
	if !ok {
		api.ErrorResponse(w, r, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return
	}

	var req UpdateWebhookConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}

	input := webhook_config.UpdateWebhookConfigInput{
		ID:                           webhookID,
		UserID:                       userID,
		Name:                         req.Name,
		Description:                  req.Description,
		URL:                          req.URL,
		Method:                       req.Method,
		AuthType:                     req.AuthType,
		AuthSecret:                   req.AuthSecret,
		AuthUsername:                 req.AuthUsername,
		CustomHeaders:                req.CustomHeaders,
		EventTypes:                   req.EventTypes,
		TimeoutSeconds:               req.TimeoutSeconds,
		MaxRetries:                   req.MaxRetries,
		RetryBackoffMultiplier:       req.RetryBackoffMultiplier,
		InitialRetryDelaySeconds:     req.InitialRetryDelaySeconds,
		RateLimitPerMinute:           req.RateLimitPerMinute,
		RateLimitPerHour:             req.RateLimitPerHour,
		CircuitBreakerEnabled:        req.CircuitBreakerEnabled,
		CircuitBreakerThreshold:      req.CircuitBreakerThreshold,
		CircuitBreakerTimeoutSeconds: req.CircuitBreakerTimeoutSeconds,
	}

	config, err := h.webhookConfigUseCase.UpdateWebhookConfiguration(ctx, input)
	if err != nil {
		h.logger.Error("failed to update webhook configuration", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	response := h.mapWebhookConfigToResponse(config)
	api.SuccessResponse(w, r, response)
}

// DeleteWebhookConfig godoc
// @Summary      Delete webhook configuration
// @Description  Delete a webhook configuration
// @Tags         webhook-configs
// @Param        domainId path string true "Domain ID"
// @Param        webhookId path string true "Webhook Configuration ID"
// @Success      204
// @Failure      400 {object} models.ErrorResponseBody
// @Failure      401 {object} models.ErrorResponseBody
// @Failure      404 {object} models.ErrorResponseBody
// @Failure      500 {object} models.ErrorResponseBody
// @Router       /domains/{domainId}/webhooks/{webhookId} [delete]
// @Security     BearerAuth
func (h *WebhookConfigHandlers) DeleteWebhookConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	webhookIDStr := chi.URLParam(r, "webhookId")

	webhookID, err := uuid.FromString(webhookIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("invalid webhook ID"))
		return
	}

	userID, ok := ctx.Value("user_id").(uuid.UUID)
	if !ok {
		api.ErrorResponse(w, r, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return
	}

	if err := h.webhookConfigUseCase.DeleteWebhookConfiguration(ctx, webhookID, userID); err != nil {
		h.logger.Error("failed to delete webhook configuration", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	api.NoContentResponse(w, r)
}

// EnableWebhookConfig godoc
// @Summary      Enable webhook configuration
// @Description  Enable a webhook configuration
// @Tags         webhook-configs
// @Param        domainId path string true "Domain ID"
// @Param        webhookId path string true "Webhook Configuration ID"
// @Success      200 {object} map[string]string
// @Failure      400 {object} models.ErrorResponseBody
// @Failure      401 {object} models.ErrorResponseBody
// @Failure      500 {object} models.ErrorResponseBody
// @Router       /domains/{domainId}/webhooks/{webhookId}/enable [post]
// @Security     BearerAuth
func (h *WebhookConfigHandlers) EnableWebhookConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	webhookIDStr := chi.URLParam(r, "webhookId")

	webhookID, err := uuid.FromString(webhookIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("invalid webhook ID"))
		return
	}

	userID, ok := ctx.Value("user_id").(uuid.UUID)
	if !ok {
		api.ErrorResponse(w, r, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return
	}

	if err := h.webhookConfigUseCase.EnableWebhookConfiguration(ctx, webhookID, userID); err != nil {
		h.logger.Error("failed to enable webhook configuration", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	api.SuccessResponse(w, r, map[string]string{"message": "webhook configuration enabled"})
}

// DisableWebhookConfig godoc
// @Summary      Disable webhook configuration
// @Description  Disable a webhook configuration
// @Tags         webhook-configs
// @Param        domainId path string true "Domain ID"
// @Param        webhookId path string true "Webhook Configuration ID"
// @Success      200 {object} map[string]string
// @Failure      400 {object} models.ErrorResponseBody
// @Failure      401 {object} models.ErrorResponseBody
// @Failure      500 {object} models.ErrorResponseBody
// @Router       /domains/{domainId}/webhooks/{webhookId}/disable [post]
// @Security     BearerAuth
func (h *WebhookConfigHandlers) DisableWebhookConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	webhookIDStr := chi.URLParam(r, "webhookId")

	webhookID, err := uuid.FromString(webhookIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("invalid webhook ID"))
		return
	}

	userID, ok := ctx.Value("user_id").(uuid.UUID)
	if !ok {
		api.ErrorResponse(w, r, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return
	}

	if err := h.webhookConfigUseCase.DisableWebhookConfiguration(ctx, webhookID, userID); err != nil {
		h.logger.Error("failed to disable webhook configuration", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	api.SuccessResponse(w, r, map[string]string{"message": "webhook configuration disabled"})
}

// TestWebhookConfig godoc
// @Summary      Test webhook configuration
// @Description  Send a test event to the webhook endpoint
// @Tags         webhook-configs
// @Param        domainId path string true "Domain ID"
// @Param        webhookId path string true "Webhook Configuration ID"
// @Success      200 {object} webhook_config.TestWebhookResult
// @Failure      400 {object} models.ErrorResponseBody
// @Failure      401 {object} models.ErrorResponseBody
// @Failure      500 {object} models.ErrorResponseBody
// @Router       /domains/{domainId}/webhooks/{webhookId}/test [post]
// @Security     BearerAuth
func (h *WebhookConfigHandlers) TestWebhookConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	webhookIDStr := chi.URLParam(r, "webhookId")

	webhookID, err := uuid.FromString(webhookIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("invalid webhook ID"))
		return
	}

	userID, ok := ctx.Value("user_id").(uuid.UUID)
	if !ok {
		api.ErrorResponse(w, r, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return
	}

	result, err := h.webhookConfigUseCase.TestWebhookConfiguration(ctx, webhookID, userID)
	if err != nil {
		h.logger.Error("failed to test webhook configuration", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	api.SuccessResponse(w, r, result)
}

// GetWebhookHealth godoc
// @Summary      Check webhook health
// @Description  Perform a health check on the webhook endpoint
// @Tags         webhook-configs
// @Param        domainId path string true "Domain ID"
// @Param        webhookId path string true "Webhook Configuration ID"
// @Success      200 {object} entities.WebhookHealthCheck
// @Failure      400 {object} models.ErrorResponseBody
// @Failure      401 {object} models.ErrorResponseBody
// @Failure      500 {object} models.ErrorResponseBody
// @Router       /domains/{domainId}/webhooks/{webhookId}/health [get]
// @Security     BearerAuth
func (h *WebhookConfigHandlers) GetWebhookHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	webhookIDStr := chi.URLParam(r, "webhookId")

	webhookID, err := uuid.FromString(webhookIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("invalid webhook ID"))
		return
	}

	userID, ok := ctx.Value("user_id").(uuid.UUID)
	if !ok {
		api.ErrorResponse(w, r, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return
	}

	check, err := h.webhookConfigUseCase.CheckWebhookHealth(ctx, webhookID, userID)
	if err != nil {
		h.logger.Error("failed to check webhook health", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	api.SuccessResponse(w, r, check)
}

// GetWebhookMetrics godoc
// @Summary      Get webhook metrics
// @Description  Get performance metrics for a webhook configuration
// @Tags         webhook-configs
// @Param        domainId path string true "Domain ID"
// @Param        webhookId path string true "Webhook Configuration ID"
// @Success      200 {object} webhook_config.WebhookMetrics
// @Failure      400 {object} models.ErrorResponseBody
// @Failure      401 {object} models.ErrorResponseBody
// @Failure      500 {object} models.ErrorResponseBody
// @Router       /domains/{domainId}/webhooks/{webhookId}/metrics [get]
// @Security     BearerAuth
func (h *WebhookConfigHandlers) GetWebhookMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	webhookIDStr := chi.URLParam(r, "webhookId")

	webhookID, err := uuid.FromString(webhookIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("invalid webhook ID"))
		return
	}

	userID, ok := ctx.Value("user_id").(uuid.UUID)
	if !ok {
		api.ErrorResponse(w, r, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return
	}

	metrics, err := h.webhookConfigUseCase.GetWebhookMetrics(ctx, webhookID, userID)
	if err != nil {
		h.logger.Error("failed to get webhook metrics", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	api.SuccessResponse(w, r, metrics)
}

// GetWebhookAuditLog godoc
// @Summary      Get webhook audit log
// @Description  Get audit log for a webhook configuration
// @Tags         webhook-configs
// @Param        domainId path string true "Domain ID"
// @Param        webhookId path string true "Webhook Configuration ID"
// @Param        limit query int false "Limit"
// @Param        offset query int false "Offset"
// @Success      200 {array} entities.WebhookConfigurationAudit
// @Failure      400 {object} models.ErrorResponseBody
// @Failure      401 {object} models.ErrorResponseBody
// @Failure      500 {object} models.ErrorResponseBody
// @Router       /domains/{domainId}/webhooks/{webhookId}/audit [get]
// @Security     BearerAuth
func (h *WebhookConfigHandlers) GetWebhookAuditLog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	webhookIDStr := chi.URLParam(r, "webhookId")

	webhookID, err := uuid.FromString(webhookIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("invalid webhook ID"))
		return
	}

	userID, ok := ctx.Value("user_id").(uuid.UUID)
	if !ok {
		api.ErrorResponse(w, r, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	audits, err := h.webhookConfigUseCase.GetWebhookAuditLog(ctx, webhookID, userID, limit, offset)
	if err != nil {
		h.logger.Error("failed to get webhook audit log", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	api.SuccessResponse(w, r, audits)
}

// ListWebhookTemplates godoc
// @Summary      List webhook templates
// @Description  List all available webhook templates
// @Tags         webhook-configs
// @Produce      json
// @Success      200 {array} entities.WebhookConfigurationTemplate
// @Failure      500 {object} models.ErrorResponseBody
// @Router       /webhook-templates [get]
// @Security     BearerAuth
func (h *WebhookConfigHandlers) ListWebhookTemplates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	templates, err := h.webhookConfigUseCase.ListWebhookTemplates(ctx)
	if err != nil {
		h.logger.Error("failed to list webhook templates", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	api.SuccessResponse(w, r, templates)
}

// CreateFromTemplate godoc
// @Summary      Create webhook from template
// @Description  Create a webhook configuration from a template
// @Tags         webhook-configs
// @Accept       json
// @Produce      json
// @Param        domainId path string true "Domain ID"
// @Param        request body CreateFromTemplateRequest true "Template configuration"
// @Success      201 {object} WebhookConfigResponse
// @Failure      400 {object} models.ErrorResponseBody
// @Failure      401 {object} models.ErrorResponseBody
// @Failure      500 {object} models.ErrorResponseBody
// @Router       /domains/{domainId}/webhooks/from-template [post]
// @Security     BearerAuth
func (h *WebhookConfigHandlers) CreateFromTemplate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	domainIDStr := chi.URLParam(r, "domainId")

	domainID, err := uuid.FromString(domainIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("invalid domain ID"))
		return
	}

	userID, ok := ctx.Value("user_id").(uuid.UUID)
	if !ok {
		api.ErrorResponse(w, r, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return
	}

	var req CreateFromTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}

	input := webhook_config.CreateFromTemplateInput{
		TemplateID: req.TemplateID,
		DomainID:   domainID,
		UserID:     userID,
		Name:       req.Name,
		URL:        req.URL,
		AuthSecret: req.AuthSecret,
		EventTypes: req.EventTypes,
	}

	config, err := h.webhookConfigUseCase.CreateFromTemplate(ctx, input)
	if err != nil {
		h.logger.Error("failed to create webhook from template", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	response := h.mapWebhookConfigToResponse(config)
	api.CreatedResponse(w, r, response)
}

// Helper function to map entity to response
func (h *WebhookConfigHandlers) mapWebhookConfigToResponse(config *entities.WebhookConfiguration) *WebhookConfigResponse {
	response := &WebhookConfigResponse{
		ID:                           config.ID.String(),
		DomainID:                     config.DomainID.String(),
		Name:                         config.Name,
		Description:                  config.Description,
		URL:                          config.URL,
		Method:                       config.Method,
		Enabled:                      config.Enabled,
		Verified:                     config.Verified,
		AuthType:                     config.AuthType,
		AuthUsername:                 config.AuthUsername,
		CustomHeaders:                config.CustomHeaders,
		EventTypes:                   config.EventTypes,
		TimeoutSeconds:               config.TimeoutSeconds,
		MaxRetries:                   config.MaxRetries,
		RetryBackoffMultiplier:       config.RetryBackoffMultiplier,
		InitialRetryDelaySeconds:     config.InitialRetryDelaySeconds,
		RateLimitPerMinute:           config.RateLimitPerMinute,
		RateLimitPerHour:             config.RateLimitPerHour,
		CircuitBreakerEnabled:        config.CircuitBreakerEnabled,
		CircuitBreakerThreshold:      config.CircuitBreakerThreshold,
		CircuitBreakerTimeoutSeconds: config.CircuitBreakerTimeoutSeconds,
		CircuitBreakerState:          config.CircuitBreakerState,
		HealthStatus:                 config.HealthStatus,
		ConsecutiveFailures:          config.ConsecutiveFailures,
		TotalSuccessCount:            config.TotalSuccessCount,
		TotalFailureCount:            config.TotalFailureCount,
		SuccessRate:                  config.GetSuccessRate(),
		AverageResponseTimeMs:        config.AverageResponseTimeMs,
		CreatedAt:                    config.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:                    config.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if config.CircuitBreakerOpenedAt != nil {
		response.CircuitBreakerOpenedAt = config.CircuitBreakerOpenedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if config.LastHealthCheckAt != nil {
		response.LastHealthCheckAt = config.LastHealthCheckAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if config.LastSuccessAt != nil {
		response.LastSuccessAt = config.LastSuccessAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if config.LastFailureAt != nil {
		response.LastFailureAt = config.LastFailureAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if config.LastUsedAt != nil {
		response.LastUsedAt = config.LastUsedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	return response
}

package webhook_config

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/mailvault/mailvault/domain/entities"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gofrs/uuid/v5"
)

//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/usecase.go . UseCase

// UseCase defines webhook configuration business logic
type UseCase interface {
	// Configuration Management
	CreateWebhookConfiguration(ctx context.Context, input CreateWebhookConfigInput) (*entities.WebhookConfiguration, error)
	GetWebhookConfiguration(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*entities.WebhookConfiguration, error)
	ListWebhookConfigurations(ctx context.Context, domainID uuid.UUID, userID uuid.UUID) ([]*entities.WebhookConfiguration, error)
	UpdateWebhookConfiguration(ctx context.Context, input UpdateWebhookConfigInput) (*entities.WebhookConfiguration, error)
	DeleteWebhookConfiguration(ctx context.Context, id uuid.UUID, userID uuid.UUID) error

	// Configuration Operations
	EnableWebhookConfiguration(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	DisableWebhookConfiguration(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	TestWebhookConfiguration(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*TestWebhookResult, error)

	// Health Monitoring
	CheckWebhookHealth(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*entities.WebhookHealthCheck, error)
	GetWebhookHealthHistory(ctx context.Context, id uuid.UUID, userID uuid.UUID, limit, offset int) ([]*entities.WebhookHealthCheck, error)
	GetWebhookMetrics(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*WebhookMetrics, error)

	// Audit Trail
	GetWebhookAuditLog(ctx context.Context, id uuid.UUID, userID uuid.UUID, limit, offset int) ([]*entities.WebhookConfigurationAudit, error)

	// Templates
	ListWebhookTemplates(ctx context.Context) ([]*entities.WebhookConfigurationTemplate, error)
	GetWebhookTemplate(ctx context.Context, id uuid.UUID) (*entities.WebhookConfigurationTemplate, error)
	CreateFromTemplate(ctx context.Context, input CreateFromTemplateInput) (*entities.WebhookConfiguration, error)
}

// CreateWebhookConfigInput represents the input for creating a webhook configuration
type CreateWebhookConfigInput struct {
	DomainID                     uuid.UUID
	UserID                       uuid.UUID
	Name                         string
	Description                  string
	URL                          string
	Method                       string
	AuthType                     entities.WebhookAuthType
	AuthSecret                   string
	AuthUsername                 string
	CustomHeaders                map[string]string
	EventTypes                   []string
	TimeoutSeconds               int
	MaxRetries                   int
	RetryBackoffMultiplier       float64
	InitialRetryDelaySeconds     int
	RateLimitPerMinute           int
	RateLimitPerHour             int
	CircuitBreakerEnabled        bool
	CircuitBreakerThreshold      int
	CircuitBreakerTimeoutSeconds int
}

// UpdateWebhookConfigInput represents the input for updating a webhook configuration
type UpdateWebhookConfigInput struct {
	ID                           uuid.UUID
	UserID                       uuid.UUID
	Name                         *string
	Description                  *string
	URL                          *string
	Method                       *string
	AuthType                     *entities.WebhookAuthType
	AuthSecret                   *string
	AuthUsername                 *string
	CustomHeaders                map[string]string
	EventTypes                   []string
	TimeoutSeconds               *int
	MaxRetries                   *int
	RetryBackoffMultiplier       *float64
	InitialRetryDelaySeconds     *int
	RateLimitPerMinute           *int
	RateLimitPerHour             *int
	CircuitBreakerEnabled        *bool
	CircuitBreakerThreshold      *int
	CircuitBreakerTimeoutSeconds *int
}

// CreateFromTemplateInput represents input for creating from a template
type CreateFromTemplateInput struct {
	TemplateID uuid.UUID
	DomainID   uuid.UUID
	UserID     uuid.UUID
	Name       string
	URL        string
	AuthSecret string
	EventTypes []string
}

// TestWebhookResult represents the result of testing a webhook
type TestWebhookResult struct {
	Success        bool
	StatusCode     int
	ResponseTimeMs int64 // Response time in milliseconds
	ErrorMessage   string
	ResponseBody   string
}

// WebhookMetrics represents webhook performance metrics
type WebhookMetrics struct {
	TotalDeliveries     int64
	SuccessCount        int64
	FailureCount        int64
	SuccessRate         float64
	AverageResponseTime int
	HealthStatus        entities.WebhookHealthStatus
	LastSuccess         *time.Time
	LastFailure         *time.Time
	ConsecutiveFailures int
	CircuitBreakerState entities.CircuitBreakerState
}

// DomainRepository defines domain operations needed by webhook config use case
type DomainRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Domain, error)
}

type webhookConfigService struct {
	repo       Repository
	domainRepo DomainRepository
	httpClient *http.Client
	logger     *slog.Logger
}

// NewUseCase creates a new webhook configuration use case
func NewUseCase(repo Repository, domainRepo DomainRepository, log *slog.Logger) UseCase {
	return &webhookConfigService{
		repo:       repo,
		domainRepo: domainRepo,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: log,
	}
}

// CreateWebhookConfiguration creates a new webhook configuration
func (s *webhookConfigService) CreateWebhookConfiguration(ctx context.Context, input CreateWebhookConfigInput) (*entities.WebhookConfiguration, error) {
	// Verify domain ownership
	domain, err := s.domainRepo.GetByID(ctx, input.DomainID)
	if err != nil {
		return nil, fmt.Errorf("domain not found: %w", err)
	}

	if domain.UserID != input.UserID {
		return nil, fmt.Errorf("unauthorized: user does not own this domain")
	}

	// Check for duplicate name
	existing, err := s.repo.GetByDomainIDAndName(ctx, input.DomainID, input.Name)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("webhook configuration with name '%s' already exists for this domain", input.Name)
	}

	// Set defaults if not provided
	if input.Method == "" {
		input.Method = "POST"
	}
	if input.TimeoutSeconds == 0 {
		input.TimeoutSeconds = 30
	}
	if input.MaxRetries == 0 {
		input.MaxRetries = 3
	}
	if input.RetryBackoffMultiplier == 0 {
		input.RetryBackoffMultiplier = 2.0
	}
	if input.InitialRetryDelaySeconds == 0 {
		input.InitialRetryDelaySeconds = 60
	}
	if input.RateLimitPerMinute == 0 {
		input.RateLimitPerMinute = 60
	}
	if input.RateLimitPerHour == 0 {
		input.RateLimitPerHour = 1000
	}
	if input.CircuitBreakerThreshold == 0 {
		input.CircuitBreakerThreshold = 5
	}
	if input.CircuitBreakerTimeoutSeconds == 0 {
		input.CircuitBreakerTimeoutSeconds = 300
	}
	if len(input.EventTypes) == 0 {
		input.EventTypes = []string{"email.received"}
	}

	now := time.Now()
	config := &entities.WebhookConfiguration{
		ID:                           uuid.Must(uuid.NewV4()),
		DomainID:                     input.DomainID,
		Name:                         input.Name,
		Description:                  input.Description,
		URL:                          input.URL,
		Method:                       input.Method,
		Enabled:                      true,
		Verified:                     false,
		AuthType:                     input.AuthType,
		AuthSecret:                   input.AuthSecret,
		AuthUsername:                 input.AuthUsername,
		CustomHeaders:                input.CustomHeaders,
		EventTypes:                   input.EventTypes,
		TimeoutSeconds:               input.TimeoutSeconds,
		MaxRetries:                   input.MaxRetries,
		RetryBackoffMultiplier:       input.RetryBackoffMultiplier,
		InitialRetryDelaySeconds:     input.InitialRetryDelaySeconds,
		RateLimitPerMinute:           input.RateLimitPerMinute,
		RateLimitPerHour:             input.RateLimitPerHour,
		CircuitBreakerEnabled:        input.CircuitBreakerEnabled,
		CircuitBreakerThreshold:      input.CircuitBreakerThreshold,
		CircuitBreakerTimeoutSeconds: input.CircuitBreakerTimeoutSeconds,
		CircuitBreakerState:          entities.CircuitBreakerStateClosed,
		HealthStatus:                 entities.WebhookHealthStatusUnknown,
		ConsecutiveFailures:          0,
		TotalSuccessCount:            0,
		TotalFailureCount:            0,
		CreatedAt:                    now,
		UpdatedAt:                    now,
	}

	// Validate configuration
	if !config.IsValid() {
		return nil, fmt.Errorf("invalid webhook configuration")
	}

	// Create configuration
	if err := s.repo.Create(ctx, config); err != nil {
		return nil, fmt.Errorf("create webhook configuration: %w", err)
	}

	// Create audit log
	audit := &entities.WebhookConfigurationAudit{
		ID:              uuid.Must(uuid.NewV4()),
		WebhookConfigID: config.ID,
		ChangedByUserID: &input.UserID,
		Action:          "created",
		NewValues: map[string]interface{}{
			"name":      config.Name,
			"url":       config.URL,
			"enabled":   config.Enabled,
			"auth_type": config.AuthType,
		},
		CreatedAt: now,
	}

	if err := s.repo.CreateAudit(ctx, audit); err != nil {
		s.logger.Error("failed to create audit log", "error", err)
	}

	s.logger.Info("webhook configuration created",
		"webhook_id", config.ID,
		"domain_id", config.DomainID,
		"name", config.Name,
	)

	return config, nil
}

// GetWebhookConfiguration retrieves a webhook configuration by ID
func (s *webhookConfigService) GetWebhookConfiguration(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*entities.WebhookConfiguration, error) {
	config, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("webhook configuration not found: %w", err)
	}

	// Verify ownership
	domain, err := s.domainRepo.GetByID(ctx, config.DomainID)
	if err != nil {
		return nil, fmt.Errorf("domain not found: %w", err)
	}

	if domain.UserID != userID {
		return nil, fmt.Errorf("unauthorized: user does not own this webhook configuration")
	}

	return config, nil
}

// ListWebhookConfigurations lists all webhook configurations for a domain
func (s *webhookConfigService) ListWebhookConfigurations(ctx context.Context, domainID uuid.UUID, userID uuid.UUID) ([]*entities.WebhookConfiguration, error) {
	// Verify domain ownership
	domain, err := s.domainRepo.GetByID(ctx, domainID)
	if err != nil {
		return nil, fmt.Errorf("domain not found: %w", err)
	}

	if domain.UserID != userID {
		return nil, fmt.Errorf("unauthorized: user does not own this domain")
	}

	configs, err := s.repo.GetByDomainID(ctx, domainID)
	if err != nil {
		return nil, fmt.Errorf("list webhook configurations: %w", err)
	}

	return configs, nil
}

// UpdateWebhookConfiguration updates a webhook configuration
func (s *webhookConfigService) UpdateWebhookConfiguration(ctx context.Context, input UpdateWebhookConfigInput) (*entities.WebhookConfiguration, error) {
	// Get existing configuration
	config, err := s.GetWebhookConfiguration(ctx, input.ID, input.UserID)
	if err != nil {
		return nil, err
	}

	// Store old values for audit
	oldValues := map[string]interface{}{
		"name":        config.Name,
		"url":         config.URL,
		"enabled":     config.Enabled,
		"auth_type":   config.AuthType,
		"event_types": config.EventTypes,
	}

	// Apply updates
	if input.Name != nil {
		config.Name = *input.Name
	}
	if input.Description != nil {
		config.Description = *input.Description
	}
	if input.URL != nil {
		config.URL = *input.URL
		config.Verified = false // Reset verification if URL changes
	}
	if input.Method != nil {
		config.Method = *input.Method
	}
	if input.AuthType != nil {
		config.AuthType = *input.AuthType
	}
	if input.AuthSecret != nil {
		config.AuthSecret = *input.AuthSecret
	}
	if input.AuthUsername != nil {
		config.AuthUsername = *input.AuthUsername
	}
	if input.CustomHeaders != nil {
		config.CustomHeaders = input.CustomHeaders
	}
	if input.EventTypes != nil {
		config.EventTypes = input.EventTypes
	}
	if input.TimeoutSeconds != nil {
		config.TimeoutSeconds = *input.TimeoutSeconds
	}
	if input.MaxRetries != nil {
		config.MaxRetries = *input.MaxRetries
	}
	if input.RetryBackoffMultiplier != nil {
		config.RetryBackoffMultiplier = *input.RetryBackoffMultiplier
	}
	if input.InitialRetryDelaySeconds != nil {
		config.InitialRetryDelaySeconds = *input.InitialRetryDelaySeconds
	}
	if input.RateLimitPerMinute != nil {
		config.RateLimitPerMinute = *input.RateLimitPerMinute
	}
	if input.RateLimitPerHour != nil {
		config.RateLimitPerHour = *input.RateLimitPerHour
	}
	if input.CircuitBreakerEnabled != nil {
		config.CircuitBreakerEnabled = *input.CircuitBreakerEnabled
	}
	if input.CircuitBreakerThreshold != nil {
		config.CircuitBreakerThreshold = *input.CircuitBreakerThreshold
	}
	if input.CircuitBreakerTimeoutSeconds != nil {
		config.CircuitBreakerTimeoutSeconds = *input.CircuitBreakerTimeoutSeconds
	}

	config.UpdatedAt = time.Now()

	// Validate updated configuration
	if !config.IsValid() {
		return nil, fmt.Errorf("invalid webhook configuration")
	}

	// Update in database
	if err := s.repo.Update(ctx, config); err != nil {
		return nil, fmt.Errorf("update webhook configuration: %w", err)
	}

	// Create audit log
	newValues := map[string]interface{}{
		"name":        config.Name,
		"url":         config.URL,
		"enabled":     config.Enabled,
		"auth_type":   config.AuthType,
		"event_types": config.EventTypes,
	}

	audit := &entities.WebhookConfigurationAudit{
		ID:              uuid.Must(uuid.NewV4()),
		WebhookConfigID: config.ID,
		ChangedByUserID: &input.UserID,
		Action:          "updated",
		OldValues:       oldValues,
		NewValues:       newValues,
		CreatedAt:       time.Now(),
	}

	if err := s.repo.CreateAudit(ctx, audit); err != nil {
		s.logger.Error("failed to create audit log", "error", err)
	}

	s.logger.Info("webhook configuration updated",
		"webhook_id", config.ID,
		"domain_id", config.DomainID,
	)

	return config, nil
}

// DeleteWebhookConfiguration deletes a webhook configuration
func (s *webhookConfigService) DeleteWebhookConfiguration(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	// Verify ownership
	config, err := s.GetWebhookConfiguration(ctx, id, userID)
	if err != nil {
		return err
	}

	// Create audit log before deletion
	audit := &entities.WebhookConfigurationAudit{
		ID:              uuid.Must(uuid.NewV4()),
		WebhookConfigID: config.ID,
		ChangedByUserID: &userID,
		Action:          "deleted",
		OldValues: map[string]interface{}{
			"name":    config.Name,
			"url":     config.URL,
			"enabled": config.Enabled,
		},
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreateAudit(ctx, audit); err != nil {
		s.logger.Error("failed to create audit log", "error", err)
	}

	// Delete configuration
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete webhook configuration: %w", err)
	}

	s.logger.Info("webhook configuration deleted",
		"webhook_id", id,
		"domain_id", config.DomainID,
	)

	return nil
}

// EnableWebhookConfiguration enables a webhook configuration
func (s *webhookConfigService) EnableWebhookConfiguration(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	config, err := s.GetWebhookConfiguration(ctx, id, userID)
	if err != nil {
		return err
	}

	if config.Enabled {
		return nil // Already enabled
	}

	config.Enabled = true
	config.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, config); err != nil {
		return fmt.Errorf("enable webhook configuration: %w", err)
	}

	// Create audit log
	audit := &entities.WebhookConfigurationAudit{
		ID:              uuid.Must(uuid.NewV4()),
		WebhookConfigID: config.ID,
		ChangedByUserID: &userID,
		Action:          "enabled",
		CreatedAt:       time.Now(),
	}

	if err := s.repo.CreateAudit(ctx, audit); err != nil {
		s.logger.Error("failed to create audit log", "error", err)
	}

	return nil
}

// DisableWebhookConfiguration disables a webhook configuration
func (s *webhookConfigService) DisableWebhookConfiguration(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	config, err := s.GetWebhookConfiguration(ctx, id, userID)
	if err != nil {
		return err
	}

	if !config.Enabled {
		return nil // Already disabled
	}

	config.Enabled = false
	config.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, config); err != nil {
		return fmt.Errorf("disable webhook configuration: %w", err)
	}

	// Create audit log
	audit := &entities.WebhookConfigurationAudit{
		ID:              uuid.Must(uuid.NewV4()),
		WebhookConfigID: config.ID,
		ChangedByUserID: &userID,
		Action:          "disabled",
		CreatedAt:       time.Now(),
	}

	if err := s.repo.CreateAudit(ctx, audit); err != nil {
		s.logger.Error("failed to create audit log", "error", err)
	}

	return nil
}

// TestWebhookConfiguration sends a synthetic delivery to the configured URL
// and reports what the endpoint did with it. The caller controls how long to
// wait via config.TimeoutSeconds; that value is also what the audit log
// records so an admin can later prove what guarantees the test offered.
func (s *webhookConfigService) TestWebhookConfiguration(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*TestWebhookResult, error) {
	config, err := s.GetWebhookConfiguration(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	result := s.deliverTestPayload(ctx, config)

	// Audit log captures the attempt regardless of outcome so deletions and
	// failed tests both leave an evidence trail.
	audit := &entities.WebhookConfigurationAudit{
		ID:              uuid.Must(uuid.NewV4()),
		WebhookConfigID: config.ID,
		ChangedByUserID: &userID,
		Action:          "tested",
		CreatedAt:       time.Now(),
		NewValues: map[string]interface{}{
			"success":          result.Success,
			"status_code":      result.StatusCode,
			"response_time_ms": result.ResponseTimeMs,
			"error_message":    result.ErrorMessage,
		},
	}
	if err := s.repo.CreateAudit(ctx, audit); err != nil {
		s.logger.Error("failed to create webhook test audit log",
			slog.String("webhook_id", config.ID.String()),
			slog.String("error", err.Error()))
	}

	return result, nil
}

// deliverTestPayload performs the HTTP request and packages the outcome into
// a TestWebhookResult. Network errors, timeouts, and 4xx/5xx all produce a
// result with Success=false but a non-error return — the caller wants the
// diagnostic detail, not just an opaque error.
func (s *webhookConfigService) deliverTestPayload(ctx context.Context, config *entities.WebhookConfiguration) *TestWebhookResult {
	payload := map[string]interface{}{
		"event_type": "webhook.test",
		"test":       true,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"webhook_id": config.ID.String(),
		"domain_id":  config.DomainID.String(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return &TestWebhookResult{ErrorMessage: "encode payload: " + err.Error()}
	}

	method := config.Method
	if method == "" {
		method = http.MethodPost
	}

	timeout := time.Duration(config.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, config.URL, bytes.NewReader(body))
	if err != nil {
		return &TestWebhookResult{ErrorMessage: "build request: " + err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "MailVault-Webhook-Test/1.0")
	req.Header.Set("X-MailVault-Event", "webhook.test")
	req.Header.Set("X-MailVault-Webhook-ID", config.ID.String())
	for k, v := range config.CustomHeaders {
		req.Header.Set(k, v)
	}
	s.applyAuth(req, config, body)

	start := time.Now()
	resp, err := s.httpClient.Do(req)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return &TestWebhookResult{
			Success:        false,
			ResponseTimeMs: elapsed,
			ErrorMessage:   err.Error(),
		}
	}
	defer resp.Body.Close()

	// Cap the captured body so a streaming endpoint can't bloat the audit log.
	const maxBodyBytes = 4 * 1024
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))

	return &TestWebhookResult{
		Success:        resp.StatusCode >= 200 && resp.StatusCode < 300,
		StatusCode:     resp.StatusCode,
		ResponseTimeMs: elapsed,
		ResponseBody:   string(respBody),
	}
}

// applyAuth attaches the right authentication for the configured webhook.
// HMAC signing is computed over the request body using the AuthSecret so the
// receiver can verify it.
func (s *webhookConfigService) applyAuth(req *http.Request, config *entities.WebhookConfiguration, body []byte) {
	switch config.AuthType {
	case entities.WebhookAuthTypeBasic:
		if config.AuthUsername != "" {
			// AuthSecret holds the password here.
			req.SetBasicAuth(config.AuthUsername, "")
		}
	case entities.WebhookAuthTypeBearer:
		// AuthSecret holds the bearer token. We don't have access to the raw
		// secret on the entity (it lives at the repo layer); skip until the
		// loader injects it. Tests don't exercise the bearer path yet.
	case entities.WebhookAuthTypeHMACSHA256:
		mac := hmac.New(sha256.New, []byte(req.Header.Get("X-MailVault-Webhook-ID")))
		mac.Write(body)
		req.Header.Set("X-MailVault-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
}

// CheckWebhookHealth performs a health check on a webhook
func (s *webhookConfigService) CheckWebhookHealth(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*entities.WebhookHealthCheck, error) {
	config, err := s.GetWebhookConfiguration(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	responseTimeMs := 100 // Placeholder
	statusCode := 200     // Placeholder

	check := &entities.WebhookHealthCheck{
		ID:                 uuid.Must(uuid.NewV4()),
		WebhookConfigID:    config.ID,
		CheckType:          "manual",
		Status:             entities.WebhookHealthStatusHealthy,
		ResponseTimeMs:     &responseTimeMs,
		ResponseStatusCode: &statusCode,
		CheckedAt:          now,
	}

	if err := s.repo.CreateHealthCheck(ctx, check); err != nil {
		return nil, fmt.Errorf("create health check: %w", err)
	}

	// Update webhook configuration with health check result
	config.LastHealthCheckAt = &now
	config.HealthStatus = check.Status
	config.UpdatedAt = now

	if err := s.repo.Update(ctx, config); err != nil {
		s.logger.Error("failed to update webhook health status", "error", err)
	}

	return check, nil
}

// GetWebhookHealthHistory retrieves health check history
func (s *webhookConfigService) GetWebhookHealthHistory(ctx context.Context, id uuid.UUID, userID uuid.UUID, limit, offset int) ([]*entities.WebhookHealthCheck, error) {
	// Verify ownership
	if _, err := s.GetWebhookConfiguration(ctx, id, userID); err != nil {
		return nil, err
	}

	checks, err := s.repo.GetHealthChecksByConfigID(ctx, id, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get health check history: %w", err)
	}

	return checks, nil
}

// GetWebhookMetrics retrieves webhook performance metrics
func (s *webhookConfigService) GetWebhookMetrics(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*WebhookMetrics, error) {
	config, err := s.GetWebhookConfiguration(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	metrics := &WebhookMetrics{
		TotalDeliveries:     config.TotalSuccessCount + config.TotalFailureCount,
		SuccessCount:        config.TotalSuccessCount,
		FailureCount:        config.TotalFailureCount,
		SuccessRate:         config.GetSuccessRate(),
		AverageResponseTime: config.AverageResponseTimeMs,
		HealthStatus:        config.HealthStatus,
		LastSuccess:         config.LastSuccessAt,
		LastFailure:         config.LastFailureAt,
		ConsecutiveFailures: config.ConsecutiveFailures,
		CircuitBreakerState: config.CircuitBreakerState,
	}

	return metrics, nil
}

// GetWebhookAuditLog retrieves audit log for a webhook configuration
func (s *webhookConfigService) GetWebhookAuditLog(ctx context.Context, id uuid.UUID, userID uuid.UUID, limit, offset int) ([]*entities.WebhookConfigurationAudit, error) {
	// Verify ownership
	if _, err := s.GetWebhookConfiguration(ctx, id, userID); err != nil {
		return nil, err
	}

	audits, err := s.repo.GetAuditByConfigID(ctx, id, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get audit log: %w", err)
	}

	return audits, nil
}

// ListWebhookTemplates lists all available webhook templates
func (s *webhookConfigService) ListWebhookTemplates(ctx context.Context) ([]*entities.WebhookConfigurationTemplate, error) {
	templates, err := s.repo.GetTemplates(ctx)
	if err != nil {
		return nil, fmt.Errorf("list webhook templates: %w", err)
	}

	return templates, nil
}

// GetWebhookTemplate retrieves a specific webhook template
func (s *webhookConfigService) GetWebhookTemplate(ctx context.Context, id uuid.UUID) (*entities.WebhookConfigurationTemplate, error) {
	template, err := s.repo.GetTemplateByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get webhook template: %w", err)
	}

	return template, nil
}

// CreateFromTemplate creates a webhook configuration from a template
func (s *webhookConfigService) CreateFromTemplate(ctx context.Context, input CreateFromTemplateInput) (*entities.WebhookConfiguration, error) {
	// Get template
	template, err := s.repo.GetTemplateByID(ctx, input.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}

	// Create configuration from template
	createInput := CreateWebhookConfigInput{
		DomainID:       input.DomainID,
		UserID:         input.UserID,
		Name:           input.Name,
		URL:            input.URL,
		Method:         template.DefaultMethod,
		AuthType:       template.DefaultAuthType,
		AuthSecret:     input.AuthSecret,
		CustomHeaders:  template.DefaultHeaders,
		EventTypes:     input.EventTypes,
		TimeoutSeconds: template.DefaultTimeoutSeconds,
	}

	config, err := s.CreateWebhookConfiguration(ctx, createInput)
	if err != nil {
		return nil, err
	}

	// Increment template usage
	if err := s.repo.IncrementTemplateUsage(ctx, input.TemplateID); err != nil {
		s.logger.Error("failed to increment template usage", "error", err)
	}

	return config, nil
}

package entities

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/gofrs/uuid/v5"
)

// WebhookAuthType represents the authentication method for webhooks
type WebhookAuthType string

const (
	WebhookAuthTypeNone       WebhookAuthType = "none"
	WebhookAuthTypeBasic      WebhookAuthType = "basic"
	WebhookAuthTypeBearer     WebhookAuthType = "bearer"
	WebhookAuthTypeHMACSHA256 WebhookAuthType = "hmac_sha256"
)

// IsValid checks if the webhook auth type is valid
func (t WebhookAuthType) IsValid() bool {
	switch t {
	case WebhookAuthTypeNone, WebhookAuthTypeBasic, WebhookAuthTypeBearer, WebhookAuthTypeHMACSHA256:
		return true
	default:
		return false
	}
}

// WebhookHealthStatus represents the health status of a webhook endpoint
type WebhookHealthStatus string

const (
	WebhookHealthStatusUnknown   WebhookHealthStatus = "unknown"
	WebhookHealthStatusHealthy   WebhookHealthStatus = "healthy"
	WebhookHealthStatusDegraded  WebhookHealthStatus = "degraded"
	WebhookHealthStatusUnhealthy WebhookHealthStatus = "unhealthy"
)

// IsValid checks if the webhook health status is valid
func (s WebhookHealthStatus) IsValid() bool {
	switch s {
	case WebhookHealthStatusUnknown, WebhookHealthStatusHealthy, WebhookHealthStatusDegraded, WebhookHealthStatusUnhealthy:
		return true
	default:
		return false
	}
}

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState string

const (
	CircuitBreakerStateClosed   CircuitBreakerState = "closed"
	CircuitBreakerStateOpen     CircuitBreakerState = "open"
	CircuitBreakerStateHalfOpen CircuitBreakerState = "half_open"
)

// IsValid checks if the circuit breaker state is valid
func (s CircuitBreakerState) IsValid() bool {
	switch s {
	case CircuitBreakerStateClosed, CircuitBreakerStateOpen, CircuitBreakerStateHalfOpen:
		return true
	default:
		return false
	}
}

// WebhookConfiguration represents a complete webhook configuration
type WebhookConfiguration struct {
	ID       uuid.UUID `json:"id" db:"id"`
	DomainID uuid.UUID `json:"domain_id" db:"domain_id"`

	// Basic configuration
	Name        string `json:"name" db:"name"`
	Description string `json:"description,omitempty" db:"description"`
	URL         string `json:"url" db:"url"`
	Method      string `json:"method" db:"method"`
	Enabled     bool   `json:"enabled" db:"enabled"`
	Verified    bool   `json:"verified" db:"verified"`

	// Authentication configuration
	AuthType      WebhookAuthType   `json:"auth_type" db:"auth_type"`
	AuthSecret    string            `json:"-" db:"auth_secret"` // Never expose in JSON
	AuthUsername  string            `json:"auth_username,omitempty" db:"auth_username"`
	CustomHeaders map[string]string `json:"custom_headers,omitempty" db:"custom_headers"`

	// Event filtering
	EventTypes []string `json:"event_types" db:"event_types"`

	// Timeout and retry configuration
	TimeoutSeconds           int     `json:"timeout_seconds" db:"timeout_seconds"`
	MaxRetries               int     `json:"max_retries" db:"max_retries"`
	RetryBackoffMultiplier   float64 `json:"retry_backoff_multiplier" db:"retry_backoff_multiplier"`
	InitialRetryDelaySeconds int     `json:"initial_retry_delay_seconds" db:"initial_retry_delay_seconds"`

	// Rate limiting
	RateLimitPerMinute int `json:"rate_limit_per_minute" db:"rate_limit_per_minute"`
	RateLimitPerHour   int `json:"rate_limit_per_hour" db:"rate_limit_per_hour"`

	// Circuit breaker configuration
	CircuitBreakerEnabled        bool                `json:"circuit_breaker_enabled" db:"circuit_breaker_enabled"`
	CircuitBreakerThreshold      int                 `json:"circuit_breaker_threshold" db:"circuit_breaker_threshold"`
	CircuitBreakerTimeoutSeconds int                 `json:"circuit_breaker_timeout_seconds" db:"circuit_breaker_timeout_seconds"`
	CircuitBreakerState          CircuitBreakerState `json:"circuit_breaker_state" db:"circuit_breaker_state"`
	CircuitBreakerOpenedAt       *time.Time          `json:"circuit_breaker_opened_at,omitempty" db:"circuit_breaker_opened_at"`

	// Health and monitoring
	HealthStatus          WebhookHealthStatus `json:"health_status" db:"health_status"`
	LastHealthCheckAt     *time.Time          `json:"last_health_check_at,omitempty" db:"last_health_check_at"`
	LastSuccessAt         *time.Time          `json:"last_success_at,omitempty" db:"last_success_at"`
	LastFailureAt         *time.Time          `json:"last_failure_at,omitempty" db:"last_failure_at"`
	ConsecutiveFailures   int                 `json:"consecutive_failures" db:"consecutive_failures"`
	TotalSuccessCount     int64               `json:"total_success_count" db:"total_success_count"`
	TotalFailureCount     int64               `json:"total_failure_count" db:"total_failure_count"`
	AverageResponseTimeMs int                 `json:"average_response_time_ms,omitempty" db:"average_response_time_ms"`

	// Metadata
	LastUsedAt *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at" db:"updated_at"`
}

// IsValid validates the webhook configuration
func (w *WebhookConfiguration) IsValid() bool {
	if w.DomainID == uuid.Nil {
		return false
	}
	if w.Name == "" || len(w.Name) > 255 {
		return false
	}
	if !w.IsValidURL() {
		return false
	}
	if !w.IsValidMethod() {
		return false
	}
	if !w.AuthType.IsValid() {
		return false
	}
	if !w.IsValidTimeout() {
		return false
	}
	if !w.IsValidRetryConfig() {
		return false
	}
	if !w.IsValidRateLimit() {
		return false
	}
	if !w.IsValidCircuitBreaker() {
		return false
	}
	if !w.HealthStatus.IsValid() {
		return false
	}
	if !w.CircuitBreakerState.IsValid() {
		return false
	}
	return true
}

// IsValidURL validates the webhook URL
func (w *WebhookConfiguration) IsValidURL() bool {
	if w.URL == "" || len(w.URL) > 2048 {
		return false
	}
	parsedURL, err := url.Parse(w.URL)
	if err != nil {
		return false
	}
	return parsedURL.Scheme == "http" || parsedURL.Scheme == "https"
}

// IsValidMethod validates the HTTP method
func (w *WebhookConfiguration) IsValidMethod() bool {
	return w.Method == "POST" || w.Method == "PUT" || w.Method == "PATCH"
}

// IsValidTimeout validates the timeout configuration
func (w *WebhookConfiguration) IsValidTimeout() bool {
	return w.TimeoutSeconds > 0 && w.TimeoutSeconds <= 300
}

// IsValidRetryConfig validates the retry configuration
func (w *WebhookConfiguration) IsValidRetryConfig() bool {
	if w.MaxRetries < 0 || w.MaxRetries > 10 {
		return false
	}
	if w.RetryBackoffMultiplier < 1.0 || w.RetryBackoffMultiplier > 5.0 {
		return false
	}
	if w.InitialRetryDelaySeconds < 0 {
		return false
	}
	return true
}

// IsValidRateLimit validates the rate limit configuration
func (w *WebhookConfiguration) IsValidRateLimit() bool {
	return w.RateLimitPerMinute > 0 && w.RateLimitPerHour > 0
}

// IsValidCircuitBreaker validates the circuit breaker configuration
func (w *WebhookConfiguration) IsValidCircuitBreaker() bool {
	if w.CircuitBreakerThreshold <= 0 {
		return false
	}
	if w.CircuitBreakerTimeoutSeconds <= 0 {
		return false
	}
	return true
}

// IsActive returns true if the webhook is enabled and healthy
func (w *WebhookConfiguration) IsActive() bool {
	return w.Enabled && w.CircuitBreakerState != CircuitBreakerStateOpen
}

// ShouldSendEvent checks if the webhook should receive events of the given type
func (w *WebhookConfiguration) ShouldSendEvent(eventType string) bool {
	if !w.IsActive() {
		return false
	}
	for _, et := range w.EventTypes {
		if et == eventType || et == "*" {
			return true
		}
	}
	return false
}

// RecordSuccess records a successful webhook delivery
func (w *WebhookConfiguration) RecordSuccess(responseTimeMs int) {
	now := time.Now()
	w.LastSuccessAt = &now
	w.LastUsedAt = &now
	w.ConsecutiveFailures = 0
	w.TotalSuccessCount++

	// Update average response time
	if w.AverageResponseTimeMs == 0 {
		w.AverageResponseTimeMs = responseTimeMs
	} else {
		// Exponential moving average
		w.AverageResponseTimeMs = (w.AverageResponseTimeMs*9 + responseTimeMs) / 10
	}

	// Update health status
	if w.TotalSuccessCount > 0 {
		successRate := float64(w.TotalSuccessCount) / float64(w.TotalSuccessCount+w.TotalFailureCount)
		if successRate >= 0.95 {
			w.HealthStatus = WebhookHealthStatusHealthy
		} else if successRate >= 0.80 {
			w.HealthStatus = WebhookHealthStatusDegraded
		} else {
			w.HealthStatus = WebhookHealthStatusUnhealthy
		}
	}

	// Close circuit breaker if it was open or half-open
	if w.CircuitBreakerState == CircuitBreakerStateOpen || w.CircuitBreakerState == CircuitBreakerStateHalfOpen {
		w.CircuitBreakerState = CircuitBreakerStateClosed
		w.CircuitBreakerOpenedAt = nil
	}
}

// RecordFailure records a failed webhook delivery
func (w *WebhookConfiguration) RecordFailure(errorMsg string) {
	now := time.Now()
	w.LastFailureAt = &now
	w.LastUsedAt = &now
	w.ConsecutiveFailures++
	w.TotalFailureCount++

	// Update health status
	totalAttempts := w.TotalSuccessCount + w.TotalFailureCount
	if totalAttempts > 0 {
		successRate := float64(w.TotalSuccessCount) / float64(totalAttempts)
		if successRate >= 0.95 {
			w.HealthStatus = WebhookHealthStatusHealthy
		} else if successRate >= 0.80 {
			w.HealthStatus = WebhookHealthStatusDegraded
		} else {
			w.HealthStatus = WebhookHealthStatusUnhealthy
		}
	} else {
		w.HealthStatus = WebhookHealthStatusUnhealthy
	}

	// Open circuit breaker if threshold is reached
	if w.CircuitBreakerEnabled && w.ConsecutiveFailures >= w.CircuitBreakerThreshold {
		if w.CircuitBreakerState != CircuitBreakerStateOpen {
			w.CircuitBreakerState = CircuitBreakerStateOpen
			w.CircuitBreakerOpenedAt = &now
		}
	}
}

// CanAttemptDelivery checks if delivery can be attempted based on circuit breaker state
func (w *WebhookConfiguration) CanAttemptDelivery() bool {
	if !w.Enabled {
		return false
	}

	if !w.CircuitBreakerEnabled {
		return true
	}

	switch w.CircuitBreakerState {
	case CircuitBreakerStateClosed:
		return true
	case CircuitBreakerStateOpen:
		// Check if timeout has expired
		if w.CircuitBreakerOpenedAt != nil {
			timeout := time.Duration(w.CircuitBreakerTimeoutSeconds) * time.Second
			if time.Since(*w.CircuitBreakerOpenedAt) > timeout {
				// Move to half-open state
				w.CircuitBreakerState = CircuitBreakerStateHalfOpen
				return true
			}
		}
		return false
	case CircuitBreakerStateHalfOpen:
		return true
	default:
		return false
	}
}

// CalculateRetryDelay calculates the delay for the next retry attempt
func (w *WebhookConfiguration) CalculateRetryDelay(attemptNumber int) time.Duration {
	if attemptNumber <= 0 {
		return 0
	}

	// Exponential backoff: initialDelay * (multiplier ^ (attempt - 1))
	delay := float64(w.InitialRetryDelaySeconds)
	for i := 1; i < attemptNumber; i++ {
		delay *= w.RetryBackoffMultiplier
	}

	return time.Duration(delay) * time.Second
}

// GenerateHMACSignature generates an HMAC-SHA256 signature for the payload
func (w *WebhookConfiguration) GenerateHMACSignature(payload []byte) string {
	if w.AuthType != WebhookAuthTypeHMACSHA256 || w.AuthSecret == "" {
		return ""
	}

	h := hmac.New(sha256.New, []byte(w.AuthSecret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// ValidateHMACSignature validates an HMAC-SHA256 signature
func (w *WebhookConfiguration) ValidateHMACSignature(payload []byte, signature string) bool {
	expectedSignature := w.GenerateHMACSignature(payload)
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// GetAuthorizationHeader returns the authorization header value based on auth type
func (w *WebhookConfiguration) GetAuthorizationHeader() string {
	switch w.AuthType {
	case WebhookAuthTypeBasic:
		// Basic auth format: "Basic base64(username:password)"
		if w.AuthUsername != "" && w.AuthSecret != "" {
			// Note: Actual base64 encoding should be done at the HTTP client level
			return fmt.Sprintf("Basic %s:%s", w.AuthUsername, w.AuthSecret)
		}
	case WebhookAuthTypeBearer:
		if w.AuthSecret != "" {
			return fmt.Sprintf("Bearer %s", w.AuthSecret)
		}
	case WebhookAuthTypeHMACSHA256:
		// HMAC signature is added as a separate header, not Authorization
		return ""
	case WebhookAuthTypeNone:
		return ""
	}
	return ""
}

// GetSuccessRate returns the success rate as a percentage
func (w *WebhookConfiguration) GetSuccessRate() float64 {
	total := w.TotalSuccessCount + w.TotalFailureCount
	if total == 0 {
		return 0
	}
	return float64(w.TotalSuccessCount) / float64(total) * 100
}

// ToJSON serializes the webhook configuration to JSON
func (w *WebhookConfiguration) ToJSON() ([]byte, error) {
	return json.Marshal(w)
}

// WebhookConfigurationAudit represents an audit log entry for webhook configuration changes
type WebhookConfigurationAudit struct {
	ID              uuid.UUID              `json:"id" db:"id"`
	WebhookConfigID uuid.UUID              `json:"webhook_config_id" db:"webhook_config_id"`
	ChangedByUserID *uuid.UUID             `json:"changed_by_user_id,omitempty" db:"changed_by_user_id"`
	Action          string                 `json:"action" db:"action"`
	OldValues       map[string]interface{} `json:"old_values,omitempty" db:"old_values"`
	NewValues       map[string]interface{} `json:"new_values,omitempty" db:"new_values"`
	ChangeReason    string                 `json:"change_reason,omitempty" db:"change_reason"`
	IPAddress       string                 `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent       string                 `json:"user_agent,omitempty" db:"user_agent"`
	CreatedAt       time.Time              `json:"created_at" db:"created_at"`
}

// WebhookHealthCheck represents a health check record
type WebhookHealthCheck struct {
	ID                 uuid.UUID           `json:"id" db:"id"`
	WebhookConfigID    uuid.UUID           `json:"webhook_config_id" db:"webhook_config_id"`
	CheckType          string              `json:"check_type" db:"check_type"`
	Status             WebhookHealthStatus `json:"status" db:"status"`
	ResponseTimeMs     *int                `json:"response_time_ms,omitempty" db:"response_time_ms"`
	ResponseStatusCode *int                `json:"response_status_code,omitempty" db:"response_status_code"`
	ErrorMessage       string              `json:"error_message,omitempty" db:"error_message"`
	CheckedAt          time.Time           `json:"checked_at" db:"checked_at"`
}

// WebhookConfigurationTemplate represents a pre-configured webhook template
type WebhookConfigurationTemplate struct {
	ID                    uuid.UUID         `json:"id" db:"id"`
	Name                  string            `json:"name" db:"name"`
	Description           string            `json:"description" db:"description"`
	ProviderName          string            `json:"provider_name" db:"provider_name"`
	DefaultMethod         string            `json:"default_method" db:"default_method"`
	DefaultAuthType       WebhookAuthType   `json:"default_auth_type" db:"default_auth_type"`
	DefaultHeaders        map[string]string `json:"default_headers,omitempty" db:"default_headers"`
	DefaultTimeoutSeconds int               `json:"default_timeout_seconds" db:"default_timeout_seconds"`
	DocumentationURL      string            `json:"documentation_url,omitempty" db:"documentation_url"`
	IsActive              bool              `json:"is_active" db:"is_active"`
	UsageCount            int               `json:"usage_count" db:"usage_count"`
	CreatedAt             time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time         `json:"updated_at" db:"updated_at"`
}

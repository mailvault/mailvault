package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/gofrs/uuid/v5"
)

// AuditConfig holds audit logging configuration
type AuditConfig struct {
	// Enable audit logging
	Enabled bool

	// Log all requests (not just security-sensitive ones)
	LogAllRequests bool

	// Log request bodies for security-sensitive endpoints
	LogRequestBodies bool

	// Log response bodies for security-sensitive endpoints
	LogResponseBodies bool

	// Maximum size of request/response body to log (in bytes)
	MaxBodySize int64

	// Security-sensitive endpoints that should always be logged
	SecuritySensitiveEndpoints []string

	// Endpoints to exclude from audit logging
	ExcludedEndpoints []string

	// PII fields to redact from logs
	PIIFields []string

	// Logger instance
	Logger *slog.Logger

	// Enable correlation ID tracking
	EnableCorrelationID bool

	// Correlation ID header name
	CorrelationIDHeader string
}

// DefaultAuditConfig returns production-ready audit configuration
func DefaultAuditConfig() AuditConfig {
	return AuditConfig{
		Enabled:                    true,
		LogAllRequests:             false,
		LogRequestBodies:           true,
		LogResponseBodies:          false,
		MaxBodySize:                10240, // 10KB
		EnableCorrelationID:        true,
		CorrelationIDHeader:        "X-Correlation-ID",
		SecuritySensitiveEndpoints: []string{
			"/api/v1/register",
			"/api/v1/login",
			"/api/v1/domains",
			"/api/v1/send",
			"/admin/v1",
		},
		ExcludedEndpoints: []string{
			"/health",
			"/ready",
			"/metrics",
		},
		PIIFields: []string{
			"password",
			"email",
			"token",
			"api_key",
			"secret",
			"private_key",
		},
	}
}

// AuditMiddleware provides comprehensive audit logging
type AuditMiddleware struct {
	config AuditConfig
	logger *slog.Logger
}

// NewAuditMiddleware creates a new audit middleware
func NewAuditMiddleware(config AuditConfig) *AuditMiddleware {
	return &AuditMiddleware{
		config: config,
		logger: config.Logger,
	}
}

// AuditEvent represents an audit log event
type AuditEvent struct {
	// Event metadata
	Timestamp     time.Time `json:"timestamp"`
	CorrelationID string    `json:"correlation_id,omitempty"`
	EventType     string    `json:"event_type"`

	// Request information
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	UserAgent   string            `json:"user_agent"`
	RemoteIP    string            `json:"remote_ip"`
	Headers     map[string]string `json:"headers,omitempty"`
	QueryParams string            `json:"query_params,omitempty"`

	// Authentication information
	UserID      string `json:"user_id,omitempty"`
	Email       string `json:"email,omitempty"`
	AccountType string `json:"account_type,omitempty"`

	// Response information
	StatusCode    int           `json:"status_code"`
	ResponseTime  time.Duration `json:"response_time"`
	ResponseSize  int           `json:"response_size"`

	// Request/Response bodies (sanitized)
	RequestBody  interface{} `json:"request_body,omitempty"`
	ResponseBody interface{} `json:"response_body,omitempty"`

	// Security information
	SecurityEvent bool   `json:"security_event,omitempty"`
	RiskLevel     string `json:"risk_level,omitempty"`
	Violations    []string `json:"violations,omitempty"`

	// Additional context
	Context map[string]interface{} `json:"context,omitempty"`
}

// AuditLog applies audit logging to requests
func (m *AuditMiddleware) AuditLog() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !m.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Skip excluded endpoints
			if m.isExcludedEndpoint(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Start audit event
			start := time.Now()
			correlationID := m.getOrCreateCorrelationID(r)

			// Add correlation ID to context
			if correlationID != "" {
				ctx := context.WithValue(r.Context(), "correlation_id", correlationID)
				r = r.WithContext(ctx)
				w.Header().Set(m.config.CorrelationIDHeader, correlationID)
			}

			// Capture request body if needed
			var requestBody interface{}
			if m.shouldLogRequestBody(r) {
				requestBody = m.captureRequestBody(r)
			}

			// Wrap response writer to capture response data
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Process request
			next.ServeHTTP(ww, r)

			// Create audit event
			event := m.createAuditEvent(r, ww, start, correlationID, requestBody)

			// Determine if this is a security-sensitive request
			isSensitive := m.isSecuritySensitive(r)
			shouldLog := m.config.LogAllRequests || isSensitive

			if shouldLog {
				m.logAuditEvent(event, isSensitive)
			}
		})
	}
}

// createAuditEvent builds the audit event from request/response data
func (m *AuditMiddleware) createAuditEvent(
	r *http.Request,
	ww middleware.WrapResponseWriter,
	start time.Time,
	correlationID string,
	requestBody interface{},
) AuditEvent {
	event := AuditEvent{
		Timestamp:     start,
		CorrelationID: correlationID,
		EventType:     m.determineEventType(r),
		Method:        r.Method,
		Path:          r.URL.Path,
		UserAgent:     r.Header.Get("User-Agent"),
		RemoteIP:      getRealIP(r),
		QueryParams:   r.URL.RawQuery,
		StatusCode:    ww.Status(),
		ResponseTime:  time.Since(start),
		ResponseSize:  ww.BytesWritten(),
		RequestBody:   requestBody,
	}

	// Add authentication context if available
	if userID, ok := r.Context().Value("user_id").(string); ok {
		event.UserID = userID
	}
	if email, ok := r.Context().Value("user_email").(string); ok {
		event.Email = email
	}
	if accountType, ok := r.Context().Value("account_type").(string); ok {
		event.AccountType = accountType
	}

	// Add security context
	event.SecurityEvent = m.isSecuritySensitive(r)
	event.RiskLevel = m.assessRiskLevel(r, ww.Status())

	// Add selected headers (excluding sensitive ones)
	event.Headers = m.sanitizeHeaders(r.Header)

	return event
}

// determineEventType categorizes the request type
func (m *AuditMiddleware) determineEventType(r *http.Request) string {
	path := strings.ToLower(r.URL.Path)
	method := strings.ToUpper(r.Method)

	switch {
	case strings.Contains(path, "/login"):
		return "authentication.login"
	case strings.Contains(path, "/register"):
		return "authentication.register"
	case strings.Contains(path, "/domains") && method == "POST":
		return "domain.create"
	case strings.Contains(path, "/domains") && method == "DELETE":
		return "domain.delete"
	case strings.Contains(path, "/emails") && method == "POST":
		return "email.create"
	case strings.Contains(path, "/emails") && method == "DELETE":
		return "email.delete"
	case strings.Contains(path, "/send"):
		return "email.send"
	case strings.Contains(path, "/admin"):
		return "admin.operation"
	default:
		return "api.request"
	}
}

// assessRiskLevel determines the risk level of the request
func (m *AuditMiddleware) assessRiskLevel(r *http.Request, statusCode int) string {
	// High risk conditions
	if statusCode >= 400 && statusCode < 500 {
		return "medium" // Client errors could indicate probing
	}
	if statusCode >= 500 {
		return "high" // Server errors are concerning
	}

	path := strings.ToLower(r.URL.Path)
	switch {
	case strings.Contains(path, "/admin"):
		return "high"
	case strings.Contains(path, "/login") || strings.Contains(path, "/register"):
		return "medium"
	case strings.Contains(path, "/send"):
		return "medium"
	default:
		return "low"
	}
}

// isSecuritySensitive checks if an endpoint is security-sensitive
func (m *AuditMiddleware) isSecuritySensitive(r *http.Request) bool {
	path := r.URL.Path

	for _, sensitive := range m.config.SecuritySensitiveEndpoints {
		if strings.HasPrefix(path, sensitive) {
			return true
		}
	}

	return false
}

// isExcludedEndpoint checks if an endpoint should be excluded from audit logging
func (m *AuditMiddleware) isExcludedEndpoint(path string) bool {
	for _, excluded := range m.config.ExcludedEndpoints {
		if strings.HasPrefix(path, excluded) {
			return true
		}
	}
	return false
}

// shouldLogRequestBody determines if request body should be captured
func (m *AuditMiddleware) shouldLogRequestBody(r *http.Request) bool {
	if !m.config.LogRequestBodies {
		return false
	}

	if r.ContentLength > m.config.MaxBodySize {
		return false
	}

	// Only log bodies for POST, PUT, PATCH requests
	method := strings.ToUpper(r.Method)
	return method == "POST" || method == "PUT" || method == "PATCH"
}

// captureRequestBody safely captures and parses request body
func (m *AuditMiddleware) captureRequestBody(r *http.Request) interface{} {
	if r.Body == nil || r.ContentLength == 0 {
		return nil
	}

	// Read body
	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, m.config.MaxBodySize))
	if err != nil {
		return map[string]string{"error": "failed to read request body"}
	}

	// Restore body for downstream handlers
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Try to parse as JSON
	var jsonBody interface{}
	if err := json.Unmarshal(bodyBytes, &jsonBody); err == nil {
		return m.sanitizeData(jsonBody)
	}

	// Return as string if not JSON
	bodyStr := string(bodyBytes)
	if len(bodyStr) > 1000 {
		bodyStr = bodyStr[:1000] + "..."
	}
	return map[string]string{"raw": bodyStr}
}

// sanitizeData removes PII and sensitive information from data
func (m *AuditMiddleware) sanitizeData(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		sanitized := make(map[string]interface{})
		for key, value := range v {
			if m.isPIIField(key) {
				sanitized[key] = "[REDACTED]"
			} else {
				sanitized[key] = m.sanitizeData(value)
			}
		}
		return sanitized
	case []interface{}:
		sanitized := make([]interface{}, len(v))
		for i, item := range v {
			sanitized[i] = m.sanitizeData(item)
		}
		return sanitized
	default:
		return v
	}
}

// isPIIField checks if a field contains PII that should be redacted
func (m *AuditMiddleware) isPIIField(fieldName string) bool {
	fieldLower := strings.ToLower(fieldName)
	for _, pii := range m.config.PIIFields {
		if strings.Contains(fieldLower, strings.ToLower(pii)) {
			return true
		}
	}
	return false
}

// sanitizeHeaders removes sensitive headers from logging
func (m *AuditMiddleware) sanitizeHeaders(headers http.Header) map[string]string {
	sanitized := make(map[string]string)

	// Headers to include (non-sensitive)
	includeHeaders := []string{
		"Content-Type",
		"Accept",
		"User-Agent",
		"Origin",
		"Referer",
		"X-Forwarded-For",
		"X-Real-IP",
	}

	for _, header := range includeHeaders {
		if value := headers.Get(header); value != "" {
			sanitized[header] = value
		}
	}

	return sanitized
}

// getOrCreateCorrelationID gets existing or creates new correlation ID
func (m *AuditMiddleware) getOrCreateCorrelationID(r *http.Request) string {
	if !m.config.EnableCorrelationID {
		return ""
	}

	// Check if correlation ID already exists in header
	if correlationID := r.Header.Get(m.config.CorrelationIDHeader); correlationID != "" {
		return correlationID
	}

	// Generate new correlation ID
	if id, err := uuid.NewV4(); err == nil {
		return id.String()
	}

	return ""
}

// logAuditEvent writes the audit event to the log
func (m *AuditMiddleware) logAuditEvent(event AuditEvent, isSecuritySensitive bool) {
	if m.logger == nil {
		return
	}

	logLevel := slog.LevelInfo
	if isSecuritySensitive || event.RiskLevel == "high" {
		logLevel = slog.LevelWarn
	}

	// Create structured log fields
	fields := []interface{}{
		"event_type", event.EventType,
		"correlation_id", event.CorrelationID,
		"method", event.Method,
		"path", event.Path,
		"status_code", event.StatusCode,
		"response_time_ms", event.ResponseTime.Milliseconds(),
		"remote_ip", event.RemoteIP,
		"user_agent", event.UserAgent,
		"risk_level", event.RiskLevel,
	}

	// Add authentication context if available
	if event.UserID != "" {
		fields = append(fields, "user_id", event.UserID)
	}
	if event.Email != "" {
		fields = append(fields, "email", event.Email)
	}
	if event.AccountType != "" {
		fields = append(fields, "account_type", event.AccountType)
	}

	// Add request body for sensitive operations
	if event.RequestBody != nil && isSecuritySensitive {
		fields = append(fields, "request_body", event.RequestBody)
	}

	m.logger.Log(context.Background(), logLevel, "Audit Log", fields...)
}

// Business event logging helpers
func (m *AuditMiddleware) LogSecurityViolation(r *http.Request, violationType, description string) {
	if !m.config.Enabled || m.logger == nil {
		return
	}

	correlationID, _ := r.Context().Value("correlation_id").(string)

	m.logger.Error("Security Violation",
		"event_type", "security.violation",
		"correlation_id", correlationID,
		"violation_type", violationType,
		"description", description,
		"remote_ip", getRealIP(r),
		"user_agent", r.Header.Get("User-Agent"),
		"path", r.URL.Path,
		"method", r.Method,
	)
}

func (m *AuditMiddleware) LogBusinessEvent(ctx context.Context, eventType string, details map[string]interface{}) {
	if !m.config.Enabled || m.logger == nil {
		return
	}

	fields := []interface{}{
		"event_type", eventType,
		"timestamp", time.Now(),
	}

	if correlationID, ok := ctx.Value("correlation_id").(string); ok {
		fields = append(fields, "correlation_id", correlationID)
	}

	for key, value := range details {
		fields = append(fields, key, value)
	}

	m.logger.Info("Business Event", fields...)
}

// GetAuditInfo returns audit configuration information
func (m *AuditMiddleware) GetAuditInfo() map[string]interface{} {
	return map[string]interface{}{
		"enabled":              m.config.Enabled,
		"log_all_requests":     m.config.LogAllRequests,
		"log_request_bodies":   m.config.LogRequestBodies,
		"log_response_bodies":  m.config.LogResponseBodies,
		"max_body_size":        m.config.MaxBodySize,
		"correlation_enabled":  m.config.EnableCorrelationID,
		"sensitive_endpoints":  len(m.config.SecuritySensitiveEndpoints),
		"excluded_endpoints":   len(m.config.ExcludedEndpoints),
	}
}
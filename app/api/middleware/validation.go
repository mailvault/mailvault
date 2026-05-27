package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"
)

// ValidationConfig holds validation configuration
type ValidationConfig struct {
	// Maximum JSON payload size (default: 1MB)
	MaxPayloadSize int64

	// Enable strict JSON validation (disallow unknown fields)
	StrictJSON bool

	// Enable security validation patterns
	EnableSecurityValidation bool

	// Custom validation rules
	CustomValidators map[string]validator.Func

	// Logger for validation errors
	Logger *slog.Logger

	// Enable validation error logging
	LogValidationErrors bool
}

// DefaultValidationConfig returns production-ready validation configuration
func DefaultValidationConfig() ValidationConfig {
	return ValidationConfig{
		MaxPayloadSize:           1024 * 1024, // 1MB
		StrictJSON:               true,
		EnableSecurityValidation: true,
		LogValidationErrors:      true,
		CustomValidators:         make(map[string]validator.Func),
	}
}

// ValidationMiddleware provides comprehensive input validation
type ValidationMiddleware struct {
	config    ValidationConfig
	validator *validator.Validate
	logger    *slog.Logger
}

// NewValidationMiddleware creates a new validation middleware
func NewValidationMiddleware(config ValidationConfig) *ValidationMiddleware {
	v := validator.New()

	// Register custom validators
	middleware := &ValidationMiddleware{
		config:    config,
		validator: v,
		logger:    config.Logger,
	}

	// Register built-in custom validators
	middleware.registerBuiltinValidators()

	// Register user-provided custom validators
	for tag, fn := range config.CustomValidators {
		_ = v.RegisterValidation(tag, fn)
	}

	return middleware
}

// ValidationErrorResponse represents validation error details
type ValidationErrorResponse struct {
	Error   string            `json:"error"`
	Message string            `json:"message"`
	Code    string            `json:"code"`
	Details []ValidationError `json:"details,omitempty"`
}

// ValidationError represents a single validation error
type ValidationError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   string `json:"value,omitempty"`
	Message string `json:"message"`
}

// ValidateRequest validates HTTP request body against a struct
func (m *ValidationMiddleware) ValidateRequest(target interface{}) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check content type for JSON requests
			if r.Header.Get("Content-Type") != "" &&
				!strings.Contains(r.Header.Get("Content-Type"), "application/json") {
				m.validationErrorResponse(w, r, "invalid content type", "INVALID_CONTENT_TYPE", nil)
				return
			}

			// Limit payload size
			if r.ContentLength > m.config.MaxPayloadSize {
				m.validationErrorResponse(w, r, "payload too large", "PAYLOAD_TOO_LARGE", nil)
				return
			}

			// Use limited reader for additional safety
			limitedReader := http.MaxBytesReader(w, r.Body, m.config.MaxPayloadSize)
			defer limitedReader.Close()

			// Parse JSON with security checks
			decoder := json.NewDecoder(limitedReader)
			if m.config.StrictJSON {
				decoder.DisallowUnknownFields()
			}

			// Create new instance of target struct
			targetType := reflect.TypeOf(target)
			if targetType.Kind() == reflect.Pointer {
				targetType = targetType.Elem()
			}

			newTarget := reflect.New(targetType).Interface()

			if err := decoder.Decode(newTarget); err != nil {
				if m.config.LogValidationErrors && m.logger != nil {
					m.logger.Warn("JSON decode error",
						"error", err.Error(),
						"path", r.URL.Path,
						"method", r.Method,
						"ip", getRealIP(r),
					)
				}
				m.validationErrorResponse(w, r, "invalid JSON format", "INVALID_JSON", nil)
				return
			}

			// Validate struct
			if err := m.validator.Struct(newTarget); err != nil {
				if validationErrors, ok := err.(validator.ValidationErrors); ok {
					details := m.formatValidationErrors(validationErrors)

					if m.config.LogValidationErrors && m.logger != nil {
						m.logger.Warn("Validation error",
							"path", r.URL.Path,
							"method", r.Method,
							"ip", getRealIP(r),
							"errors", len(details),
						)
					}

					m.validationErrorResponse(w, r, "validation failed", "VALIDATION_ERROR", details)
					return
				}
				m.validationErrorResponse(w, r, "validation error", "VALIDATION_ERROR", nil)
				return
			}

			// Perform additional security validation if enabled
			if m.config.EnableSecurityValidation {
				if err := m.performSecurityValidation(newTarget); err != nil {
					if m.config.LogValidationErrors && m.logger != nil {
						m.logger.Warn("Security validation failed",
							"error", err.Error(),
							"path", r.URL.Path,
							"method", r.Method,
							"ip", getRealIP(r),
						)
					}
					m.validationErrorResponse(w, r, "security validation failed", "SECURITY_VALIDATION_ERROR", nil)
					return
				}
			}

			// Add validated data to context
			ctx := context.WithValue(r.Context(), "validated_data", newTarget)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// formatValidationErrors converts validator errors to our error format
func (m *ValidationMiddleware) formatValidationErrors(errs validator.ValidationErrors) []ValidationError {
	var details []ValidationError

	for _, err := range errs {
		var message string

		switch err.Tag() {
		case "required":
			message = fmt.Sprintf("Field '%s' is required", err.Field())
		case "email":
			message = fmt.Sprintf("Field '%s' must be a valid email address", err.Field())
		case "min":
			message = fmt.Sprintf("Field '%s' must be at least %s characters", err.Field(), err.Param())
		case "max":
			message = fmt.Sprintf("Field '%s' must be at most %s characters", err.Field(), err.Param())
		case "url":
			message = fmt.Sprintf("Field '%s' must be a valid URL", err.Field())
		case "uuid":
			message = fmt.Sprintf("Field '%s' must be a valid UUID", err.Field())
		case "domain":
			message = fmt.Sprintf("Field '%s' must be a valid domain name", err.Field())
		case "email_list":
			message = fmt.Sprintf("Field '%s' must contain valid email addresses", err.Field())
		case "safe_string":
			message = fmt.Sprintf("Field '%s' contains invalid or potentially unsafe characters", err.Field())
		default:
			message = fmt.Sprintf("Field '%s' failed validation (%s)", err.Field(), err.Tag())
		}

		// Don't include sensitive values in error messages
		value := ""
		if !m.isSensitiveField(err.Field()) {
			value = fmt.Sprintf("%v", err.Value())
		}

		details = append(details, ValidationError{
			Field:   err.Field(),
			Tag:     err.Tag(),
			Value:   value,
			Message: message,
		})
	}

	return details
}

// isSensitiveField checks if a field contains sensitive data
func (m *ValidationMiddleware) isSensitiveField(fieldName string) bool {
	sensitiveFields := []string{
		"password", "secret", "token", "key", "api_key",
		"private_key", "webhook_secret", "auth_token",
	}

	fieldLower := strings.ToLower(fieldName)
	for _, sensitive := range sensitiveFields {
		if strings.Contains(fieldLower, sensitive) {
			return true
		}
	}
	return false
}

// performSecurityValidation performs additional security checks
func (m *ValidationMiddleware) performSecurityValidation(data interface{}) error {
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}

		// Check string fields for security issues
		if field.Kind() == reflect.String {
			value := field.String()
			if err := m.validateStringField(fieldType.Name, value); err != nil {
				return fmt.Errorf("field %s: %w", fieldType.Name, err)
			}
		}

		// Check slice of strings
		if field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.String {
			for j := 0; j < field.Len(); j++ {
				value := field.Index(j).String()
				if err := m.validateStringField(fieldType.Name, value); err != nil {
					return fmt.Errorf("field %s[%d]: %w", fieldType.Name, j, err)
				}
			}
		}
	}

	return nil
}

// validateStringField performs security validation on string fields
func (m *ValidationMiddleware) validateStringField(fieldName, value string) error {
	if value == "" {
		return nil
	}

	// Check for potential injection attacks
	if m.containsSQLInjectionPattern(value) {
		return fmt.Errorf("potential SQL injection detected")
	}

	if m.containsXSSPattern(value) {
		return fmt.Errorf("potential XSS attack detected")
	}

	if m.containsCommandInjectionPattern(value) {
		return fmt.Errorf("potential command injection detected")
	}

	// Check for excessively long strings (DoS protection)
	if len(value) > 10000 { // 10KB limit for string fields
		return fmt.Errorf("string too long (max 10000 characters)")
	}

	return nil
}

// containsSQLInjectionPattern checks for basic SQL injection patterns
func (m *ValidationMiddleware) containsSQLInjectionPattern(value string) bool {
	lowerValue := strings.ToLower(value)
	sqlPatterns := []string{
		"union select", "drop table", "delete from", "update set",
		"insert into", "exec(", "execute(", "sp_", "xp_",
		"'; drop", "\"; drop", "' or '1'='1", "\" or \"1\"=\"1",
		"' or 1=1", "\" or 1=1", "' union", "\" union",
	}

	for _, pattern := range sqlPatterns {
		if strings.Contains(lowerValue, pattern) {
			return true
		}
	}
	return false
}

// containsXSSPattern checks for basic XSS patterns
func (m *ValidationMiddleware) containsXSSPattern(value string) bool {
	xssPatterns := []string{
		"<script", "</script>", "javascript:", "onload=", "onerror=",
		"onclick=", "onmouseover=", "onfocus=", "onblur=", "onchange=",
		"expression(", "eval(", "setTimeout(", "setInterval(",
	}

	lowerValue := strings.ToLower(value)
	for _, pattern := range xssPatterns {
		if strings.Contains(lowerValue, pattern) {
			return true
		}
	}
	return false
}

// containsCommandInjectionPattern checks for command injection patterns
func (m *ValidationMiddleware) containsCommandInjectionPattern(value string) bool {
	cmdPatterns := []string{
		"; rm ", "; cat ", "; ls ", "; pwd", "; whoami",
		"& rm ", "& cat ", "& ls ", "& pwd", "& whoami",
		"| rm ", "| cat ", "| ls ", "| pwd", "| whoami",
		"`rm ", "`cat ", "`ls ", "`pwd", "`whoami",
		"$(rm ", "$(cat ", "$(ls ", "$(pwd", "$(whoami",
	}

	lowerValue := strings.ToLower(value)
	for _, pattern := range cmdPatterns {
		if strings.Contains(lowerValue, pattern) {
			return true
		}
	}
	return false
}

// registerBuiltinValidators registers custom validation rules
func (m *ValidationMiddleware) registerBuiltinValidators() {
	// Domain name validator
	_ = m.validator.RegisterValidation("domain", func(fl validator.FieldLevel) bool {
		domain := fl.Field().String()
		if domain == "" {
			return false
		}

		// Basic domain validation regex
		domainRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
		return domainRegex.MatchString(domain) && len(domain) <= 253
	})

	// Email list validator
	_ = m.validator.RegisterValidation("email_list", func(fl validator.FieldLevel) bool {
		if fl.Field().Kind() != reflect.Slice {
			return false
		}

		emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

		for i := 0; i < fl.Field().Len(); i++ {
			email := fl.Field().Index(i).String()
			if !emailRegex.MatchString(email) {
				return false
			}
		}
		return true
	})

	// Safe string validator (no special characters that could be dangerous)
	_ = m.validator.RegisterValidation("safe_string", func(fl validator.FieldLevel) bool {
		value := fl.Field().String()
		// Allow alphanumeric, spaces, hyphens, underscores, and basic punctuation
		safeRegex := regexp.MustCompile(`^[a-zA-Z0-9\s\-_.@,!?()]+$`)
		return safeRegex.MatchString(value)
	})

	// Public key validator (basic PEM format check)
	_ = m.validator.RegisterValidation("public_key", func(fl validator.FieldLevel) bool {
		value := fl.Field().String()
		return strings.HasPrefix(value, "-----BEGIN PUBLIC KEY-----") &&
			strings.HasSuffix(value, "-----END PUBLIC KEY-----")
	})
}

// validationErrorResponse sends a validation error response
func (m *ValidationMiddleware) validationErrorResponse(w http.ResponseWriter, r *http.Request, message, code string, details []ValidationError) {
	render.Status(r, http.StatusBadRequest)
	render.JSON(w, r, ValidationErrorResponse{
		Error:   "Validation Error",
		Message: message,
		Code:    code,
		Details: details,
	})
}

// GetValidatedData retrieves validated data from request context
func GetValidatedData(ctx context.Context) (interface{}, bool) {
	data, ok := ctx.Value("validated_data").(interface{})
	return data, ok
}

// Helper function for getting real IP (defined in ratelimit.go)
// Reusing the same function to avoid duplication

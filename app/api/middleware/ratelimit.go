package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/httprate"
	"github.com/go-chi/render"
)

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	// General API rate limits
	RequestsPerMinute int
	BurstSize         int

	// Authentication endpoint rate limits (stricter)
	AuthRequestsPerMinute int
	AuthBurstSize         int

	// Per-user rate limits (for authenticated endpoints)
	UserRequestsPerMinute int
	UserBurstSize         int

	// Email sending rate limits
	EmailSendPerMinute int
	EmailSendBurst     int

	// Enable logging of rate limit violations
	LogViolations bool
	Logger        *slog.Logger
}

// DefaultRateLimitConfig returns production-ready rate limit configuration
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		// General API: 100 requests per minute per IP
		RequestsPerMinute: 100,
		BurstSize:         20,

		// Auth endpoints: 10 requests per minute per IP (stricter)
		AuthRequestsPerMinute: 10,
		AuthBurstSize:         5,

		// Per-user: 200 requests per minute (higher for authenticated users)
		UserRequestsPerMinute: 200,
		UserBurstSize:         40,

		// Email sending: 50 emails per minute per domain
		EmailSendPerMinute: 50,
		EmailSendBurst:     10,

		LogViolations: true,
	}
}

// RateLimitMiddleware provides comprehensive rate limiting
type RateLimitMiddleware struct {
	config RateLimitConfig
	logger *slog.Logger
}

// NewRateLimitMiddleware creates a new rate limiting middleware
func NewRateLimitMiddleware(config RateLimitConfig) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		config: config,
		logger: config.Logger,
	}
}

// RateLimitResponse represents the rate limit error response
type RateLimitResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// rateLimitHandler creates a custom rate limit handler with logging
func (m *RateLimitMiddleware) rateLimitHandler(requestType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.config.LogViolations && m.logger != nil {
			m.logger.Warn("Rate limit exceeded",
				"type", requestType,
				"ip", getRealIP(r),
				"path", r.URL.Path,
				"method", r.Method,
				"user_agent", r.Header.Get("User-Agent"),
			)
		}

		render.Status(r, http.StatusTooManyRequests)
		render.JSON(w, r, RateLimitResponse{
			Error:   "Rate limit exceeded",
			Message: "Too many requests. Please slow down and try again later.",
			Code:    "RATE_LIMIT_EXCEEDED",
		})
	}
}

// GeneralRateLimit applies general API rate limiting by IP
func (m *RateLimitMiddleware) GeneralRateLimit() func(http.Handler) http.Handler {
	return httprate.Limit(
		m.config.RequestsPerMinute,
		time.Minute,
		httprate.WithKeyFuncs(httprate.KeyByRealIP),
		httprate.WithLimitHandler(m.rateLimitHandler("general_api")),
	)
}

// AuthRateLimit applies stricter rate limiting for authentication endpoints
func (m *RateLimitMiddleware) AuthRateLimit() func(http.Handler) http.Handler {
	return httprate.Limit(
		m.config.AuthRequestsPerMinute,
		time.Minute,
		httprate.WithKeyFuncs(httprate.KeyByRealIP),
		httprate.WithLimitHandler(m.rateLimitHandler("auth")),
	)
}

// UserRateLimit applies per-user rate limiting for authenticated endpoints
func (m *RateLimitMiddleware) UserRateLimit() func(http.Handler) http.Handler {
	return httprate.Limit(
		m.config.UserRequestsPerMinute,
		time.Minute,
		httprate.WithKeyFuncs(m.userKeyFunc),
		httprate.WithLimitHandler(m.rateLimitHandler("user")),
	)
}

// EmailSendRateLimit applies rate limiting specifically for email sending
func (m *RateLimitMiddleware) EmailSendRateLimit() func(http.Handler) http.Handler {
	return httprate.Limit(
		m.config.EmailSendPerMinute,
		time.Minute,
		httprate.WithKeyFuncs(m.emailSendKeyFunc),
		httprate.WithLimitHandler(m.rateLimitHandler("email_send")),
	)
}

// userKeyFunc creates a rate limit key based on user ID from context
func (m *RateLimitMiddleware) userKeyFunc(r *http.Request) (string, error) {
	// Try to get user ID from context (set by auth middleware)
	if userID, ok := r.Context().Value("user_id").(string); ok && userID != "" {
		return "user:" + userID, nil
	}

	// Fallback to IP-based rate limiting if no user context
	return "ip:" + getRealIP(r), nil
}

// emailSendKeyFunc creates a rate limit key for email sending based on domain API key
func (m *RateLimitMiddleware) emailSendKeyFunc(r *http.Request) (string, error) {
	// Check for API key in header
	apiKey := r.Header.Get("X-API-Key")
	if apiKey != "" {
		// Use a hash or truncated version for privacy
		return "apikey:" + hashAPIKey(apiKey), nil
	}

	// Check for authenticated user
	if userID, ok := r.Context().Value("user_id").(string); ok && userID != "" {
		return "user_email:" + userID, nil
	}

	// Fallback to IP-based limiting
	return "ip_email:" + getRealIP(r), nil
}

// hashAPIKey creates a simple hash of the API key for rate limiting keys
func hashAPIKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return apiKey
	}
	// Use first 4 and last 4 characters for privacy
	return apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
}

// getRealIP extracts the real IP address from the request
func getRealIP(r *http.Request) string {
	// Check common headers for real IP
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		if idx := len(ip); idx > 0 {
			if comma := 0; comma < idx {
				for i, c := range ip {
					if c == ',' {
						comma = i
						break
					}
				}
				if comma > 0 {
					return ip[:comma]
				}
			}
			return ip
		}
	}

	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	if ip := r.Header.Get("X-Client-IP"); ip != "" {
		return ip
	}

	// Fallback to RemoteAddr
	return r.RemoteAddr
}

// CombinedRateLimit combines IP and user-based rate limiting
func (m *RateLimitMiddleware) CombinedRateLimit() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Apply both IP-based and user-based rate limiting
		ipLimit := m.GeneralRateLimit()
		userLimit := m.UserRateLimit()

		return ipLimit(userLimit(next))
	}
}

// HealthCheckBypass allows health check endpoints to bypass rate limiting
func (m *RateLimitMiddleware) HealthCheckBypass(paths ...string) func(http.Handler) http.Handler {
	bypassPaths := make(map[string]bool)
	for _, path := range paths {
		bypassPaths[path] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Bypass rate limiting for health check paths
			if bypassPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// Apply rate limiting for other paths
			m.GeneralRateLimit()(next).ServeHTTP(w, r)
		})
	}
}

// GetRateLimitInfo returns current rate limit information for debugging
type RateLimitInfo struct {
	RequestsPerMinute     int `json:"requests_per_minute"`
	AuthRequestsPerMinute int `json:"auth_requests_per_minute"`
	UserRequestsPerMinute int `json:"user_requests_per_minute"`
	EmailSendPerMinute    int `json:"email_send_per_minute"`
}

func (m *RateLimitMiddleware) GetRateLimitInfo() RateLimitInfo {
	return RateLimitInfo{
		RequestsPerMinute:     m.config.RequestsPerMinute,
		AuthRequestsPerMinute: m.config.AuthRequestsPerMinute,
		UserRequestsPerMinute: m.config.UserRequestsPerMinute,
		EmailSendPerMinute:    m.config.EmailSendPerMinute,
	}
}
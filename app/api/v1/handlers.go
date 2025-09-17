package v1

import (
	"context"
	"encoding/json"
	"log/slog"
	"mailvault/app/api/middleware"
	"mailvault/app/api/v1/admin"
	"mailvault/app/api/v1/auth"
	"mailvault/app/api/v1/domains"
	"mailvault/app/api/v1/emails"
	"mailvault/app/api/v1/send"
	"mailvault/app/api/v1/users"
	"mailvault/domain/smtp_stats"
	"net/http"
	"time"

	authDomain "mailvault/domain/auth"

	userDomain "mailvault/domain/user"

	"github.com/go-chi/chi/v5"
)

type ApiHandlers struct {
	AuthProvider     authDomain.Provider
	UserUseCase      users.UseCase
	AuthUseCase      auth.UseCase
	DomainUseCase    domains.UseCase
	EmailUseCase     emails.UseCase
	SMTPStatsUseCase *smtp_stats.UseCase
	UserAdminUseCase *userDomain.UseCase
	AuthSecretKey    string
	AuthTokenTTL     string
	Logger           *slog.Logger
	// For health checks
	HealthChecker HealthChecker
	// For metrics collection
	MetricsMiddleware *middleware.MetricsMiddleware
}

// HealthChecker interface for database health checks
type HealthChecker interface {
	Ping(ctx context.Context) error
}

func (h *ApiHandlers) Routes(r chi.Router) {
	// Initialize rate limiting middleware
	rateLimitConfig := middleware.DefaultRateLimitConfig()
	rateLimitConfig.Logger = h.Logger
	rateLimitMw := middleware.NewRateLimitMiddleware(rateLimitConfig)
	// Use the metrics middleware passed from main (already initialized)

	// Health endpoints without rate limiting (for monitoring)
	r.Get("/health", h.Health)
	r.Get("/ready", h.Readiness)

	// Metrics endpoint removed - now served on separate server

	// Initialize handlers
	// Parse JWT TTL
	authHandlers := auth.NewAuthHandlers(h.AuthProvider, h.AuthUseCase, []byte(h.AuthSecretKey), h.AuthTokenTTL)
	usersHandlers := users.NewUsersHandlers(h.UserUseCase)
	domainsHandlers := domains.NewDomainsHandlers(h.DomainUseCase)
	emailsHandlers := emails.NewEmailsHandlers(h.EmailUseCase)
	sendHandlers := send.NewSendHandlers(h.DomainUseCase)

	// Initialize middleware
	authMiddleware, err := middleware.NewAuthMiddleware(h.AuthSecretKey)
	if err != nil {
		h.Logger.Error("Failed to initialize auth middleware", "error", err)
		panic(err)
	}

	// Initialize admin auth middleware
	adminAuthMw, err := middleware.NewAuthMiddleware(h.AuthSecretKey)
	if err != nil {
		h.Logger.Error("Failed to initialize admin auth middleware", "error", err)
		panic(err)
	}

	// Initialize admin handlers
	adminHandlers := admin.NewAdminHandler(
		h.SMTPStatsUseCase,
		h.UserAdminUseCase,
		adminAuthMw,
		h.Logger,
	)

	r.Route("/api/v1", func(r chi.Router) {
		// Apply metrics collection to all API endpoints
		if h.MetricsMiddleware != nil {
			r.Use(h.MetricsMiddleware.MetricsHandler())
		}
		// Apply general rate limiting to all API endpoints
		r.Use(rateLimitMw.GeneralRateLimit())

		// Public auth endpoints with stricter rate limiting
		r.Group(func(r chi.Router) {
			r.Use(rateLimitMw.AuthRateLimit())
			r.Post("/register", authHandlers.Register)
			r.Post("/login", authHandlers.Login)
		})

		// Protected users endpoints
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware.RequireAuth)
			r.Use(rateLimitMw.UserRateLimit())
			r.Get("/me", usersHandlers.Me)
		})

		// Protected domain endpoints
		r.Route("/domains", func(r chi.Router) {
			r.Use(authMiddleware.RequireAuth)
			r.Use(rateLimitMw.UserRateLimit())
			r.Post("/", domainsHandlers.CreateDomain)
			r.Get("/", domainsHandlers.GetDomains)
			r.Get("/{id}", domainsHandlers.GetDomain)
			r.Put("/{id}", domainsHandlers.UpdateDomain)
			r.Delete("/{id}", domainsHandlers.DeleteDomain)

			// Email addresses for domains
			r.Route("/{domainId}/emails", func(r chi.Router) {
				r.Post("/", emailsHandlers.CreateEmailAddress)
				r.Get("/", emailsHandlers.GetEmailAddresses)
				r.Get("/{emailId}", emailsHandlers.GetEmailAddress)
				r.Put("/{emailId}", emailsHandlers.UpdateEmailAddress)
				r.Delete("/{emailId}", emailsHandlers.DeleteEmailAddress)
				r.Get("/{emailId}/received", emailsHandlers.GetReceivedEmails)
			})
		})

		// Email endpoints for CLI access
		r.Route("/emails", func(r chi.Router) {
			r.Use(authMiddleware.RequireAuth)
			r.Use(rateLimitMw.UserRateLimit())
			r.Get("/received", emailsHandlers.ListReceivedEmailsForUser)
		})

		// Direct access to received emails by ID
		r.Route("/received", func(r chi.Router) {
			r.Use(authMiddleware.RequireAuth)
			r.Use(rateLimitMw.UserRateLimit())
			r.Get("/{receivedEmailId}", emailsHandlers.GetReceivedEmail)
			r.Get("/{receivedEmailId}/parsed", emailsHandlers.GetParsedReceivedEmail)
			r.Delete("/{receivedEmailId}", emailsHandlers.DeleteReceivedEmail)
		})

		// Public email sending endpoint with dedicated rate limiting
		r.Group(func(r chi.Router) {
			r.Use(rateLimitMw.EmailSendRateLimit())
			r.Post("/send", sendHandlers.SendEmail)
		})
	})
	// Admin endpoints
	r.Mount("/admin/v1", adminHandlers.Routes())
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string                 `json:"status"`
	Timestamp string                 `json:"timestamp"`
	Version   string                 `json:"version,omitempty"`
	Checks    map[string]HealthCheck `json:"checks"`
}

// HealthCheck represents individual service health
type HealthCheck struct {
	Status   string `json:"status"`
	Duration string `json:"duration,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Health returns the health status of the API with detailed checks
// @Summary Health check
// @Description Check if the API is running and healthy, including database connectivity
// @Tags System
// @Produce json
// @Success 200 {object} HealthResponse "API is healthy"
// @Failure 503 {object} HealthResponse "API is unhealthy"
// @Router /health [get]
func (h *ApiHandlers) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	startTime := time.Now()
	response := HealthResponse{
		Status:    "ok",
		Timestamp: startTime.Format(time.RFC3339),
		Checks:    make(map[string]HealthCheck),
	}

	overallHealthy := true

	// Database health check
	if h.HealthChecker != nil {
		dbStart := time.Now()
		if err := h.HealthChecker.Ping(ctx); err != nil {
			response.Checks["database"] = HealthCheck{
				Status:   "unhealthy",
				Duration: time.Since(dbStart).String(),
				Error:    "database connection failed",
			}
			overallHealthy = false
		} else {
			response.Checks["database"] = HealthCheck{
				Status:   "healthy",
				Duration: time.Since(dbStart).String(),
			}
		}
	} else {
		response.Checks["database"] = HealthCheck{
			Status: "unknown",
			Error:  "health checker not configured",
		}
	}

	// Auth provider health check
	authStart := time.Now()
	if h.AuthProvider != nil {
		response.Checks["auth_provider"] = HealthCheck{
			Status:   "healthy",
			Duration: time.Since(authStart).String(),
		}
	} else {
		response.Checks["auth_provider"] = HealthCheck{
			Status: "unhealthy",
			Error:  "auth provider not configured",
		}
		overallHealthy = false
	}

	// API health check
	response.Checks["api"] = HealthCheck{
		Status:   "healthy",
		Duration: time.Since(startTime).String(),
	}

	// Set overall status
	if !overallHealthy {
		response.Status = "unhealthy"
	}

	// Set response status code
	statusCode := http.StatusOK
	if !overallHealthy {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.Logger.Error("failed to encode health response", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// Readiness returns the readiness status for Kubernetes probes
// @Summary Readiness check
// @Description Check if the API is ready to accept traffic (focused on critical dependencies)
// @Tags System
// @Produce json
// @Success 200 {object} HealthResponse "API is ready"
// @Failure 503 {object} HealthResponse "API is not ready"
// @Router /ready [get]
func (h *ApiHandlers) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	response := HealthResponse{
		Status:    "ready",
		Timestamp: startTime.Format(time.RFC3339),
		Checks:    make(map[string]HealthCheck),
	}

	ready := true

	// Database readiness check (critical for operation)
	if h.HealthChecker != nil {
		dbStart := time.Now()
		if err := h.HealthChecker.Ping(ctx); err != nil {
			response.Checks["database"] = HealthCheck{
				Status:   "not_ready",
				Duration: time.Since(dbStart).String(),
				Error:    "database not accessible",
			}
			ready = false
		} else {
			response.Checks["database"] = HealthCheck{
				Status:   "ready",
				Duration: time.Since(dbStart).String(),
			}
		}
	} else {
		response.Checks["database"] = HealthCheck{
			Status: "not_ready",
			Error:  "database not configured",
		}
		ready = false
	}

	// Auth provider readiness (critical for operation)
	if h.AuthProvider == nil {
		response.Checks["auth_provider"] = HealthCheck{
			Status: "not_ready",
			Error:  "auth provider not configured",
		}
		ready = false
	} else {
		response.Checks["auth_provider"] = HealthCheck{
			Status: "ready",
		}
	}

	// Set overall status
	if !ready {
		response.Status = "not_ready"
	}

	// Set response status code
	statusCode := http.StatusOK
	if !ready {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.Logger.Error("failed to encode readiness response", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

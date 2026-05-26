package v1

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/mailvault/mailvault/app/api/middleware"
	"github.com/mailvault/mailvault/app/api/v1/admin"
	"github.com/mailvault/mailvault/app/api/v1/auth"
	"github.com/mailvault/mailvault/app/api/v1/domains"
	"github.com/mailvault/mailvault/app/api/v1/emails"
	"github.com/mailvault/mailvault/app/api/v1/send"
	"github.com/mailvault/mailvault/app/api/v1/users"
	"github.com/mailvault/mailvault/app/api/v1/webhook_configs"
	"github.com/mailvault/mailvault/domain/smtp_stats"

	authDomain "github.com/mailvault/mailvault/domain/auth"
	userDomain "github.com/mailvault/mailvault/domain/user"

	"github.com/go-chi/chi/v5"
)

type ApiHandlers struct {
	AuthProvider         authDomain.Provider
	UserUseCase          users.UseCase
	AuthUseCase          auth.UseCase
	DomainUseCase        domains.UseCase
	EmailUseCase         emails.UseCase
	SMTPStatsUseCase     *smtp_stats.UseCase
	UserAdminUseCase     *userDomain.UseCase
	WebhookConfigUseCase webhook_configs.UseCase
	AuthSecretKey        string
	AuthTokenTTL         string
	Logger               *slog.Logger
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

	r.Get("/health", h.Health)
	r.Get("/ready", h.Readiness)

	authHandlers := auth.NewAuthHandlers(h.AuthProvider, h.AuthUseCase, []byte(h.AuthSecretKey), h.AuthTokenTTL)
	usersHandlers := users.NewUsersHandlers(h.UserUseCase)
	domainsHandlers := domains.NewDomainsHandlers(h.DomainUseCase, h.Logger)
	emailsHandlers := emails.NewEmailsHandlers(h.EmailUseCase)
	sendHandlers := send.NewSendHandlers(h.DomainUseCase, h.Logger)
	var webhookConfigHandlers *webhook_configs.WebhookConfigHandlers
	if h.WebhookConfigUseCase != nil {
		webhookConfigHandlers = webhook_configs.NewWebhookConfigHandlers(h.WebhookConfigUseCase, h.Logger)
	}

	authMiddleware, err := middleware.NewAuthMiddleware(h.AuthSecretKey)
	if err != nil {
		h.Logger.Error("Failed to initialize auth middleware", "error", err)
		panic(err)
	}

	adminAuthMw, err := middleware.NewAuthMiddleware(h.AuthSecretKey)
	if err != nil {
		h.Logger.Error("Failed to initialize admin auth middleware", "error", err)
		panic(err)
	}

	adminHandlers := admin.NewAdminHandler(
		h.SMTPStatsUseCase,
		h.UserAdminUseCase,
		adminAuthMw,
		h.Logger,
	)

	r.Route("/api/v1", func(r chi.Router) {
		if h.MetricsMiddleware != nil {
			r.Use(h.MetricsMiddleware.MetricsHandler())
		}
		r.Use(rateLimitMw.GeneralRateLimit())

		r.Group(func(r chi.Router) {
			r.Use(rateLimitMw.AuthRateLimit())
			r.Post("/register", authHandlers.Register)
			r.Post("/login", authHandlers.Login)
			r.Post("/auth/confirm", authHandlers.ConfirmEmail)
			r.Post("/auth/confirm-token", authHandlers.ConfirmEmailWithToken)
			r.Post("/auth/resend-confirmation", authHandlers.ResendConfirmation)
		})

		r.Group(func(r chi.Router) {
			r.Use(authMiddleware.RequireAuth)
			r.Use(rateLimitMw.UserRateLimit())
			r.Get("/me", usersHandlers.Me)
		})

		r.Route("/domains", func(r chi.Router) {
			r.Use(authMiddleware.RequireAuth)
			r.Use(rateLimitMw.UserRateLimit())
			r.Post("/", domainsHandlers.CreateDomain)
			r.Get("/", domainsHandlers.GetDomains)
			r.Get("/{id}", domainsHandlers.GetDomain)
			r.Put("/{id}", domainsHandlers.UpdateDomain)
			r.Delete("/{id}", domainsHandlers.DeleteDomain)

			r.Post("/{id}/validate", domainsHandlers.ValidateDomain)
			r.Post("/{id}/validation/retry", domainsHandlers.RetryValidation)

			r.Route("/{domainId}/emails", func(r chi.Router) {
				r.Post("/", emailsHandlers.CreateEmailAddress)
				r.Get("/", emailsHandlers.GetEmailAddresses)
				r.Get("/{emailId}", emailsHandlers.GetEmailAddress)
				r.Put("/{emailId}", emailsHandlers.UpdateEmailAddress)
				r.Delete("/{emailId}", emailsHandlers.DeleteEmailAddress)
				r.Get("/{emailId}/received", emailsHandlers.GetReceivedEmails)
			})

			if webhookConfigHandlers != nil {
				r.Route("/{domainId}/webhooks", func(r chi.Router) {
					r.Post("/", webhookConfigHandlers.CreateWebhookConfig)
					r.Get("/", webhookConfigHandlers.ListWebhookConfigs)
					r.Post("/from-template", webhookConfigHandlers.CreateFromTemplate)
					r.Get("/{webhookId}", webhookConfigHandlers.GetWebhookConfig)
					r.Put("/{webhookId}", webhookConfigHandlers.UpdateWebhookConfig)
					r.Delete("/{webhookId}", webhookConfigHandlers.DeleteWebhookConfig)
					r.Post("/{webhookId}/enable", webhookConfigHandlers.EnableWebhookConfig)
					r.Post("/{webhookId}/disable", webhookConfigHandlers.DisableWebhookConfig)
					r.Post("/{webhookId}/test", webhookConfigHandlers.TestWebhookConfig)
					r.Get("/{webhookId}/health", webhookConfigHandlers.GetWebhookHealth)
					r.Get("/{webhookId}/metrics", webhookConfigHandlers.GetWebhookMetrics)
					r.Get("/{webhookId}/audit", webhookConfigHandlers.GetWebhookAuditLog)
				})
			}
		})

		if webhookConfigHandlers != nil {
			r.Group(func(r chi.Router) {
				r.Use(authMiddleware.RequireAuth)
				r.Use(rateLimitMw.UserRateLimit())
				r.Get("/webhook-templates", webhookConfigHandlers.ListWebhookTemplates)
			})
		}

		r.Route("/emails", func(r chi.Router) {
			r.Use(authMiddleware.RequireAuth)
			r.Use(rateLimitMw.UserRateLimit())
			r.Get("/received", emailsHandlers.ListReceivedEmailsForUser)
		})

		r.Route("/received", func(r chi.Router) {
			r.Use(authMiddleware.RequireAuth)
			r.Use(rateLimitMw.UserRateLimit())
			r.Get("/{receivedEmailId}", emailsHandlers.GetReceivedEmail)
			r.Get("/{receivedEmailId}/parsed", emailsHandlers.GetParsedReceivedEmail)
			r.Delete("/{receivedEmailId}", emailsHandlers.DeleteReceivedEmail)
		})

		r.Group(func(r chi.Router) {
			r.Use(rateLimitMw.EmailSendRateLimit())
			r.Post("/send", sendHandlers.SendEmail)
		})
	})

	// Admin endpoints
	r.Mount("/admin/v1", adminHandlers.Routes())
}

type HealthResponse struct {
	Status    string                 `json:"status"`
	Timestamp string                 `json:"timestamp"`
	Version   string                 `json:"version,omitempty"`
	Checks    map[string]HealthCheck `json:"checks"`
}

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

	response.Checks["api"] = HealthCheck{
		Status:   "healthy",
		Duration: time.Since(startTime).String(),
	}

	if !overallHealthy {
		response.Status = "unhealthy"
	}
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

	if !ready {
		response.Status = "not_ready"
	}
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

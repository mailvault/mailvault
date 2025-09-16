package v1

import (
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
}

func (h *ApiHandlers) Routes(r chi.Router) {
	r.Get("/health", h.Health)

	// Initialize handlers
	// Parse JWT TTL
	authHandlers := auth.NewAuthHandlers(h.AuthProvider, h.AuthUseCase, []byte(h.AuthSecretKey), h.AuthTokenTTL)
	usersHandlers := users.NewUsersHandlers(h.UserUseCase)
	domainsHandlers := domains.NewDomainsHandlers(h.DomainUseCase)
	emailsHandlers := emails.NewEmailsHandlers(h.EmailUseCase)
	sendHandlers := send.NewSendHandlers(h.DomainUseCase)

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(h.AuthSecretKey)

	// Initialize admin auth middleware
	adminAuthMw := middleware.NewAuthMiddleware(h.AuthSecretKey)

	// Initialize admin handlers
	adminHandlers := admin.NewAdminHandler(
		h.SMTPStatsUseCase,
		h.UserAdminUseCase,
		adminAuthMw,
		h.Logger,
	)

	r.Route("/api/v1", func(r chi.Router) {
		// Public auth endpoints
		r.Post("/register", authHandlers.Register)
		r.Post("/login", authHandlers.Login)

		// Protected users endpoints
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware.RequireAuth)
			r.Get("/me", usersHandlers.Me)
		})

		// Protected domain endpoints
		r.Route("/domains", func(r chi.Router) {
			r.Use(authMiddleware.RequireAuth)
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
			r.Get("/received", emailsHandlers.ListReceivedEmailsForUser)
		})

		// Direct access to received emails by ID
		r.Route("/received", func(r chi.Router) {
			r.Use(authMiddleware.RequireAuth)
			r.Get("/{receivedEmailId}", emailsHandlers.GetReceivedEmail)
			r.Get("/{receivedEmailId}/parsed", emailsHandlers.GetParsedReceivedEmail)
			r.Delete("/{receivedEmailId}", emailsHandlers.DeleteReceivedEmail)
		})

		// Public email sending endpoint (API key auth)
		r.Post("/send", sendHandlers.SendEmail)
	})
	// Admin endpoints
	r.Mount("/admin/v1", adminHandlers.Routes())
}

// Health returns the health status of the API
// @Summary Health check
// @Description Check if the API is running and healthy
// @Tags System
// @Produce json
// @Success 200 {object} map[string]string "API is healthy"
// @Router /health [get]
func (h *ApiHandlers) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

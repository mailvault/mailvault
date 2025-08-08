package v1

import (
	"net/http"

	"mailsafe/domain/auth"
	domainpkg "mailsafe/domain/domain"
	"mailsafe/domain/email"
	"mailsafe/domain/user"
	"mailsafe/internal/api/middleware"
	"mailsafe/internal/config"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type ApiHandlers struct {
	AuthProvider  auth.Provider
	UserUseCase   *user.UseCase
	DomainUseCase *domainpkg.UseCase
	EmailUseCase  *email.UseCase
	Config        config.Config
}

func (h *ApiHandlers) Routes(r chi.Router) {
	r.Get("/health", h.Health)

	// Initialize handlers
	// Parse JWT TTL
	jwtTTL := parseJWTTTL(h.Config.AuthTokenTTL)
	authHandlers := NewAuthHandlers(h.AuthProvider, h.UserUseCase, []byte(h.Config.AuthSecretKey), jwtTTL)
	domainsHandlers := NewDomainsHandlers(h.DomainUseCase)
	emailsHandlers := NewEmailsHandlers(h.EmailUseCase)
	sendHandlers := NewSendHandlers(h.DomainUseCase)

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(h.Config)

	r.Route("/api/v1", func(r chi.Router) {
		// Public auth endpoints
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authHandlers.Register)
			r.Post("/login", authHandlers.Login)

			// Protected auth endpoints
			r.Group(func(r chi.Router) {
				r.Use(authMiddleware.RequireAuth)
				r.Get("/me", authHandlers.Me)
			})
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

		// Public email sending endpoint (API key auth)
		r.Post("/send", sendHandlers.SendEmail)
	})
}

func (h *ApiHandlers) Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

type ErrorResponseBody struct {
	Error string `json:"error"`
}

func errorResponse(w http.ResponseWriter, r *http.Request, code int, err error) {
	render.Status(r, code)
	render.JSON(w, r, ErrorResponseBody{
		Error: err.Error(),
	})
}

func unknownErrorResponse(w http.ResponseWriter, r *http.Request) {
	render.Status(r, http.StatusInternalServerError)
	render.PlainText(w, r, http.StatusText(http.StatusInternalServerError))
}

package v1

import (
	"net/http"
	
	"privatemail/domain/user"
	domain "privatemail/domain/domain" 
	"privatemail/domain/email"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type ApiHandlers struct {
	UserUseCase   *user.UseCase
	DomainUseCase *domain.UseCase
	EmailUseCase  *email.UseCase
}

func (h *ApiHandlers) Routes(r chi.Router) {
	r.Get("/health", h.Health)
	r.Route("/api/v1", func(r chi.Router) {
		// TODO: Implement PrivateMail API routes
		// r.Route("/users", func(r chi.Router) { ... })
		// r.Route("/domains", func(r chi.Router) { ... })
		// r.Route("/emails", func(r chi.Router) { ... })
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

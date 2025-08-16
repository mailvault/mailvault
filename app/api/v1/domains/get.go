package domains

import (
	"mailvault/app/api"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// GetDomains gets all domains for the authenticated user
// @Summary Get user domains
// @Description Retrieve all domains belonging to the authenticated user
// @Tags Domains
// @Produce json
// @Security BearerAuth
// @Success 200 {array} DomainResult "List of user domains"
// @Failure 401 {object} ErrorResponseBody "Unauthorized"
// @Failure 500 {object} ErrorResponseBody "Internal server error"
// @Router /domains [get]
func (h *DomainsHandlers) GetDomains(w http.ResponseWriter, r *http.Request) {
	userID, err := api.GetUserIDFromContext(r)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusUnauthorized, err)
		return
	}

	domains, err := h.domainUseCase.GetDomainsByUserID(r.Context(), userID)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	results := make([]*DomainResult, len(domains))
	for i, domain := range domains {
		results[i] = h.mapDomainToResult(domain)
	}

	api.SuccessResponse(w, r, results)
}

// GetDomain gets a specific domain by ID
// @Summary Get domain by ID
// @Description Retrieve a specific domain by its ID (must belong to authenticated user)
// @Tags Domains
// @Produce json
// @Security BearerAuth
// @Param id path string true "Domain ID" format(uuid)
// @Success 200 {object} DomainResult "Domain details"
// @Failure 400 {object} ErrorResponseBody "Bad request - invalid domain ID"
// @Failure 401 {object} ErrorResponseBody "Unauthorized"
// @Failure 403 {object} ErrorResponseBody "Forbidden - domain does not belong to user"
// @Failure 404 {object} ErrorResponseBody "Domain not found"
// @Router /domains/{id} [get]
func (h *DomainsHandlers) GetDomain(w http.ResponseWriter, r *http.Request) {
	domainIDStr := chi.URLParam(r, "id")
	domainID, err := api.ParseUUID(domainIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	userID, err := api.GetUserIDFromContext(r)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusUnauthorized, err)
		return
	}

	domain, err := h.domainUseCase.GetDomainByID(r.Context(), domainID)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusNotFound, err)
		return
	}

	// Verify domain belongs to user
	if domain.UserID != userID {
		api.ErrorResponse(w, r, http.StatusForbidden, api.ErrUnauthorized)
		return
	}

	result := h.mapDomainToResult(domain)
	api.SuccessResponse(w, r, result)
}

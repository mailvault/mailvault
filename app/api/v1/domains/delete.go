package domains

import (
	"github.com/mailvault/mailvault/app/api"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// DeleteDomain deletes a domain
// @Summary Delete domain
// @Description Delete a domain and all associated email addresses and received emails
// @Tags Domains
// @Security BearerAuth
// @Param id path string true "Domain ID" format(uuid)
// @Success 204 "Domain deleted successfully"
// @Failure 400 {object} models.ErrorResponseBody "Bad request"
// @Failure 401 {object} models.ErrorResponseBody "Unauthorized"
// @Failure 404 {object} models.ErrorResponseBody "Domain not found or does not belong to user"
// @Router /domains/{id} [delete]
func (h *DomainsHandlers) DeleteDomain(w http.ResponseWriter, r *http.Request) {
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

	err = h.domainUseCase.DeleteDomain(r.Context(), domainID, userID)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	api.NoContentResponse(w, r)
}

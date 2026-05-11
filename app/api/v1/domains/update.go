package domains

import (
	"encoding/json"
	"mailvault/app/api"
	domainpkg "mailvault/domain/domain"
	"mailvault/domain/entities"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// UpdateDomainRequest represents domain update request
type UpdateDomainRequest struct {
	PublicKey          *string                     `json:"public_key,omitempty"`
	StorageEnabled     *bool                       `json:"storage_enabled,omitempty"`
	AutoCreateAddress  *bool                       `json:"auto_create_address,omitempty"`
	VerificationStatus entities.VerificationStatus `json:"verification_status,omitempty"`
}

// UpdateDomain updates an existing domain
// @Summary Update domain
// @Description Update domain settings including public key, verification status, and webhook configuration
// @Tags Domains
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Domain ID" format(uuid)
// @Param request body UpdateDomainRequest true "Domain update details"
// @Success 200 {object} DomainResult "Domain updated successfully"
// @Failure 400 {object} models.ErrorResponseBody "Bad request"
// @Failure 401 {object} models.ErrorResponseBody "Unauthorized"
// @Failure 403 {object} models.ErrorResponseBody "Forbidden - domain does not belong to user"
// @Failure 404 {object} models.ErrorResponseBody "Domain not found"
// @Router /domains/{id} [put]
func (h *DomainsHandlers) UpdateDomain(w http.ResponseWriter, r *http.Request) {
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

	var req UpdateDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Verify domain belongs to user
	domain, err := h.domainUseCase.GetDomainByID(r.Context(), domainID)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusNotFound, err)
		return
	}

	if domain.UserID != userID {
		api.ErrorResponse(w, r, http.StatusForbidden, api.ErrUnauthorized)
		return
	}

	// Update domain
	updatedDomain, err := h.domainUseCase.UpdateDomain(r.Context(), domainID, domainpkg.UpdateDomainInput{
		PublicKey:         req.PublicKey,
		StorageEnabled:    req.StorageEnabled,
		AutoCreateAddress: req.AutoCreateAddress,
	})
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	result := h.mapDomainToResult(updatedDomain)
	api.SuccessResponse(w, r, result)
}

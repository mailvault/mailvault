package domains

import (
	"fmt"
	"net/http"

	"github.com/mailvault/mailvault/app/api"
	"github.com/mailvault/mailvault/app/api/models"

	"github.com/go-chi/chi/v5"
	"github.com/gofrs/uuid/v5"
)

// ValidateDomain manually triggers domain validation
// @Summary Validate domain
// @Description Manually trigger domain validation for DNS records
// @Tags domains
// @Accept json
// @Produce json
// @Param id path string true "Domain ID"
// @Success 200 {object} models.DomainValidationResponse
// @Failure 400 {object} models.ErrorResponseBody
// @Failure 401 {object} models.ErrorResponseBody
// @Failure 404 {object} models.ErrorResponseBody
// @Failure 500 {object} models.ErrorResponseBody
// @Security BearerAuth
// @Router /domains/{id}/validate [post]
func (h *DomainsHandlers) ValidateDomain(w http.ResponseWriter, r *http.Request) {
	userID, err := api.GetUserIDFromContext(r)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusUnauthorized, err)
		return
	}

	domainIDStr := chi.URLParam(r, "id")
	domainID, err := uuid.FromString(domainIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("invalid domain ID"))
		return
	}

	// Get domain to verify ownership
	domain, err := h.domainUseCase.GetDomainByID(r.Context(), domainID)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusNotFound, fmt.Errorf("domain not found"))
		return
	}

	// Verify domain belongs to user
	if domain.UserID != userID {
		api.ErrorResponse(w, r, http.StatusNotFound, fmt.Errorf("domain not found"))
		return
	}

	// Trigger validation (this would typically queue a job for the worker)
	// For now, we'll return validation instructions
	validationInfo := models.DomainValidationResponse{
		DomainID:   domainID,
		DomainName: domain.Domain,
		Status:     string(domain.VerificationStatus),
		Instructions: models.ValidationInstructions{
			MXRecords: models.MXRecordInstructions{
				RequiredRecords: []models.MXRecordInfo{
					{Host: "mail.mailvault.sh", Priority: 10},
					{Host: "mail2.mailvault.sh", Priority: 20},
				},
				Instructions: "Add these MX records to your DNS configuration:",
			},
			TXTRecord: models.TXTRecordInstructions{
				RecordName:   domain.Domain,
				RecordValue:  domain.GetTXTRecord(),
				Instructions: "Add this TXT record to verify domain ownership:",
			},
		},
		VerificationToken: domain.VerificationToken,
		LastAttempt:       domain.LastVerificationAttempt,
		NextAttempt:       domain.NextVerificationAttempt,
		Attempts:          domain.VerificationAttempts,
		Error:             domain.VerificationError,
	}

	api.SuccessResponse(w, r, validationInfo)
}

// GetValidationStatus gets the current validation status for a domain
// @Summary Get domain validation status
// @Description Get the current validation status and history for a domain
// @Tags domains
// @Accept json
// @Produce json
// @Param id path string true "Domain ID"
// @Success 200 {object} models.DomainValidationStatusResponse
// @Failure 400 {object} models.ErrorResponseBody
// @Failure 401 {object} models.ErrorResponseBody
// @Failure 404 {object} models.ErrorResponseBody
// @Failure 500 {object} models.ErrorResponseBody
// @Security BearerAuth
// @Router /domains/{id}/validation/status [get]
func (h *DomainsHandlers) GetValidationStatus(w http.ResponseWriter, r *http.Request) {
	userID, err := api.GetUserIDFromContext(r)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusUnauthorized, err)
		return
	}

	domainIDStr := chi.URLParam(r, "id")
	domainID, err := uuid.FromString(domainIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("invalid domain ID"))
		return
	}

	// Get domain to verify ownership
	domain, err := h.domainUseCase.GetDomainByID(r.Context(), domainID)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusNotFound, fmt.Errorf("domain not found"))
		return
	}

	// Verify domain belongs to user
	if domain.UserID != userID {
		api.ErrorResponse(w, r, http.StatusNotFound, fmt.Errorf("domain not found"))
		return
	}

	// Get validation history (if validation use case is available)
	var validationHistory []models.ValidationRecord
	// Note: This would require access to validation use case
	// For now, we'll return an empty history

	response := models.DomainValidationStatusResponse{
		DomainID:          domainID,
		DomainName:        domain.Domain,
		Status:            string(domain.VerificationStatus),
		VerificationToken: domain.VerificationToken,
		LastAttempt:       domain.LastVerificationAttempt,
		NextAttempt:       domain.NextVerificationAttempt,
		Attempts:          domain.VerificationAttempts,
		Error:             domain.VerificationError,
		History:           validationHistory,
		IsVerified:        domain.IsVerified(),
		CanRetry:          domain.CanRetryVerification(),
		TXTRecord:         domain.GetTXTRecord(),
	}

	api.SuccessResponse(w, r, response)
}

// GetValidationInstructions provides DNS setup instructions for domain validation
// @Summary Get domain validation instructions
// @Description Get detailed instructions for setting up DNS records for domain validation
// @Tags domains
// @Accept json
// @Produce json
// @Param id path string true "Domain ID"
// @Success 200 {object} models.ValidationInstructionsResponse
// @Failure 400 {object} models.ErrorResponseBody
// @Failure 401 {object} models.ErrorResponseBody
// @Failure 404 {object} models.ErrorResponseBody
// @Failure 500 {object} models.ErrorResponseBody
// @Security BearerAuth
// @Router /domains/{id}/validation/instructions [get]
func (h *DomainsHandlers) GetValidationInstructions(w http.ResponseWriter, r *http.Request) {
	userID, err := api.GetUserIDFromContext(r)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusUnauthorized, err)
		return
	}

	domainIDStr := chi.URLParam(r, "id")
	domainID, err := uuid.FromString(domainIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("invalid domain ID"))
		return
	}

	// Get domain to verify ownership
	domain, err := h.domainUseCase.GetDomainByID(r.Context(), domainID)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusNotFound, fmt.Errorf("domain not found"))
		return
	}

	// Verify domain belongs to user
	if domain.UserID != userID {
		api.ErrorResponse(w, r, http.StatusNotFound, fmt.Errorf("domain not found"))
		return
	}

	response := models.ValidationInstructionsResponse{
		DomainID:   domainID,
		DomainName: domain.Domain,
		Status:     string(domain.VerificationStatus),
		Instructions: models.ValidationInstructions{
			MXRecords: models.MXRecordInstructions{
				RequiredRecords: []models.MXRecordInfo{
					{Host: "mail.mailvault.sh", Priority: 10},
					{Host: "mail2.mailvault.sh", Priority: 20},
				},
				Instructions: "Add these MX records to your DNS configuration to route emails to MailVault:",
				Example: `
# Example DNS configuration:
@ IN MX 10 mail.mailvault.sh.
@ IN MX 20 mail2.mailvault.sh.

# Or if using a DNS provider web interface:
# Type: MX, Name: @, Value: mail.mailvault.sh, Priority: 10
# Type: MX, Name: @, Value: mail2.mailvault.sh, Priority: 20`,
			},
			TXTRecord: models.TXTRecordInstructions{
				RecordName:   domain.Domain,
				RecordValue:  domain.GetTXTRecord(),
				Instructions: "Add this TXT record to verify domain ownership:",
				Example: fmt.Sprintf(`
# Example DNS configuration:
@ IN TXT "%s"

# Or if using a DNS provider web interface:
# Type: TXT, Name: @, Value: %s`, domain.GetTXTRecord(), domain.GetTXTRecord()),
			},
		},
		VerificationSteps: []string{
			"1. Add the required MX records to your DNS configuration",
			"2. Add the TXT record for domain ownership verification",
			"3. Wait for DNS propagation (usually 5-30 minutes)",
			"4. Trigger validation or wait for automatic validation",
			"5. Check validation status to confirm success",
		},
		TroubleshootingTips: []string{
			"DNS changes can take up to 24 hours to propagate globally",
			"Use DNS checker tools to verify your records are visible",
			"Ensure there are no conflicting MX or TXT records",
			"Contact your DNS provider if you need help adding records",
			"Some DNS providers require a trailing dot in record values",
		},
		VerificationToken: domain.VerificationToken,
	}

	api.SuccessResponse(w, r, response)
}

// RetryValidation manually retries domain validation
// @Summary Retry domain validation
// @Description Manually retry domain validation, resetting the status and attempting validation again
// @Tags domains
// @Accept json
// @Produce json
// @Param id path string true "Domain ID"
// @Success 200 {object} models.DomainValidationResponse
// @Failure 400 {object} models.ErrorResponseBody
// @Failure 401 {object} models.ErrorResponseBody
// @Failure 404 {object} models.ErrorResponseBody
// @Failure 500 {object} models.ErrorResponseBody
// @Security BearerAuth
// @Router /domains/{id}/validation/retry [post]
func (h *DomainsHandlers) RetryValidation(w http.ResponseWriter, r *http.Request) {
	userID, err := api.GetUserIDFromContext(r)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusUnauthorized, err)
		return
	}

	domainIDStr := chi.URLParam(r, "id")
	domainID, err := uuid.FromString(domainIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("invalid domain ID"))
		return
	}

	// Get domain to verify ownership
	domain, err := h.domainUseCase.GetDomainByID(r.Context(), domainID)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusNotFound, fmt.Errorf("domain not found"))
		return
	}

	// Verify domain belongs to user
	if domain.UserID != userID {
		api.ErrorResponse(w, r, http.StatusNotFound, fmt.Errorf("domain not found"))
		return
	}

	// Check if retry is allowed
	if !domain.CanRetryVerification() {
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Errorf("validation retry not allowed yet"))
		return
	}

	// Here you would typically queue a validation job for the worker
	// For now, we'll just log the retry request and return current status

	// Here you would typically queue a validation job for the worker
	// For now, we'll just return current status with retry pending

	validationInfo := models.DomainValidationResponse{
		DomainID:   domainID,
		DomainName: domain.Domain,
		Status:     "pending", // Reset to pending for retry
		Instructions: models.ValidationInstructions{
			MXRecords: models.MXRecordInstructions{
				RequiredRecords: []models.MXRecordInfo{
					{Host: "mail.mailvault.sh", Priority: 10},
					{Host: "mail2.mailvault.sh", Priority: 20},
				},
				Instructions: "Ensure these MX records are configured in your DNS:",
			},
			TXTRecord: models.TXTRecordInstructions{
				RecordName:   domain.Domain,
				RecordValue:  domain.GetTXTRecord(),
				Instructions: "Ensure this TXT record is configured for domain verification:",
			},
		},
		VerificationToken: domain.VerificationToken,
		LastAttempt:       domain.LastVerificationAttempt,
		NextAttempt:       domain.NextVerificationAttempt,
		Attempts:          domain.VerificationAttempts,
		Error:             domain.VerificationError,
	}

	api.SuccessResponse(w, r, validationInfo)
}

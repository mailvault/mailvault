package emails

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"mailvault/app/api"
	"mailvault/domain/email"
	"mailvault/domain/entities"

	"github.com/go-chi/chi/v5"
	"github.com/gofrs/uuid/v5"
)

//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/usecase.go . UseCase

// UseCase defines the behavior required by this package from the email use case.
type UseCase interface {
	CreateEmailAddressFromInput(ctx context.Context, req email.CreateEmailAddressInput) (*entities.EmailAddress, error)
	GetEmailAddressesByDomainID(ctx context.Context, domainID uuid.UUID) ([]*entities.EmailAddress, error)
	GetEmailAddressByID(ctx context.Context, id uuid.UUID) (*entities.EmailAddress, error)
	UpdateEmailAddress(ctx context.Context, id uuid.UUID, req email.UpdateEmailAddressInput) (*entities.EmailAddress, error)
	DeleteEmailAddress(ctx context.Context, id uuid.UUID) error
	GetReceivedEmails(ctx context.Context, emailID uuid.UUID, limit, offset int) ([]*entities.ReceivedEmail, error)
	GetReceivedEmailByID(ctx context.Context, receivedEmailID uuid.UUID, userID uuid.UUID) (*entities.ReceivedEmail, error)
	DeleteReceivedEmail(ctx context.Context, receivedEmailID uuid.UUID, userID uuid.UUID) error
}

// EmailsHandlers contains email-related endpoints
type EmailsHandlers struct {
	emailUseCase UseCase
}

func NewEmailsHandlers(emailUseCase UseCase) *EmailsHandlers {
	return &EmailsHandlers{
		emailUseCase: emailUseCase,
	}
}

// CreateEmailRequest represents email address creation request
type CreateEmailRequest struct {
	LocalPart        string   `json:"local_part" validate:"required"`
	IsCatchAll       bool     `json:"is_catch_all"`
	ForwardAddresses []string `json:"forward_addresses,omitempty"`
}

// UpdateEmailRequest represents email address update request
type UpdateEmailRequest struct {
	IsCatchAll       *bool    `json:"is_catch_all,omitempty"`
	ForwardAddresses []string `json:"forward_addresses,omitempty"`
}

// EmailAddressResult represents email address data in responses
type EmailAddressResult struct {
	ID               string   `json:"id"`
	DomainID         string   `json:"domain_id"`
	LocalPart        string   `json:"local_part"`
	FullAddress      string   `json:"full_address"`
	IsCatchAll       bool     `json:"is_catch_all"`
	ForwardAddresses []string `json:"forward_addresses"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
}

// ReceivedEmailResult represents received email data in responses
type ReceivedEmailResult struct {
	ID             string `json:"id"`
	SequenceNumber int    `json:"sequence_number"`
	FromAddress    string `json:"from_address"`
	Subject        string `json:"subject"`
	EncryptedBody  string `json:"encrypted_body"`
	ReceivedAt     string `json:"received_at"`
}

// CreateEmailAddress creates a new email address for a domain
// @Summary Create email address
// @Description Create a new email address for a specific domain with optional forwarding and catch-all settings
// @Tags Emails
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param domainId path string true "Domain ID" format(uuid)
// @Param request body CreateEmailRequest true "Email address creation details"
// @Success 201 {object} EmailAddressResult "Email address created successfully"
// @Failure 400 {object} models.ErrorResponseBody "Bad request"
// @Failure 401 {object} models.ErrorResponseBody "Unauthorized"
// @Router /domains/{domainId}/emails [post]
func (h *EmailsHandlers) CreateEmailAddress(w http.ResponseWriter, r *http.Request) {
	domainIDStr := chi.URLParam(r, "domainId")
	domainID, err := api.ParseUUID(domainIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	var req CreateEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Create email address
	emailAddress, err := h.emailUseCase.CreateEmailAddressFromInput(r.Context(), email.CreateEmailAddressInput{
		DomainID:         domainID,
		LocalPart:        req.LocalPart,
		IsCatchAll:       req.IsCatchAll,
		ForwardAddresses: req.ForwardAddresses,
	})
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	result := h.mapEmailAddressToResult(emailAddress, "")
	api.CreatedResponse(w, r, result)
}

// GetEmailAddresses gets all email addresses for a domain
// @Summary Get domain email addresses
// @Description Retrieve all email addresses configured for a specific domain
// @Tags Emails
// @Produce json
// @Security BearerAuth
// @Param domainId path string true "Domain ID" format(uuid)
// @Success 200 {array} EmailAddressResult "List of email addresses"
// @Failure 400 {object} models.ErrorResponseBody "Bad request"
// @Failure 401 {object} models.ErrorResponseBody "Unauthorized"
// @Failure 500 {object} models.ErrorResponseBody "Internal server error"
// @Router /domains/{domainId}/emails [get]
func (h *EmailsHandlers) GetEmailAddresses(w http.ResponseWriter, r *http.Request) {
	domainIDStr := chi.URLParam(r, "domainId")
	domainID, err := api.ParseUUID(domainIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	emailAddresses, err := h.emailUseCase.GetEmailAddressesByDomainID(r.Context(), domainID)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	results := make([]*EmailAddressResult, len(emailAddresses))
	for i, emailAddress := range emailAddresses {
		results[i] = h.mapEmailAddressToResult(emailAddress, "")
	}

	api.SuccessResponse(w, r, results)
}

// GetEmailAddress gets a specific email address by ID
// @Summary Get email address by ID
// @Description Retrieve a specific email address by its ID
// @Tags Emails
// @Produce json
// @Security BearerAuth
// @Param domainId path string true "Domain ID" format(uuid)
// @Param emailId path string true "Email Address ID" format(uuid)
// @Success 200 {object} EmailAddressResult "Email address details"
// @Failure 400 {object} models.ErrorResponseBody "Bad request"
// @Failure 401 {object} models.ErrorResponseBody "Unauthorized"
// @Failure 404 {object} models.ErrorResponseBody "Email address not found"
// @Router /domains/{domainId}/emails/{emailId} [get]
func (h *EmailsHandlers) GetEmailAddress(w http.ResponseWriter, r *http.Request) {
	emailIDStr := chi.URLParam(r, "emailId")
	emailID, err := api.ParseUUID(emailIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	emailAddress, err := h.emailUseCase.GetEmailAddressByID(r.Context(), emailID)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusNotFound, err)
		return
	}

	result := h.mapEmailAddressToResult(emailAddress, "")
	api.SuccessResponse(w, r, result)
}

// UpdateEmailAddress updates an existing email address
// @Summary Update email address
// @Description Update email address settings such as catch-all and forwarding configuration
// @Tags Emails
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param domainId path string true "Domain ID" format(uuid)
// @Param emailId path string true "Email Address ID" format(uuid)
// @Param request body UpdateEmailRequest true "Email address update details"
// @Success 200 {object} EmailAddressResult "Email address updated successfully"
// @Failure 400 {object} models.ErrorResponseBody "Bad request"
// @Failure 401 {object} models.ErrorResponseBody "Unauthorized"
// @Router /domains/{domainId}/emails/{emailId} [put]
func (h *EmailsHandlers) UpdateEmailAddress(w http.ResponseWriter, r *http.Request) {
	emailIDStr := chi.URLParam(r, "emailId")
	emailID, err := api.ParseUUID(emailIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	var req UpdateEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Update email address
	emailAddress, err := h.emailUseCase.UpdateEmailAddress(r.Context(), emailID, email.UpdateEmailAddressInput{
		IsCatchAll:       req.IsCatchAll,
		ForwardAddresses: req.ForwardAddresses,
	})
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	result := h.mapEmailAddressToResult(emailAddress, "")
	api.SuccessResponse(w, r, result)
}

// DeleteEmailAddress deletes an email address
// @Summary Delete email address
// @Description Delete an email address and all associated received emails
// @Tags Emails
// @Security BearerAuth
// @Param domainId path string true "Domain ID" format(uuid)
// @Param emailId path string true "Email Address ID" format(uuid)
// @Success 204 "Email address deleted successfully"
// @Failure 400 {object} models.ErrorResponseBody "Bad request"
// @Failure 401 {object} models.ErrorResponseBody "Unauthorized"
// @Router /domains/{domainId}/emails/{emailId} [delete]
func (h *EmailsHandlers) DeleteEmailAddress(w http.ResponseWriter, r *http.Request) {
	emailIDStr := chi.URLParam(r, "emailId")
	emailID, err := api.ParseUUID(emailIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	err = h.emailUseCase.DeleteEmailAddress(r.Context(), emailID)
	if err != nil {
		slog.Error("failed to delete email address", "error", err, "email_id", emailID)
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	api.NoContentResponse(w, r)
}

// GetReceivedEmails gets received emails for an email address
// @Summary Get received emails
// @Description Retrieve received emails for a specific email address with pagination
// @Tags Emails
// @Produce json
// @Security BearerAuth
// @Param domainId path string true "Domain ID" format(uuid)
// @Param emailId path string true "Email Address ID" format(uuid)
// @Param limit query int false "Number of emails per page" default(10)
// @Param offset query int false "Number of emails to skip" default(0)
// @Success 200 {object} models.PaginatedResponse "Paginated list of received emails"
// @Failure 400 {object} models.ErrorResponseBody "Bad request"
// @Failure 401 {object} models.ErrorResponseBody "Unauthorized"
// @Failure 500 {object} models.ErrorResponseBody "Internal server error"
// @Router /domains/{domainId}/emails/{emailId}/received [get]
func (h *EmailsHandlers) GetReceivedEmails(w http.ResponseWriter, r *http.Request) {
	emailIDStr := chi.URLParam(r, "emailId")
	emailID, err := api.ParseUUID(emailIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	pagination := api.GetPaginationParams(r)

	receivedEmails, err := h.emailUseCase.GetReceivedEmails(r.Context(), emailID, pagination.Limit, pagination.Offset)
	if err != nil {
		slog.Error("failed to get received emails", "error", err, "email_id", emailID)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	results := make([]*ReceivedEmailResult, len(receivedEmails))
	for i, receivedEmail := range receivedEmails {
		results[i] = h.mapReceivedEmailToResult(receivedEmail)
	}

	response := api.PaginatedResponse{
		Data: results,
		Pagination: struct {
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
			Total  int `json:"total,omitempty"`
		}{
			Limit:  pagination.Limit,
			Offset: pagination.Offset,
		},
	}

	api.SuccessResponse(w, r, response)
}

// GetReceivedEmail gets a specific received email by ID
// @Summary Get received email by ID
// @Description Retrieve a specific received email by its ID (must belong to authenticated user)
// @Tags Emails
// @Produce json
// @Security BearerAuth
// @Param receivedEmailId path string true "Received Email ID" format(uuid)
// @Success 200 {object} ReceivedEmailResult "Received email details"
// @Failure 400 {object} models.ErrorResponseBody "Bad request"
// @Failure 401 {object} models.ErrorResponseBody "Unauthorized"
// @Failure 404 {object} models.ErrorResponseBody "Received email not found"
// @Router /received/{receivedEmailId} [get]
func (h *EmailsHandlers) GetReceivedEmail(w http.ResponseWriter, r *http.Request) {
	receivedEmailIDStr := chi.URLParam(r, "receivedEmailId")
	receivedEmailID, err := api.ParseUUID(receivedEmailIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Get user ID from context
	userID, err := api.GetUserIDFromContext(r)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusUnauthorized, err)
		return
	}

	receivedEmail, err := h.emailUseCase.GetReceivedEmailByID(r.Context(), receivedEmailID, userID)
	if err != nil {
		slog.Error("failed to get received email", "error", err, "received_email_id", receivedEmailID)
		api.ErrorResponse(w, r, http.StatusNotFound, err)
		return
	}

	result := h.mapReceivedEmailToResult(receivedEmail)
	api.SuccessResponse(w, r, result)
}

// DeleteReceivedEmail deletes a specific received email by ID
// @Summary Delete received email by ID
// @Description Delete a specific received email by its ID (must belong to authenticated user)
// @Tags Emails
// @Security BearerAuth
// @Param receivedEmailId path string true "Received Email ID" format(uuid)
// @Success 204 "Received email deleted successfully"
// @Failure 400 {object} models.ErrorResponseBody "Bad request"
// @Failure 401 {object} models.ErrorResponseBody "Unauthorized"
// @Failure 403 {object} models.ErrorResponseBody "Forbidden"
// @Failure 404 {object} models.ErrorResponseBody "Received email not found"
// @Router /received/{receivedEmailId} [delete]
func (h *EmailsHandlers) DeleteReceivedEmail(w http.ResponseWriter, r *http.Request) {
	receivedEmailIDStr := chi.URLParam(r, "receivedEmailId")
	receivedEmailID, err := api.ParseUUID(receivedEmailIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Get user ID from context
	userID, err := api.GetUserIDFromContext(r)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusUnauthorized, err)
		return
	}

	if err := h.emailUseCase.DeleteReceivedEmail(r.Context(), receivedEmailID, userID); err != nil {
		slog.Error("failed to delete received email", "error", err, "received_email_id", receivedEmailID)
		// We don't differentiate error types here; report as BadRequest to avoid leaking details
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	api.NoContentResponse(w, r)
}

// mapEmailAddressToResult converts email address entity to API result
func (h *EmailsHandlers) mapEmailAddressToResult(emailAddress *entities.EmailAddress, domain string) *EmailAddressResult {
	fullAddress := emailAddress.LocalPart + "@" + domain
	if domain == "" {
		fullAddress = emailAddress.LocalPart + "@<domain>"
	}

	return &EmailAddressResult{
		ID:               emailAddress.ID.String(),
		DomainID:         emailAddress.DomainID.String(),
		LocalPart:        emailAddress.LocalPart,
		FullAddress:      fullAddress,
		IsCatchAll:       emailAddress.IsCatchAll,
		ForwardAddresses: emailAddress.ForwardAddresses,
		CreatedAt:        emailAddress.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:        emailAddress.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// mapReceivedEmailToResult converts received email entity to API result
func (h *EmailsHandlers) mapReceivedEmailToResult(receivedEmail *entities.ReceivedEmail) *ReceivedEmailResult {
	subject := ""
	if receivedEmail.Subject != nil {
		subject = *receivedEmail.Subject
	}

	return &ReceivedEmailResult{
		ID:             receivedEmail.ID.String(),
		SequenceNumber: receivedEmail.SequenceNumber,
		FromAddress:    receivedEmail.FromAddress,
		Subject:        subject,
		EncryptedBody:  receivedEmail.EncryptedBody,
		ReceivedAt:     receivedEmail.ReceivedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

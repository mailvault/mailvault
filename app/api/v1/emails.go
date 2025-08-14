package v1

import (
	"context"
	"encoding/json"
	"net/http"

	"mailvault/domain/email"
	"mailvault/domain/entities"

	"github.com/go-chi/chi/v5"
	"github.com/gofrs/uuid/v5"
)

//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/email_usecase.go . EmailUseCase

// EmailUseCase defines the behavior required by this package from the email use case.
type EmailUseCase interface {
	CreateEmailAddressFromInput(ctx context.Context, req email.CreateEmailAddressInput) (*entities.EmailAddress, error)
	GetEmailAddressesByDomainID(ctx context.Context, domainID uuid.UUID) ([]*entities.EmailAddress, error)
	GetEmailAddressByID(ctx context.Context, id uuid.UUID) (*entities.EmailAddress, error)
	UpdateEmailAddress(ctx context.Context, id uuid.UUID, req email.UpdateEmailAddressInput) (*entities.EmailAddress, error)
	DeleteEmailAddress(ctx context.Context, id uuid.UUID) error
	GetReceivedEmails(ctx context.Context, emailID uuid.UUID, limit, offset int) ([]*entities.ReceivedEmail, error)
	GetReceivedEmailByID(ctx context.Context, receivedEmailID uuid.UUID, userID uuid.UUID) (*entities.ReceivedEmail, error)
}

// EmailsHandlers contains email-related endpoints
type EmailsHandlers struct {
	emailUseCase EmailUseCase
}

func NewEmailsHandlers(emailUseCase EmailUseCase) *EmailsHandlers {
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
func (h *EmailsHandlers) CreateEmailAddress(w http.ResponseWriter, r *http.Request) {
	domainIDStr := chi.URLParam(r, "domainId")
	domainID, err := parseUUID(domainIDStr)
	if err != nil {
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	var req CreateEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, r, http.StatusBadRequest, err)
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
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	result := h.mapEmailAddressToResult(emailAddress, "")
	createdResponse(w, r, result)
}

// GetEmailAddresses gets all email addresses for a domain
func (h *EmailsHandlers) GetEmailAddresses(w http.ResponseWriter, r *http.Request) {
	domainIDStr := chi.URLParam(r, "domainId")
	domainID, err := parseUUID(domainIDStr)
	if err != nil {
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	emailAddresses, err := h.emailUseCase.GetEmailAddressesByDomainID(r.Context(), domainID)
	if err != nil {
		errorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	results := make([]*EmailAddressResult, len(emailAddresses))
	for i, emailAddress := range emailAddresses {
		results[i] = h.mapEmailAddressToResult(emailAddress, "")
	}

	successResponse(w, r, results)
}

// GetEmailAddress gets a specific email address by ID
func (h *EmailsHandlers) GetEmailAddress(w http.ResponseWriter, r *http.Request) {
	emailIDStr := chi.URLParam(r, "emailId")
	emailID, err := parseUUID(emailIDStr)
	if err != nil {
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	emailAddress, err := h.emailUseCase.GetEmailAddressByID(r.Context(), emailID)
	if err != nil {
		errorResponse(w, r, http.StatusNotFound, err)
		return
	}

	result := h.mapEmailAddressToResult(emailAddress, "")
	successResponse(w, r, result)
}

// UpdateEmailAddress updates an existing email address
func (h *EmailsHandlers) UpdateEmailAddress(w http.ResponseWriter, r *http.Request) {
	emailIDStr := chi.URLParam(r, "emailId")
	emailID, err := parseUUID(emailIDStr)
	if err != nil {
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	var req UpdateEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Update email address
	emailAddress, err := h.emailUseCase.UpdateEmailAddress(r.Context(), emailID, email.UpdateEmailAddressInput{
		IsCatchAll:       req.IsCatchAll,
		ForwardAddresses: req.ForwardAddresses,
	})
	if err != nil {
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	result := h.mapEmailAddressToResult(emailAddress, "")
	successResponse(w, r, result)
}

// DeleteEmailAddress deletes an email address
func (h *EmailsHandlers) DeleteEmailAddress(w http.ResponseWriter, r *http.Request) {
	emailIDStr := chi.URLParam(r, "emailId")
	emailID, err := parseUUID(emailIDStr)
	if err != nil {
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	err = h.emailUseCase.DeleteEmailAddress(r.Context(), emailID)
	if err != nil {
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	noContentResponse(w, r)
}

// GetReceivedEmails gets received emails for an email address
func (h *EmailsHandlers) GetReceivedEmails(w http.ResponseWriter, r *http.Request) {
	emailIDStr := chi.URLParam(r, "emailId")
	emailID, err := parseUUID(emailIDStr)
	if err != nil {
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	pagination := getPaginationParams(r)

	receivedEmails, err := h.emailUseCase.GetReceivedEmails(r.Context(), emailID, pagination.Limit, pagination.Offset)
	if err != nil {
		errorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	results := make([]*ReceivedEmailResult, len(receivedEmails))
	for i, receivedEmail := range receivedEmails {
		results[i] = h.mapReceivedEmailToResult(receivedEmail)
	}

	response := PaginatedResponse{
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

	successResponse(w, r, response)
}

// GetReceivedEmail gets a specific received email by ID
func (h *EmailsHandlers) GetReceivedEmail(w http.ResponseWriter, r *http.Request) {
	receivedEmailIDStr := chi.URLParam(r, "receivedEmailId")
	receivedEmailID, err := parseUUID(receivedEmailIDStr)
	if err != nil {
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Get user ID from context
	userID, err := getUserIDFromContext(r)
	if err != nil {
		errorResponse(w, r, http.StatusUnauthorized, err)
		return
	}

	receivedEmail, err := h.emailUseCase.GetReceivedEmailByID(r.Context(), receivedEmailID, userID)
	if err != nil {
		errorResponse(w, r, http.StatusNotFound, err)
		return
	}

	result := h.mapReceivedEmailToResult(receivedEmail)
	successResponse(w, r, result)
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

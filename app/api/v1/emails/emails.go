package emails

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/mailvault/mailvault/app/api"
	"github.com/mailvault/mailvault/domain/email"
	"github.com/mailvault/mailvault/domain/entities"
	"github.com/mailvault/mailvault/internal/emailrender"

	"github.com/go-chi/chi/v5"
	"github.com/gofrs/uuid/v5"
)

//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/usecase.go . UseCase

// UseCase defines the behavior required by this package from the email use case.
type UseCase interface {
	CreateEmailAddressFromInput(ctx context.Context, req email.CreateEmailAddressInput) (*entities.EmailAddress, error)
	GetEmailAddressesByDomainID(ctx context.Context, domainID uuid.UUID) ([]*entities.EmailAddress, error)
	GetEmailAddressByID(ctx context.Context, id uuid.UUID) (*entities.EmailAddress, error)
	GetEmailAddressByAddress(ctx context.Context, address string) (*entities.EmailAddress, error)
	UpdateEmailAddress(ctx context.Context, id uuid.UUID, req email.UpdateEmailAddressInput) (*entities.EmailAddress, error)
	DeleteEmailAddress(ctx context.Context, id uuid.UUID) error
	GetReceivedEmails(ctx context.Context, emailID uuid.UUID, limit, offset int) ([]*entities.ReceivedEmail, error)
	GetReceivedEmailByID(ctx context.Context, receivedEmailID uuid.UUID, userID uuid.UUID) (*entities.ReceivedEmail, error)
	DeleteReceivedEmail(ctx context.Context, receivedEmailID uuid.UUID, userID uuid.UUID) error
	GetReceivedEmailsByUser(ctx context.Context, userID uuid.UUID, limit, offset int, filter email.GetReceivedEmailsFilter) ([]*entities.ReceivedEmail, int, error)
	GetDomainByID(ctx context.Context, domainID uuid.UUID) (*entities.Domain, error)
}

// EmailsHandlers contains email-related endpoints
type EmailsHandlers struct {
	emailUseCase UseCase
	renderEngine *emailrender.RenderEngine
}

func NewEmailsHandlers(emailUseCase UseCase) *EmailsHandlers {
	return &EmailsHandlers{
		emailUseCase: emailUseCase,
		renderEngine: emailrender.NewRenderEngine(),
	}
}

// CreateEmailRequest represents email address creation request
type CreateEmailRequest struct {
	LocalPart         string   `json:"local_part" validate:"required"`
	ForwardAddresses  []string `json:"forward_addresses,omitempty"`
	ForwardingEnabled bool     `json:"forwarding_enabled"`
}

// UpdateEmailRequest represents email address update request
type UpdateEmailRequest struct {
	ForwardAddresses  []string `json:"forward_addresses,omitempty"`
	ForwardingEnabled *bool    `json:"forwarding_enabled,omitempty"`
}

// EmailAddressResult represents email address data in responses
type EmailAddressResult struct {
	ID                string   `json:"id"`
	DomainID          string   `json:"domain_id"`
	LocalPart         string   `json:"local_part"`
	FullAddress       string   `json:"full_address"`
	ForwardAddresses  []string `json:"forward_addresses"`
	ForwardingEnabled bool     `json:"forwarding_enabled"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
}

// ReceivedEmailResult represents received email data in responses
type ReceivedEmailResult struct {
	ID               string                  `json:"id"`
	SequenceNumber   int                     `json:"sequence_number"`
	FromAddress      string                  `json:"from_address"`
	Subject          string                  `json:"subject"`
	EncryptedBody    string                  `json:"encrypted_body"`
	DomainName       string                  `json:"domain_name"`
	EmailAddress     string                  `json:"email_address"`
	ReceivedAt       string                  `json:"received_at"`
	SMTPVerification *SMTPVerificationResult `json:"smtp_verification,omitempty"`
}

type SMTPVerificationResult struct {
	VerifiedAt         string  `json:"verified_at"`
	SenderIP           string  `json:"sender_ip"`
	SenderDomain       string  `json:"sender_domain"`
	SPFResult          string  `json:"spf_result"`
	SPFMechanism       string  `json:"spf_mechanism"`
	DKIMValid          bool    `json:"dkim_valid"`
	DKIMDomain         string  `json:"dkim_domain"`
	DKIMSelector       string  `json:"dkim_selector"`
	DMARCResult        string  `json:"dmarc_result"`
	DMARCPolicy        string  `json:"dmarc_policy"`
	DMARCAlignmentSPF  bool    `json:"dmarc_alignment_spf"`
	DMARCAlignmentDKIM bool    `json:"dmarc_alignment_dkim"`
	SpamScore          float64 `json:"spam_score"`
	ContentVerdict     string  `json:"content_verdict"`
	ReputationScore    float64 `json:"reputation_score"`
	IsBlacklisted      bool    `json:"is_blacklisted"`
	FinalAction        string  `json:"final_action"`
	IsQuarantined      bool    `json:"is_quarantined"`
}

// ParsedEmailResult represents parsed email content in API responses
type ParsedEmailResult struct {
	ID               string                     `json:"id"`
	SequenceNumber   int                        `json:"sequence_number"`
	FromAddress      string                     `json:"from_address"`
	Subject          string                     `json:"subject"`
	DomainName       string                     `json:"domain_name"`
	EmailAddress     string                     `json:"email_address"`
	ReceivedAt       string                     `json:"received_at"`
	ParsedContent    *emailrender.EmailResponse `json:"parsed_content,omitempty"`
	AvailableFormats []string                   `json:"available_formats,omitempty"`
	RenderingError   string                     `json:"rendering_error,omitempty"`
}

// CreateEmailAddress creates a new email address for a domain
// @Summary Create email address
// @Description Create a new email address for a specific domain with optional forwarding settings
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
		slog.Error("failed to decode create email request", "error", err)
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Create email address
	emailAddress, err := h.emailUseCase.CreateEmailAddressFromInput(r.Context(), email.CreateEmailAddressInput{
		DomainID:          domainID,
		LocalPart:         req.LocalPart,
		ForwardAddresses:  req.ForwardAddresses,
		ForwardingEnabled: req.ForwardingEnabled,
	})
	if err != nil {
		slog.Error("failed to create email address", "error", err, "domain_id", domainID, "local_part", req.LocalPart)
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
// @Description Update email address settings such as forwarding configuration
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
		ForwardAddresses:  req.ForwardAddresses,
		ForwardingEnabled: req.ForwardingEnabled,
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

// ListReceivedEmailsForUser lists all received emails for a user across all domains
// @Summary List user's received emails
// @Description Get a list of all received emails for the authenticated user across all domains with advanced filtering
// @Tags Emails
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Maximum number of emails to return" default(50)
// @Param offset query int false "Number of emails to skip" default(0)
// @Param domain query string false "Filter by domain"
// @Param sort_by query string false "Sort field: received_at, sequence_number, from_address, subject" default(received_at)
// @Param sort_order query string false "Sort order: asc, desc" default(desc)
// @Param email_address query string false "Filter by recipient email address"
// @Param from_address query string false "Filter by sender email address"
// @Param date_from query string false "Filter from date (YYYY-MM-DD)"
// @Param date_to query string false "Filter to date (YYYY-MM-DD)"
// @Param spam_min query number false "Minimum spam score (0-1)"
// @Param spam_max query number false "Maximum spam score (0-1)"
// @Param security_status query string false "Security status: clean, suspicious, quarantined, high_risk"
// @Param search query string false "Full-text search in subject and from address"
// @Success 200 {object} models.PaginatedResponse "List of received emails"
// @Failure 400 {object} models.ErrorResponseBody "Bad request"
// @Failure 401 {object} models.ErrorResponseBody "Unauthorized"
// @Router /emails/received [get]
func (h *EmailsHandlers) ListReceivedEmailsForUser(w http.ResponseWriter, r *http.Request) {
	userID, err := api.GetUserIDFromContext(r)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusUnauthorized, err)
		return
	}

	// Parse query parameters
	pagination := api.GetPaginationParams(r)

	// Build filter from query parameters
	filter := email.GetReceivedEmailsFilter{
		Domain:         r.URL.Query().Get("domain"),
		SortBy:         r.URL.Query().Get("sort_by"),
		SortOrder:      r.URL.Query().Get("sort_order"),
		EmailAddress:   r.URL.Query().Get("email_address"),
		FromAddress:    r.URL.Query().Get("from_address"),
		DateFrom:       r.URL.Query().Get("date_from"),
		DateTo:         r.URL.Query().Get("date_to"),
		SecurityStatus: r.URL.Query().Get("security_status"),
		Search:         r.URL.Query().Get("search"),
	}

	// Parse spam score filters
	if spamMinStr := r.URL.Query().Get("spam_min"); spamMinStr != "" {
		if spamMin, err := strconv.ParseFloat(spamMinStr, 64); err == nil {
			filter.SpamMin = spamMin
		}
	}

	if spamMaxStr := r.URL.Query().Get("spam_max"); spamMaxStr != "" {
		if spamMax, err := strconv.ParseFloat(spamMaxStr, 64); err == nil {
			filter.SpamMax = spamMax
		}
	}

	// Get received emails with filter
	emails, total, err := h.emailUseCase.GetReceivedEmailsByUser(r.Context(), userID, pagination.Limit, pagination.Offset, filter)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Convert to API results
	results := make([]*ReceivedEmailResult, len(emails))
	for i, email := range emails {
		results[i] = h.mapReceivedEmailToResult(email)
	}

	// Create paginated response
	response := api.PaginatedResponse{
		Data: results,
		Pagination: struct {
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
			Total  int `json:"total,omitempty"`
		}{
			Limit:  pagination.Limit,
			Offset: pagination.Offset,
			Total:  total,
		},
	}

	api.SuccessResponse(w, r, response)
}

// GetParsedReceivedEmail gets a specific received email with parsed content
// @Summary Get parsed received email by ID
// @Description Retrieve a specific received email with parsed content in multiple formats
// @Tags Emails
// @Produce json
// @Security BearerAuth
// @Param receivedEmailId path string true "Received Email ID" format(uuid)
// @Param private_key query string false "Base64 encoded private key for decryption"
// @Param format query string false "Content format preference: auto, plain, html, markdown, raw" default(auto)
// @Success 200 {object} ParsedEmailResult "Parsed received email details"
// @Failure 400 {object} models.ErrorResponseBody "Bad request"
// @Failure 401 {object} models.ErrorResponseBody "Unauthorized"
// @Failure 404 {object} models.ErrorResponseBody "Received email not found"
// @Router /received/{receivedEmailId}/parsed [get]
func (h *EmailsHandlers) GetParsedReceivedEmail(w http.ResponseWriter, r *http.Request) {
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

	// Get received email
	receivedEmail, err := h.emailUseCase.GetReceivedEmailByID(r.Context(), receivedEmailID, userID)
	if err != nil {
		slog.Error("failed to get received email", "error", err, "received_email_id", receivedEmailID)
		api.ErrorResponse(w, r, http.StatusNotFound, err)
		return
	}

	// Create base result
	result := h.mapReceivedEmailToParsedResult(receivedEmail)

	// Check if private key is provided for parsing
	privateKeyBase64 := r.URL.Query().Get("private_key")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "auto"
	}
	_ = format // format parameter used for future enhancements

	if privateKeyBase64 != "" {
		// Parse encrypted email content
		parsed, err := h.renderEngine.ParseFromEncrypted(receivedEmail.EncryptedBody, privateKeyBase64)
		if err != nil {
			result.RenderingError = fmt.Sprintf("failed to parse email: %v", err)
		} else {
			// Create email response
			response := h.renderEngine.CreateEmailResponse(receivedEmail, parsed)
			result.ParsedContent = response
			result.AvailableFormats = parsed.GetAvailableFormats()
		}
	} else {
		result.RenderingError = "Private key required for content parsing. Provide 'private_key' query parameter."
	}

	api.SuccessResponse(w, r, result)
}

// mapEmailAddressToResult converts email address entity to API result
func (h *EmailsHandlers) mapEmailAddressToResult(emailAddress *entities.EmailAddress, domain string) *EmailAddressResult {
	fullAddress := emailAddress.LocalPart + "@" + domain
	if domain == "" {
		fullAddress = emailAddress.LocalPart + "@<domain>"
	}

	return &EmailAddressResult{
		ID:                emailAddress.ID.String(),
		DomainID:          emailAddress.DomainID.String(),
		LocalPart:         emailAddress.LocalPart,
		FullAddress:       fullAddress,
		ForwardAddresses:  emailAddress.ForwardAddresses,
		ForwardingEnabled: emailAddress.ForwardingEnabled,
		CreatedAt:         emailAddress.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:         emailAddress.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// mapReceivedEmailToResult converts received email entity to API result
func (h *EmailsHandlers) mapReceivedEmailToResult(receivedEmail *entities.ReceivedEmail) *ReceivedEmailResult {
	subject := ""
	if receivedEmail.Subject != nil {
		subject = *receivedEmail.Subject
	}

	return &ReceivedEmailResult{
		ID:               receivedEmail.ID.String(),
		SequenceNumber:   receivedEmail.SequenceNumber,
		FromAddress:      receivedEmail.FromAddress,
		Subject:          subject,
		EncryptedBody:    receivedEmail.EncryptedBody,
		DomainName:       receivedEmail.DomainName,
		EmailAddress:     receivedEmail.EmailAddress,
		ReceivedAt:       receivedEmail.ReceivedAt.Format("2006-01-02T15:04:05Z07:00"),
		SMTPVerification: h.mapSMTPVerificationToResult(receivedEmail.SMTPVerification),
	}
}

// mapSMTPVerificationToResult converts SMTP verification entity to API result
func (h *EmailsHandlers) mapSMTPVerificationToResult(smtpVerification *entities.SMTPVerificationStat) *SMTPVerificationResult {
	if smtpVerification == nil {
		return nil
	}
	return &SMTPVerificationResult{
		VerifiedAt:         smtpVerification.VerifiedAt.Format("2006-01-02T15:04:05Z07:00"),
		SenderIP:           smtpVerification.SenderIP.String(),
		SenderDomain:       smtpVerification.SenderDomain,
		SPFResult:          smtpVerification.SPFResult,
		SPFMechanism:       smtpVerification.SPFMechanism,
		DKIMValid:          smtpVerification.DKIMValid,
		DKIMDomain:         smtpVerification.DKIMDomain,
		DKIMSelector:       smtpVerification.DKIMSelector,
		DMARCResult:        smtpVerification.DMARCResult,
		DMARCPolicy:        smtpVerification.DMARCPolicy,
		DMARCAlignmentSPF:  smtpVerification.DMARCAlignmentSPF,
		DMARCAlignmentDKIM: smtpVerification.DMARCAlignmentDKIM,
		SpamScore:          smtpVerification.SpamScore,
		ContentVerdict:     smtpVerification.ContentVerdict,
		ReputationScore:    smtpVerification.ReputationScore,
		IsBlacklisted:      smtpVerification.IsBlacklisted,
		FinalAction:        smtpVerification.FinalAction,
		IsQuarantined:      smtpVerification.IsQuarantined,
	}
}

// mapReceivedEmailToParsedResult converts received email entity to parsed API result
func (h *EmailsHandlers) mapReceivedEmailToParsedResult(receivedEmail *entities.ReceivedEmail) *ParsedEmailResult {
	subject := ""
	if receivedEmail.Subject != nil {
		subject = *receivedEmail.Subject
	}

	return &ParsedEmailResult{
		ID:             receivedEmail.ID.String(),
		SequenceNumber: receivedEmail.SequenceNumber,
		FromAddress:    receivedEmail.FromAddress,
		Subject:        subject,
		DomainName:     receivedEmail.DomainName,
		EmailAddress:   receivedEmail.EmailAddress,
		ReceivedAt:     receivedEmail.ReceivedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

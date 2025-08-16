package send

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"mailvault/app/api"
	"mailvault/domain/entities"

	"github.com/go-chi/render"
)

//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/usecase.go . UseCase

// UseCase defines the behavior required by this package from the send use case.
type UseCase interface {
	GetDomainByAPIKey(ctx context.Context, apiKey string) (*entities.Domain, error)
}

// SendHandlers contains email sending endpoints
type SendHandlers struct {
	sendUseCase UseCase
}

func NewSendHandlers(sendUseCase UseCase) *SendHandlers {
	return &SendHandlers{
		sendUseCase: sendUseCase,
	}
}

// SendEmailRequest represents email sending request
type SendEmailRequest struct {
	From     string   `json:"from" validate:"required,email"`
	To       []string `json:"to" validate:"required,min=1"`
	CC       []string `json:"cc,omitempty"`
	BCC      []string `json:"bcc,omitempty"`
	Subject  string   `json:"subject" validate:"required"`
	TextBody string   `json:"text_body,omitempty"`
	HTMLBody string   `json:"html_body,omitempty"`
}

// SendEmailResponse represents email sending response
type SendEmailResponse struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
}

// SendEmail sends an email using the domain's API key
// @Summary Send email
// @Description Send an email using domain API key authentication. The from address must belong to the authenticated domain.
// @Tags Email Sending
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body SendEmailRequest true "Email sending details"
// @Success 202 {object} SendEmailResponse "Email queued for delivery"
// @Failure 400 {object} ErrorResponseBody "Bad request - invalid email data"
// @Failure 401 {object} ErrorResponseBody "Unauthorized - invalid or missing API key"
// @Router /send [post]
func (h *SendHandlers) SendEmail(w http.ResponseWriter, r *http.Request) {
	// Extract API key from header
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		// Also check Authorization header with Bearer prefix
		authHeader := r.Header.Get("Authorization")
		if after, ok := strings.CutPrefix(authHeader, "Bearer "); ok {
			apiKey = after
		}
	}

	if apiKey == "" {
		api.ErrorResponse(w, r, http.StatusUnauthorized, api.ErrUnauthorized)
		return
	}

	// Validate API key and get domain
	domain, err := h.sendUseCase.GetDomainByAPIKey(r.Context(), apiKey)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusUnauthorized, err)
		return
	}

	var req SendEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Validate that 'from' address belongs to the domain
	if !h.isFromAddressValid(req.From, domain.Domain) {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, api.ErrorResponseBody{
			Error: "from address must belong to the authenticated domain",
		})
		return
	}

	// Validate that we have at least text or HTML body
	if req.TextBody == "" && req.HTMLBody == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, api.ErrorResponseBody{
			Error: "either text_body or html_body is required",
		})
		return
	}

	// Note: Actual email sending would be implemented here
	// This would typically queue the email for processing by an SMTP service

	// Generate a proper message ID
	messageID := generateMessageID()

	response := SendEmailResponse{
		MessageID: messageID,
		Status:    "accepted",
	}

	render.Status(r, http.StatusAccepted)
	render.JSON(w, r, response)
}

// isFromAddressValid checks if the from address belongs to the domain
func (h *SendHandlers) isFromAddressValid(fromAddress, domainName string) bool {
	parts := strings.Split(fromAddress, "@")
	if len(parts) != 2 {
		return false
	}

	emailDomain := strings.ToLower(parts[1])
	return emailDomain == strings.ToLower(domainName)
}

// generateMessageID generates a unique message ID
func generateMessageID() string {
	// Generate timestamp prefix
	timestamp := time.Now().Unix()

	// Generate random bytes
	bytes := make([]byte, 8)
	rand.Read(bytes)
	randomHex := hex.EncodeToString(bytes)

	// Format: mv_<timestamp>_<random>
	return "mv_" + strings.ToLower(hex.EncodeToString([]byte{byte(timestamp)})) + "_" + randomHex
}

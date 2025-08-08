package v1

import (
	"encoding/json"
	"net/http"
	"strings"

	domainpkg "mailsafe/domain/domain"

	"github.com/go-chi/render"
)

// SendHandlers contains email sending endpoints
type SendHandlers struct {
	domainUseCase *domainpkg.UseCase
}

func NewSendHandlers(domainUseCase *domainpkg.UseCase) *SendHandlers {
	return &SendHandlers{
		domainUseCase: domainUseCase,
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
		errorResponse(w, r, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	// Validate API key and get domain
	domain, err := h.domainUseCase.GetDomainByAPIKey(r.Context(), apiKey)
	if err != nil {
		errorResponse(w, r, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	var req SendEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Validate that 'from' address belongs to the domain
	if !h.isFromAddressValid(req.From, domain.Domain) {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrorResponseBody{
			Error: "from address must belong to the authenticated domain",
		})
		return
	}

	// Validate that we have at least text or HTML body
	if req.TextBody == "" && req.HTMLBody == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrorResponseBody{
			Error: "either text_body or html_body is required",
		})
		return
	}

	// TODO: Implement actual email sending logic
	// For now, we'll return a mock response

	// Generate a mock message ID
	messageID := "pm_" + generateMessageID()

	response := SendEmailResponse{
		MessageID: messageID,
		Status:    "queued",
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
	// TODO: Implement proper message ID generation
	// For now, return a placeholder
	return "1234567890abcdef"
}

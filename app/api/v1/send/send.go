package send

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"mailvault/app/api"
	billingdomain "mailvault/domain/billing"
	"mailvault/domain/entities"
	"mailvault/internal/utils"

	"github.com/go-chi/render"
	"github.com/gofrs/uuid/v5"
)

//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/usecase.go . UseCase

// UseCase defines the behavior required by this package from the send use case.
type UseCase interface {
	GetDomainByAPIKey(ctx context.Context, apiKey string) (*entities.Domain, error)
}

// BillingUseCase defines the billing operations required by send handlers.
type BillingUseCase interface {
	CheckLimit(ctx context.Context, userID uuid.UUID, metric entities.UsageMetric) (*billingdomain.CheckLimitResult, error)
	IncrementUsage(ctx context.Context, userID uuid.UUID, metric entities.UsageMetric, amount int64) error
}

// SendHandlers contains email sending endpoints
type SendHandlers struct {
	sendUseCase    UseCase
	billingUseCase BillingUseCase
	logger         *slog.Logger
}

func NewSendHandlers(sendUseCase UseCase, billingUseCase BillingUseCase, logger *slog.Logger) *SendHandlers {
	return &SendHandlers{
		sendUseCase:    sendUseCase,
		billingUseCase: billingUseCase,
		logger:         logger,
	}
}

// planLimitExceededResponse is the 402 response body for plan limit violations.
type planLimitExceededResponse struct {
	Error      string `json:"error"`
	Message    string `json:"message"`
	Current    int64  `json:"current"`
	Limit      int    `json:"limit"`
	UpgradeURL string `json:"upgrade_url"`
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
// @Failure 400 {object} models.ErrorResponseBody "Bad request - invalid email data"
// @Failure 401 {object} models.ErrorResponseBody "Unauthorized - invalid or missing API key"
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

	// Check email send limit for the domain owner.
	limitResult, err := h.billingUseCase.CheckLimit(r.Context(), domain.UserID, entities.UsageMetricEmailsSent)
	if err != nil {
		h.logger.Error("failed to check email send limit", "user_id", domain.UserID, "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}
	if !limitResult.Allowed {
		render.Status(r, http.StatusPaymentRequired)
		render.JSON(w, r, planLimitExceededResponse{
			Error:      "plan_limit_exceeded",
			Message:    fmt.Sprintf("daily email send limit reached (%d/%d). upgrade your plan to send more emails", limitResult.Current, limitResult.Limit),
			Current:    limitResult.Current,
			Limit:      limitResult.Limit,
			UpgradeURL: "/billing",
		})
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

	// Increment emails_sent usage counter after successful acceptance.
	if err := h.billingUseCase.IncrementUsage(r.Context(), domain.UserID, entities.UsageMetricEmailsSent, 1); err != nil {
		h.logger.Error("failed to increment email send usage", "user_id", domain.UserID, "error", err)
		// Non-fatal: email was accepted, log and continue.
	}

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
	emailDomain, err := utils.ExtractDomain(fromAddress)
	if err != nil {
		return false
	}
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

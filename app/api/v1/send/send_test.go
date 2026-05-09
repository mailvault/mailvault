package send

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"mailvault/app/api/v1/send/mocks"
	billingdomain "mailvault/domain/billing"
	"mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
)

// noopBillingUseCase is a test double that always allows operations.
type noopBillingUseCase struct{}

func (n *noopBillingUseCase) CheckLimit(_ context.Context, _ uuid.UUID, _ entities.UsageMetric) (*billingdomain.CheckLimitResult, error) {
	return &billingdomain.CheckLimitResult{Allowed: true, Unlimited: true}, nil
}

func (n *noopBillingUseCase) IncrementUsage(_ context.Context, _ uuid.UUID, _ entities.UsageMetric, _ int64) error {
	return nil
}

func TestSendHandlers_SendEmail_Success(t *testing.T) {
	t.Parallel()

	mock := &mocks.UseCaseMock{
		GetDomainByAPIKeyFunc: func(ctx context.Context, apiKey string) (*entities.Domain, error) {
			assert.Equal(t, "pm_api_key", apiKey)
			return &entities.Domain{Domain: "example.com"}, nil
		},
	}
	h := NewSendHandlers(mock, &noopBillingUseCase{}, slog.Default())

	reqBody, _ := json.Marshal(SendEmailRequest{
		From:     "noreply@example.com",
		To:       []string{"user@dest.com"},
		Subject:  "Hello",
		TextBody: "Body",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/send", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer pm_api_key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendEmail(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp SendEmailResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "accepted", resp.Status)
	assert.NotEmpty(t, resp.MessageID)
}

func TestSendHandlers_SendEmail_MissingAPIKey(t *testing.T) {
	t.Parallel()

	h := NewSendHandlers(&mocks.UseCaseMock{}, &noopBillingUseCase{}, slog.Default())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/send", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendEmail(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSendHandlers_SendEmail_InvalidFromDomain(t *testing.T) {
	t.Parallel()

	mock := &mocks.UseCaseMock{GetDomainByAPIKeyFunc: func(ctx context.Context, apiKey string) (*entities.Domain, error) {
		return &entities.Domain{Domain: "example.com"}, nil
	}}
	h := NewSendHandlers(mock, &noopBillingUseCase{}, slog.Default())

	reqBody, _ := json.Marshal(SendEmailRequest{From: "noreply@other.com", To: []string{"x@y.com"}, Subject: "Hello", TextBody: "Body"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/send", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer pm_api_key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendEmail(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSendHandlers_SendEmail_BodyMissing(t *testing.T) {
	t.Parallel()

	mock := &mocks.UseCaseMock{GetDomainByAPIKeyFunc: func(ctx context.Context, apiKey string) (*entities.Domain, error) {
		return &entities.Domain{Domain: "example.com"}, nil
	}}
	h := NewSendHandlers(mock, &noopBillingUseCase{}, slog.Default())

	reqBody, _ := json.Marshal(SendEmailRequest{From: "noreply@example.com", To: []string{"x@y.com"}, Subject: "Hello"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/send", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer pm_api_key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendEmail(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSendHandlers_SendEmail_InvalidJSON(t *testing.T) {
	t.Parallel()

	mock := &mocks.UseCaseMock{GetDomainByAPIKeyFunc: func(ctx context.Context, apiKey string) (*entities.Domain, error) {
		return &entities.Domain{Domain: "example.com"}, nil
	}}
	h := NewSendHandlers(mock, &noopBillingUseCase{}, slog.Default())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/send", bytes.NewReader([]byte("{")))
	req.Header.Set("Authorization", "Bearer pm_api_key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendEmail(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSendHandlers_SendEmail_InvalidAPIKey(t *testing.T) {
	t.Parallel()

	mock := &mocks.UseCaseMock{GetDomainByAPIKeyFunc: func(ctx context.Context, apiKey string) (*entities.Domain, error) {
		return nil, errors.New("invalid key")
	}}
	h := NewSendHandlers(mock, &noopBillingUseCase{}, slog.Default())

	reqBody, _ := json.Marshal(SendEmailRequest{From: "noreply@example.com", To: []string{"x@y.com"}, Subject: "Hello", TextBody: "Body"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/send", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer pm_api_key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendEmail(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

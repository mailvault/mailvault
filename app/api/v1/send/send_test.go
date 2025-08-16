package send

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"mailvault/app/api/v1/send/mocks"
	"mailvault/domain/entities"

	"github.com/stretchr/testify/assert"
)

func TestSendHandlers_SendEmail_Success(t *testing.T) {
	t.Parallel()

	mock := &mocks.UseCaseMock{
		GetDomainByAPIKeyFunc: func(ctx context.Context, apiKey string) (*entities.Domain, error) {
			assert.Equal(t, "pm_api_key", apiKey)
			return &entities.Domain{Domain: "example.com"}, nil
		},
	}
	h := NewSendHandlers(mock)

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
	assert.Equal(t, "queued", resp.Status)
	assert.NotEmpty(t, resp.MessageID)
}

func TestSendHandlers_SendEmail_MissingAPIKey(t *testing.T) {
	t.Parallel()

	h := NewSendHandlers(&mocks.UseCaseMock{})
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
	h := NewSendHandlers(mock)

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
	h := NewSendHandlers(mock)

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
	h := NewSendHandlers(mock)

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
	h := NewSendHandlers(mock)

	reqBody, _ := json.Marshal(SendEmailRequest{From: "noreply@example.com", To: []string{"x@y.com"}, Subject: "Hello", TextBody: "Body"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/send", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer pm_api_key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendEmail(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

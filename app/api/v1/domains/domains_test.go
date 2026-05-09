package domains

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	billingdomain "mailvault/domain/billing"
	domainpkg "mailvault/domain/domain"
	"mailvault/domain/entities"

	"mailvault/app/api/v1/domains/mocks"

	"github.com/go-chi/chi/v5"
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

func TestDomainsHandlers_CreateDomain(t *testing.T) {
	mockUseCase := &mocks.UseCaseMock{}
	handler := NewDomainsHandlers(mockUseCase, &noopBillingUseCase{}, slog.Default())

	userID := uuid.Must(uuid.NewV4())
	domainName := "test.com"
	publicKey := "test-public-key"

	t.Run("successful creation", func(t *testing.T) {
		reqBody := CreateDomainRequest{
			Domain:    domainName,
			PublicKey: publicKey,
		}

		expectedDomain := &entities.Domain{
			ID:                 uuid.Must(uuid.NewV4()),
			UserID:             userID,
			Domain:             domainName,
			PublicKey:          publicKey,
			APIKey:             "pm_test_key",
			VerificationStatus: entities.VerificationStatusPending,
			StorageEnabled:     true,
			CreatedAt:          time.Now().UTC(),
			UpdatedAt:          time.Now().UTC(),
		}

		mockUseCase.CreateDomainFunc = func(ctx context.Context, req domainpkg.CreateDomainInput) (*entities.Domain, error) {
			assert.Equal(t, userID, req.UserID)
			assert.Equal(t, domainName, req.Domain)
			assert.Equal(t, publicKey, req.PublicKey)
			return expectedDomain, nil
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/domains", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "user_id", userID.String())
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.CreateDomain(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var result DomainResult
		err := json.Unmarshal(w.Body.Bytes(), &result)
		assert.NoError(t, err)
		assert.Equal(t, expectedDomain.ID.String(), result.ID)
		assert.Equal(t, domainName, result.Domain)
		assert.Equal(t, publicKey, result.PublicKey)

		assert.Equal(t, expectedDomain.ID.String(), result.ID)
		assert.Equal(t, domainName, result.Domain)
		assert.Equal(t, publicKey, result.PublicKey)
	})

	t.Run("with webhook config", func(t *testing.T) {
		reqBody := CreateDomainRequest{
			Domain:    domainName,
			PublicKey: publicKey,
			WebhookConfig: &WebhookConfigRequest{
				URL:     "https://example.com/webhook",
				Secret:  "secret",
				Enabled: true,
			},
		}

		expectedDomain := &entities.Domain{
			ID:        uuid.Must(uuid.NewV4()),
			UserID:    userID,
			Domain:    domainName,
			PublicKey: publicKey,
			WebhookConfig: &entities.WebhookConfig{
				URL:     "https://example.com/webhook",
				Secret:  "secret",
				Enabled: true,
			},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}

		mockUseCase.CreateDomainFunc = func(ctx context.Context, req domainpkg.CreateDomainInput) (*entities.Domain, error) {
			assert.Equal(t, userID, req.UserID)
			assert.Equal(t, domainName, req.Domain)
			assert.Equal(t, publicKey, req.PublicKey)
			return expectedDomain, nil
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/domains", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "user_id", userID.String())
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.CreateDomain(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var result DomainResult
		err := json.Unmarshal(w.Body.Bytes(), &result)
		assert.NoError(t, err)
		assert.NotNil(t, result.WebhookConfig)
		assert.Equal(t, "https://example.com/webhook", result.WebhookConfig.URL)

		assert.NotNil(t, result.WebhookConfig)
		assert.Equal(t, "https://example.com/webhook", result.WebhookConfig.URL)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/domains", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "user_id", userID.String())
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.CreateDomain(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("unauthorized", func(t *testing.T) {
		body, _ := json.Marshal(CreateDomainRequest{Domain: domainName, PublicKey: publicKey})
		req := httptest.NewRequest("POST", "/domains", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.CreateDomain(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("use case error", func(t *testing.T) {
		reqBody := CreateDomainRequest{
			Domain:    domainName,
			PublicKey: publicKey,
		}

		mockUseCase.CreateDomainFunc = func(ctx context.Context, req domainpkg.CreateDomainInput) (*entities.Domain, error) {
			return nil, assert.AnError
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/domains", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "user_id", userID.String())
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.CreateDomain(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestDomainsHandlers_GetDomains(t *testing.T) {
	mockUseCase := &mocks.UseCaseMock{}
	handler := NewDomainsHandlers(mockUseCase, &noopBillingUseCase{}, slog.Default())

	userID := uuid.Must(uuid.NewV4())

	t.Run("successful retrieval", func(t *testing.T) {
		expectedDomains := []*entities.Domain{
			{
				ID:                 uuid.Must(uuid.NewV4()),
				UserID:             userID,
				Domain:             "domain1.com",
				PublicKey:          "key1",
				APIKey:             "pm_key1",
				VerificationStatus: entities.VerificationStatusVerified,
				StorageEnabled:     true,
				CreatedAt:          time.Now().UTC(),
				UpdatedAt:          time.Now().UTC(),
			},
			{
				ID:                 uuid.Must(uuid.NewV4()),
				UserID:             userID,
				Domain:             "domain2.com",
				PublicKey:          "key2",
				APIKey:             "pm_key2",
				VerificationStatus: entities.VerificationStatusPending,
				StorageEnabled:     false,
				CreatedAt:          time.Now().UTC(),
				UpdatedAt:          time.Now().UTC(),
			},
		}

		mockUseCase.GetDomainsByUserIDFunc = func(ctx context.Context, userID uuid.UUID) ([]*entities.Domain, error) {
			assert.Equal(t, userID, userID)
			return expectedDomains, nil
		}

		req := httptest.NewRequest("GET", "/domains", nil)
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "user_id", userID.String())
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.GetDomains(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var results []*DomainResult
		err := json.Unmarshal(w.Body.Bytes(), &results)
		assert.NoError(t, err)
		assert.Len(t, results, 2)
		assert.Equal(t, "domain1.com", results[0].Domain)
		assert.Equal(t, "domain2.com", results[1].Domain)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("empty results", func(t *testing.T) {
		mockUseCase.GetDomainsByUserIDFunc = func(ctx context.Context, userID uuid.UUID) ([]*entities.Domain, error) {
			assert.Equal(t, userID, userID)
			return []*entities.Domain{}, nil
		}

		req := httptest.NewRequest("GET", "/domains", nil)
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "user_id", userID.String())
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.GetDomains(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var results []*DomainResult
		err := json.Unmarshal(w.Body.Bytes(), &results)
		assert.NoError(t, err)
		assert.Empty(t, results)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("use case error", func(t *testing.T) {
		mockUseCase.GetDomainsByUserIDFunc = func(ctx context.Context, userID uuid.UUID) ([]*entities.Domain, error) {
			assert.Equal(t, userID, userID)
			return nil, assert.AnError
		}

		req := httptest.NewRequest("GET", "/domains", nil)
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "user_id", userID.String())
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.GetDomains(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestDomainsHandlers_GetDomain(t *testing.T) {
	mockUseCase := &mocks.UseCaseMock{}
	handler := NewDomainsHandlers(mockUseCase, &noopBillingUseCase{}, slog.Default())

	userID := uuid.Must(uuid.NewV4())
	domainID := uuid.Must(uuid.NewV4())

	t.Run("successful retrieval", func(t *testing.T) {
		expectedDomain := &entities.Domain{
			ID:                 domainID,
			UserID:             userID,
			Domain:             "test.com",
			PublicKey:          "test-key",
			APIKey:             "pm_test_key",
			VerificationStatus: entities.VerificationStatusVerified,
			StorageEnabled:     true,
			CreatedAt:          time.Now().UTC(),
			UpdatedAt:          time.Now().UTC(),
		}

		mockUseCase.GetDomainByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
			assert.Equal(t, domainID, id)
			return expectedDomain, nil
		}

		req := httptest.NewRequest("GET", "/domains/"+domainID.String(), nil)
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "user_id", userID.String())
		req = req.WithContext(ctx)
		rc := chi.NewRouteContext()
		rc.URLParams.Add("id", domainID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rc))

		w := httptest.NewRecorder()

		handler.GetDomain(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var result DomainResult
		err := json.Unmarshal(w.Body.Bytes(), &result)
		assert.NoError(t, err)
		assert.Equal(t, domainID.String(), result.ID)
		assert.Equal(t, "test.com", result.Domain)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("invalid domain ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/domains/invalid-id", nil)
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "user_id", userID.String())
		req = req.WithContext(ctx)
		rc := chi.NewRouteContext()
		rc.URLParams.Add("id", "invalid-id")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rc))

		w := httptest.NewRecorder()

		handler.GetDomain(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("domain not found", func(t *testing.T) {
		mockUseCase.GetDomainByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
			assert.Equal(t, domainID, id)
			return nil, assert.AnError
		}

		req := httptest.NewRequest("GET", "/domains/"+domainID.String(), nil)
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "user_id", userID.String())
		req = req.WithContext(ctx)
		rc := chi.NewRouteContext()
		rc.URLParams.Add("id", domainID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rc))

		w := httptest.NewRecorder()

		handler.GetDomain(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("forbidden - different user", func(t *testing.T) {
		otherUserID := uuid.Must(uuid.NewV4())
		expectedDomain := &entities.Domain{
			ID:     domainID,
			UserID: otherUserID, // Different user
		}

		mockUseCase.GetDomainByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
			assert.Equal(t, domainID, id)
			return expectedDomain, nil
		}

		req := httptest.NewRequest("GET", "/domains/"+domainID.String(), nil)
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "user_id", userID.String())
		req = req.WithContext(ctx)
		rc := chi.NewRouteContext()
		rc.URLParams.Add("id", domainID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rc))

		w := httptest.NewRecorder()

		handler.GetDomain(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestDomainsHandlers_UpdateDomain(t *testing.T) {
	mockUseCase := &mocks.UseCaseMock{}
	handler := NewDomainsHandlers(mockUseCase, &noopBillingUseCase{}, slog.Default())

	userID := uuid.Must(uuid.NewV4())
	domainID := uuid.Must(uuid.NewV4())

	t.Run("successful update", func(t *testing.T) {
		reqBody := UpdateDomainRequest{
			PublicKey:          stringPtr("updated-key"),
			VerificationStatus: entities.VerificationStatusVerified,
		}

		existingDomain := &entities.Domain{
			ID:     domainID,
			UserID: userID,
			Domain: "test.com",
		}

		updatedDomain := &entities.Domain{
			ID:                 domainID,
			UserID:             userID,
			Domain:             "test.com",
			PublicKey:          "updated-key",
			VerificationStatus: entities.VerificationStatusVerified,
			UpdatedAt:          time.Now().UTC(),
		}

		mockUseCase.GetDomainByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
			assert.Equal(t, domainID, id)
			return existingDomain, nil
		}
		mockUseCase.UpdateDomainFunc = func(ctx context.Context, id uuid.UUID, req domainpkg.UpdateDomainInput) (*entities.Domain, error) {
			assert.Equal(t, domainID, id)
			return updatedDomain, nil
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("PUT", "/domains/"+domainID.String(), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "user_id", userID.String())
		req = req.WithContext(ctx)
		rc := chi.NewRouteContext()
		rc.URLParams.Add("id", domainID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rc))

		w := httptest.NewRecorder()

		handler.UpdateDomain(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var result DomainResult
		err := json.Unmarshal(w.Body.Bytes(), &result)
		assert.NoError(t, err)
		assert.Equal(t, "updated-key", result.PublicKey)
		assert.Equal(t, string(entities.VerificationStatusVerified), result.VerificationStatus)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestDomainsHandlers_DeleteDomain(t *testing.T) {
	mockUseCase := &mocks.UseCaseMock{}
	handler := NewDomainsHandlers(mockUseCase, &noopBillingUseCase{}, slog.Default())

	userID := uuid.Must(uuid.NewV4())
	domainID := uuid.Must(uuid.NewV4())

	t.Run("successful deletion", func(t *testing.T) {
		mockUseCase.DeleteDomainFunc = func(ctx context.Context, id uuid.UUID, uid uuid.UUID) error {
			assert.Equal(t, domainID, id)
			assert.Equal(t, userID, uid)
			return nil
		}

		req := httptest.NewRequest("DELETE", "/domains/"+domainID.String(), nil)
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "user_id", userID.String())
		req = req.WithContext(ctx)
		rc := chi.NewRouteContext()
		rc.URLParams.Add("id", domainID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rc))

		w := httptest.NewRecorder()

		handler.DeleteDomain(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("use case error", func(t *testing.T) {
		mockUseCase.DeleteDomainFunc = func(ctx context.Context, id uuid.UUID, uid uuid.UUID) error {
			assert.Equal(t, domainID, id)
			assert.Equal(t, userID, uid)
			return assert.AnError
		}

		req := httptest.NewRequest("DELETE", "/domains/"+domainID.String(), nil)

		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "user_id", userID.String())
		req = req.WithContext(ctx)
		rc := chi.NewRouteContext()
		rc.URLParams.Add("id", domainID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rc))

		w := httptest.NewRecorder()

		handler.DeleteDomain(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestDomainsHandlers_mapDomainToResult(t *testing.T) {
	handler := NewDomainsHandlers(&mocks.UseCaseMock{}, &noopBillingUseCase{}, slog.Default())

	t.Run("without webhook config", func(t *testing.T) {
		domain := &entities.Domain{
			ID:                 uuid.Must(uuid.NewV4()),
			Domain:             "test.com",
			PublicKey:          "test-key",
			APIKey:             "pm_test_key",
			VerificationStatus: entities.VerificationStatusVerified,
			StorageEnabled:     false,
			CreatedAt:          time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			UpdatedAt:          time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		}

		result := handler.mapDomainToResult(domain)

		assert.Equal(t, domain.ID.String(), result.ID)
		assert.Equal(t, domain.Domain, result.Domain)
		assert.Equal(t, domain.PublicKey, result.PublicKey)
		assert.Equal(t, domain.APIKey, result.APIKey)
		assert.Equal(t, string(domain.VerificationStatus), result.VerificationStatus)
		assert.Equal(t, domain.StorageEnabled, result.StorageEnabled)
		assert.Equal(t, "2024-01-01T12:00:00Z", result.CreatedAt)
		assert.Equal(t, "2024-01-02T12:00:00Z", result.UpdatedAt)
		assert.Nil(t, result.WebhookConfig)
	})

	t.Run("with webhook config", func(t *testing.T) {
		domain := &entities.Domain{
			ID: uuid.Must(uuid.NewV4()),
			WebhookConfig: &entities.WebhookConfig{
				URL:     "https://example.com/webhook",
				Secret:  "secret",
				Headers: map[string]string{"Authorization": "Bearer token"},
				Enabled: true,
			},
			CreatedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		}

		result := handler.mapDomainToResult(domain)

		assert.NotNil(t, result.WebhookConfig)
		assert.Equal(t, "https://example.com/webhook", result.WebhookConfig.URL)
		assert.Equal(t, "secret", result.WebhookConfig.Secret)
		assert.Equal(t, map[string]string{"Authorization": "Bearer token"}, result.WebhookConfig.Headers)
		assert.True(t, result.WebhookConfig.Enabled)
	})
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

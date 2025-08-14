package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domainpkg "mailvault/domain/domain"
	"mailvault/domain/entities"

	"github.com/go-chi/chi/v5"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDomainUseCase is a mock implementation for testing
type MockDomainUseCase struct {
	mock.Mock
}

func (m *MockDomainUseCase) CreateDomain(ctx context.Context, req domainpkg.CreateDomainInput) (*entities.Domain, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.Domain), args.Error(1)
}

func (m *MockDomainUseCase) GetDomainByID(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.Domain), args.Error(1)
}

func (m *MockDomainUseCase) GetDomainsByUserID(ctx context.Context, userID uuid.UUID) ([]*entities.Domain, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entities.Domain), args.Error(1)
}

func (m *MockDomainUseCase) GetDomainByAPIKey(ctx context.Context, apiKey string) (*entities.Domain, error) {
	args := m.Called(ctx, apiKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.Domain), args.Error(1)
}

func (m *MockDomainUseCase) GetDomainByName(ctx context.Context, domainName string) (*entities.Domain, error) {
	args := m.Called(ctx, domainName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.Domain), args.Error(1)
}

func (m *MockDomainUseCase) UpdateDomain(ctx context.Context, id uuid.UUID, req domainpkg.UpdateDomainInput) (*entities.Domain, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.Domain), args.Error(1)
}

func (m *MockDomainUseCase) DeleteDomain(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	args := m.Called(ctx, id, userID)
	return args.Error(0)
}

// Helper function to create a request with user context
func createRequestWithUser(method, url string, body interface{}, userID uuid.UUID) *http.Request {
	var bodyReader *bytes.Reader
	if body != nil {
		bodyBytes, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(bodyBytes)
		req := httptest.NewRequest(method, url, bodyReader)
		req.Header.Set("Content-Type", "application/json")
		// Add user ID to context
		ctx := context.WithValue(req.Context(), "user_id", userID.String())
		return req.WithContext(ctx)
	}
	req := httptest.NewRequest(method, url, nil)

	// Add user ID to context
	ctx := context.WithValue(req.Context(), "user_id", userID.String())
	return req.WithContext(ctx)
}

func TestDomainsHandlers_CreateDomain(t *testing.T) {
	mockUseCase := new(MockDomainUseCase)
	handler := NewDomainsHandlers(mockUseCase)

	userID := uuid.Must(uuid.NewV4())
	domainName := "test.com"
	publicKey := "test-public-key"

	t.Run("successful creation", func(t *testing.T) {
		reqBody := CreateDomainRequest{
			Domain:    domainName,
			PublicKey: publicKey,
		}

		expectedDomain := &entities.Domain{
			ID:             uuid.Must(uuid.NewV4()),
			UserID:         userID,
			Domain:         domainName,
			PublicKey:      publicKey,
			APIKey:         "pm_test_key",
			Verified:       false,
			StorageEnabled: true,
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}

		mockUseCase.On("CreateDomain", mock.Anything, mock.MatchedBy(func(req domainpkg.CreateDomainInput) bool {
			return req.UserID == userID && req.Domain == domainName && req.PublicKey == publicKey
		})).Return(expectedDomain, nil).Once()

		req := createRequestWithUser("POST", "/domains", reqBody, userID)
		w := httptest.NewRecorder()

		handler.CreateDomain(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var result DomainResult
		err := json.Unmarshal(w.Body.Bytes(), &result)
		assert.NoError(t, err)
		assert.Equal(t, expectedDomain.ID.String(), result.ID)
		assert.Equal(t, domainName, result.Domain)
		assert.Equal(t, publicKey, result.PublicKey)

		mockUseCase.AssertExpectations(t)
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

		mockUseCase.On("CreateDomain", mock.Anything, mock.Anything).Return(expectedDomain, nil).Once()

		req := createRequestWithUser("POST", "/domains", reqBody, userID)
		w := httptest.NewRecorder()

		handler.CreateDomain(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var result DomainResult
		err := json.Unmarshal(w.Body.Bytes(), &result)
		assert.NoError(t, err)
		assert.NotNil(t, result.WebhookConfig)
		assert.Equal(t, "https://example.com/webhook", result.WebhookConfig.URL)

		mockUseCase.AssertExpectations(t)
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
		reqBody := CreateDomainRequest{
			Domain:    domainName,
			PublicKey: publicKey,
		}

		req := createRequestWithUser("POST", "/domains", reqBody, uuid.Nil) // No user in context
		req = httptest.NewRequest("POST", "/domains", bytes.NewReader([]byte{}))
		w := httptest.NewRecorder()

		handler.CreateDomain(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("use case error", func(t *testing.T) {
		reqBody := CreateDomainRequest{
			Domain:    domainName,
			PublicKey: publicKey,
		}

		mockUseCase.On("CreateDomain", mock.Anything, mock.Anything).Return(nil, assert.AnError).Once()

		req := createRequestWithUser("POST", "/domains", reqBody, userID)
		w := httptest.NewRecorder()

		handler.CreateDomain(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		mockUseCase.AssertExpectations(t)
	})
}

func TestDomainsHandlers_GetDomains(t *testing.T) {
	mockUseCase := new(MockDomainUseCase)
	handler := NewDomainsHandlers(mockUseCase)

	userID := uuid.Must(uuid.NewV4())

	t.Run("successful retrieval", func(t *testing.T) {
		expectedDomains := []*entities.Domain{
			{
				ID:             uuid.Must(uuid.NewV4()),
				UserID:         userID,
				Domain:         "domain1.com",
				PublicKey:      "key1",
				APIKey:         "pm_key1",
				Verified:       true,
				StorageEnabled: true,
				CreatedAt:      time.Now().UTC(),
				UpdatedAt:      time.Now().UTC(),
			},
			{
				ID:             uuid.Must(uuid.NewV4()),
				UserID:         userID,
				Domain:         "domain2.com",
				PublicKey:      "key2",
				APIKey:         "pm_key2",
				Verified:       false,
				StorageEnabled: false,
				CreatedAt:      time.Now().UTC(),
				UpdatedAt:      time.Now().UTC(),
			},
		}

		mockUseCase.On("GetDomainsByUserID", mock.Anything, userID).Return(expectedDomains, nil).Once()

		req := createRequestWithUser("GET", "/domains", nil, userID)
		w := httptest.NewRecorder()

		handler.GetDomains(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var results []*DomainResult
		err := json.Unmarshal(w.Body.Bytes(), &results)
		assert.NoError(t, err)
		assert.Len(t, results, 2)
		assert.Equal(t, "domain1.com", results[0].Domain)
		assert.Equal(t, "domain2.com", results[1].Domain)

		mockUseCase.AssertExpectations(t)
	})

	t.Run("empty results", func(t *testing.T) {
		mockUseCase.On("GetDomainsByUserID", mock.Anything, userID).Return([]*entities.Domain{}, nil).Once()

		req := createRequestWithUser("GET", "/domains", nil, userID)
		w := httptest.NewRecorder()

		handler.GetDomains(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var results []*DomainResult
		err := json.Unmarshal(w.Body.Bytes(), &results)
		assert.NoError(t, err)
		assert.Empty(t, results)

		mockUseCase.AssertExpectations(t)
	})

	t.Run("use case error", func(t *testing.T) {
		mockUseCase.On("GetDomainsByUserID", mock.Anything, userID).Return(nil, assert.AnError).Once()

		req := createRequestWithUser("GET", "/domains", nil, userID)
		w := httptest.NewRecorder()

		handler.GetDomains(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		mockUseCase.AssertExpectations(t)
	})
}

func TestDomainsHandlers_GetDomain(t *testing.T) {
	mockUseCase := new(MockDomainUseCase)
	handler := NewDomainsHandlers(mockUseCase)

	userID := uuid.Must(uuid.NewV4())
	domainID := uuid.Must(uuid.NewV4())

	t.Run("successful retrieval", func(t *testing.T) {
		expectedDomain := &entities.Domain{
			ID:             domainID,
			UserID:         userID,
			Domain:         "test.com",
			PublicKey:      "test-key",
			APIKey:         "pm_test_key",
			Verified:       true,
			StorageEnabled: true,
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}

		mockUseCase.On("GetDomainByID", mock.Anything, domainID).Return(expectedDomain, nil).Once()

		req := createRequestWithUser("GET", "/domains/"+domainID.String(), nil, userID)

		// Set up router with URL parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", domainID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.GetDomain(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var result DomainResult
		err := json.Unmarshal(w.Body.Bytes(), &result)
		assert.NoError(t, err)
		assert.Equal(t, domainID.String(), result.ID)
		assert.Equal(t, "test.com", result.Domain)

		mockUseCase.AssertExpectations(t)
	})

	t.Run("invalid domain ID", func(t *testing.T) {
		req := createRequestWithUser("GET", "/domains/invalid-id", nil, userID)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "invalid-id")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.GetDomain(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("domain not found", func(t *testing.T) {
		mockUseCase.On("GetDomainByID", mock.Anything, domainID).Return(nil, assert.AnError).Once()

		req := createRequestWithUser("GET", "/domains/"+domainID.String(), nil, userID)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", domainID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.GetDomain(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		mockUseCase.AssertExpectations(t)
	})

	t.Run("forbidden - different user", func(t *testing.T) {
		otherUserID := uuid.Must(uuid.NewV4())
		expectedDomain := &entities.Domain{
			ID:     domainID,
			UserID: otherUserID, // Different user
		}

		mockUseCase.On("GetDomainByID", mock.Anything, domainID).Return(expectedDomain, nil).Once()

		req := createRequestWithUser("GET", "/domains/"+domainID.String(), nil, userID)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", domainID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.GetDomain(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)

		mockUseCase.AssertExpectations(t)
	})
}

func TestDomainsHandlers_UpdateDomain(t *testing.T) {
	mockUseCase := new(MockDomainUseCase)
	handler := NewDomainsHandlers(mockUseCase)

	userID := uuid.Must(uuid.NewV4())
	domainID := uuid.Must(uuid.NewV4())

	t.Run("successful update", func(t *testing.T) {
		reqBody := UpdateDomainRequest{
			PublicKey: stringPtr("updated-key"),
			Verified:  boolPtr(true),
		}

		existingDomain := &entities.Domain{
			ID:     domainID,
			UserID: userID,
			Domain: "test.com",
		}

		updatedDomain := &entities.Domain{
			ID:        domainID,
			UserID:    userID,
			Domain:    "test.com",
			PublicKey: "updated-key",
			Verified:  true,
			UpdatedAt: time.Now().UTC(),
		}

		mockUseCase.On("GetDomainByID", mock.Anything, domainID).Return(existingDomain, nil).Once()
		mockUseCase.On("UpdateDomain", mock.Anything, domainID, mock.Anything).Return(updatedDomain, nil).Once()

		req := createRequestWithUser("PUT", "/domains/"+domainID.String(), reqBody, userID)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", domainID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.UpdateDomain(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var result DomainResult
		err := json.Unmarshal(w.Body.Bytes(), &result)
		assert.NoError(t, err)
		assert.Equal(t, "updated-key", result.PublicKey)
		assert.True(t, result.Verified)

		mockUseCase.AssertExpectations(t)
	})
}

func TestDomainsHandlers_DeleteDomain(t *testing.T) {
	mockUseCase := new(MockDomainUseCase)
	handler := NewDomainsHandlers(mockUseCase)

	userID := uuid.Must(uuid.NewV4())
	domainID := uuid.Must(uuid.NewV4())

	t.Run("successful deletion", func(t *testing.T) {
		mockUseCase.On("DeleteDomain", mock.Anything, domainID, userID).Return(nil).Once()

		req := createRequestWithUser("DELETE", "/domains/"+domainID.String(), nil, userID)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", domainID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.DeleteDomain(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)

		mockUseCase.AssertExpectations(t)
	})

	t.Run("use case error", func(t *testing.T) {
		mockUseCase.On("DeleteDomain", mock.Anything, domainID, userID).Return(assert.AnError).Once()

		req := createRequestWithUser("DELETE", "/domains/"+domainID.String(), nil, userID)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", domainID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.DeleteDomain(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		mockUseCase.AssertExpectations(t)
	})
}

func TestDomainsHandlers_mapDomainToResult(t *testing.T) {
	handler := &DomainsHandlers{}

	t.Run("without webhook config", func(t *testing.T) {
		domain := &entities.Domain{
			ID:             uuid.Must(uuid.NewV4()),
			Domain:         "test.com",
			PublicKey:      "test-key",
			APIKey:         "pm_test_key",
			Verified:       true,
			StorageEnabled: false,
			CreatedAt:      time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			UpdatedAt:      time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		}

		result := handler.mapDomainToResult(domain)

		assert.Equal(t, domain.ID.String(), result.ID)
		assert.Equal(t, domain.Domain, result.Domain)
		assert.Equal(t, domain.PublicKey, result.PublicKey)
		assert.Equal(t, domain.APIKey, result.APIKey)
		assert.Equal(t, domain.Verified, result.Verified)
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

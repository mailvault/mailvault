package domains

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mailvault/app/api/v1/domains/mocks"
	"mailvault/domain/entities"

	"github.com/go-chi/chi/v5"
	"github.com/gofrs/uuid/v5"
)

func setupValidateHandler() (*DomainsHandlers, *mocks.UseCaseMock) {
	mockUseCase := &mocks.UseCaseMock{}
	handler := NewDomainsHandlers(mockUseCase, nil, nil)
	return handler, mockUseCase
}

func newDomainForValidation(id, userID uuid.UUID, domainName string) *entities.Domain {
	return &entities.Domain{
		ID:                 id,
		UserID:             userID,
		Domain:             domainName,
		VerificationStatus: entities.VerificationStatusPending,
		VerificationToken:  "mailvault-verification-testtoken",
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
	}
}

func TestValidateDomain_Success(t *testing.T) {
	handler, mockUseCase := setupValidateHandler()

	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	domain := newDomainForValidation(domainID, userID, "test.com")

	mockUseCase.GetDomainByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
		return domain, nil
	}

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/domains/%s/validate", domainID), nil)
	ctx := context.WithValue(req.Context(), "user_id", userID.String())
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", domainID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ValidateDomain(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestValidateDomain_InvalidDomainID(t *testing.T) {
	handler, _ := setupValidateHandler()

	userID := uuid.Must(uuid.NewV4())

	req := httptest.NewRequest(http.MethodPost, "/domains/invalid-uuid/validate", nil)
	ctx := context.WithValue(req.Context(), "user_id", userID.String())
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-uuid")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ValidateDomain(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestValidateDomain_DomainNotFound(t *testing.T) {
	handler, mockUseCase := setupValidateHandler()

	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	mockUseCase.GetDomainByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
		return nil, fmt.Errorf("domain not found")
	}

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/domains/%s/validate", domainID), nil)
	ctx := context.WithValue(req.Context(), "user_id", userID.String())
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", domainID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ValidateDomain(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestGetValidationStatus_Success(t *testing.T) {
	handler, mockUseCase := setupValidateHandler()

	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	domain := newDomainForValidation(domainID, userID, "test.com")
	domain.VerificationStatus = entities.VerificationStatusVerified
	domain.VerificationAttempts = 2

	mockUseCase.GetDomainByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
		return domain, nil
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/domains/%s/validation/status", domainID), nil)
	ctx := context.WithValue(req.Context(), "user_id", userID.String())
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", domainID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetValidationStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
}

func TestGetValidationInstructions_Success(t *testing.T) {
	handler, mockUseCase := setupValidateHandler()

	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	domain := newDomainForValidation(domainID, userID, "test.com")

	mockUseCase.GetDomainByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
		return domain, nil
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/domains/%s/validation/instructions", domainID), nil)
	ctx := context.WithValue(req.Context(), "user_id", userID.String())
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", domainID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetValidationInstructions(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestRetryValidation_Success(t *testing.T) {
	handler, mockUseCase := setupValidateHandler()

	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	domain := newDomainForValidation(domainID, userID, "test.com")
	// Retry requires the previous verification to have failed.
	domain.VerificationStatus = entities.VerificationStatusFailed
	// Allow retry: NextVerificationAttempt is zero (not set)

	mockUseCase.GetDomainByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
		return domain, nil
	}

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/domains/%s/validation/retry", domainID), nil)
	ctx := context.WithValue(req.Context(), "user_id", userID.String())
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", domainID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.RetryValidation(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestValidation_UnauthorizedUser(t *testing.T) {
	handler, mockUseCase := setupValidateHandler()

	domainID := uuid.Must(uuid.NewV4())
	domainUserID := uuid.Must(uuid.NewV4())
	requestUserID := uuid.Must(uuid.NewV4()) // Different user

	domain := newDomainForValidation(domainID, domainUserID, "test.com")

	mockUseCase.GetDomainByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
		return domain, nil
	}

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/domains/%s/validate", domainID), nil)
	ctx := context.WithValue(req.Context(), "user_id", requestUserID.String())
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", domainID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ValidateDomain(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestValidation_NoUserInContext(t *testing.T) {
	handler, _ := setupValidateHandler()

	domainID := uuid.Must(uuid.NewV4())

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/domains/%s/validate", domainID), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", domainID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	handler.ValidateDomain(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

// Benchmark tests
func BenchmarkValidateDomain(b *testing.B) {
	handler, mockUseCase := setupValidateHandler()

	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	domain := newDomainForValidation(domainID, userID, "test.com")
	mockUseCase.GetDomainByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
		return domain, nil
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/domains/%s/validate", domainID), nil)
		ctx := context.WithValue(req.Context(), "user_id", userID.String())
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", domainID.String())
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler.ValidateDomain(rr, req)
	}
}

func BenchmarkGetValidationStatus(b *testing.B) {
	handler, mockUseCase := setupValidateHandler()

	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	domain := newDomainForValidation(domainID, userID, "test.com")
	domain.VerificationStatus = entities.VerificationStatusVerified
	mockUseCase.GetDomainByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
		return domain, nil
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/domains/%s/validation/status", domainID), nil)
		ctx := context.WithValue(req.Context(), "user_id", userID.String())
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", domainID.String())
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler.GetValidationStatus(rr, req)
	}
}

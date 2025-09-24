package domains

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mailvault/domain/validation"

	"github.com/go-chi/chi/v5"
	"github.com/gofrs/uuid/v5"
)

// Mock validation use case for testing
type mockValidationUseCase struct {
	validateResult                   *validation.FullValidationResult
	validateError                    error
	validationStatus                 *validation.ValidationStatus
	validationStatusError            error
	validationInstructions           *validation.ValidationInstructions
	validationInstructionsError      error
	retryResult                      *validation.FullValidationResult
	retryError                       error
	validateDomainCalls              []uuid.UUID
	getValidationStatusCalls         []uuid.UUID
	getValidationInstructionsCalls   []uuid.UUID
	retryValidationCalls             []uuid.UUID
}

func (m *mockValidationUseCase) ValidateDomain(ctx context.Context, domainID uuid.UUID) (*validation.FullValidationResult, error) {
	m.validateDomainCalls = append(m.validateDomainCalls, domainID)
	if m.validateError != nil {
		return nil, m.validateError
	}
	if m.validateResult != nil {
		return m.validateResult, nil
	}
	return &validation.FullValidationResult{
		Domain:       "test.com",
		OverallValid: true,
		TotalTime:    100 * time.Millisecond,
	}, nil
}

func (m *mockValidationUseCase) GetValidationStatus(ctx context.Context, domainID uuid.UUID) (*validation.ValidationStatus, error) {
	m.getValidationStatusCalls = append(m.getValidationStatusCalls, domainID)
	if m.validationStatusError != nil {
		return nil, m.validationStatusError
	}
	if m.validationStatus != nil {
		return m.validationStatus, nil
	}
	return &validation.ValidationStatus{
		Status:   validation.VerificationStatusVerified,
		Attempts: 1,
		Records:  []*validation.ValidationRecord{},
	}, nil
}

func (m *mockValidationUseCase) GetValidationInstructions(ctx context.Context, domainID uuid.UUID) (*validation.ValidationInstructions, error) {
	m.getValidationInstructionsCalls = append(m.getValidationInstructionsCalls, domainID)
	if m.validationInstructionsError != nil {
		return nil, m.validationInstructionsError
	}
	if m.validationInstructions != nil {
		return m.validationInstructions, nil
	}
	return &validation.ValidationInstructions{
		Domain:    "test.com",
		MXRecords: []string{"mail.mailvault.sh", "mail2.mailvault.sh"},
		TXTRecord: "mailvault-verification=test123",
	}, nil
}

func (m *mockValidationUseCase) RetryValidation(ctx context.Context, domainID uuid.UUID) (*validation.FullValidationResult, error) {
	m.retryValidationCalls = append(m.retryValidationCalls, domainID)
	if m.retryError != nil {
		return nil, m.retryError
	}
	if m.retryResult != nil {
		return m.retryResult, nil
	}
	return &validation.FullValidationResult{
		Domain:       "test.com",
		OverallValid: true,
		TotalTime:    150 * time.Millisecond,
	}, nil
}

// Mock domain use case for testing
type mockDomainUseCase struct {
	domain    *mockDomain
	getError  error
}

type mockDomain struct {
	ID     uuid.UUID
	UserID uuid.UUID
	Domain string
}

func (m *mockDomainUseCase) GetDomainByID(ctx context.Context, id uuid.UUID) (interface{}, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	if m.domain != nil {
		return m.domain, nil
	}
	return &mockDomain{
		ID:     id,
		UserID: uuid.Must(uuid.NewV4()),
		Domain: "test.com",
	}, nil
}

func setupTestHandler() (*DomainsHandlers, *mockValidationUseCase, *mockDomainUseCase) {
	mockValidationUC := &mockValidationUseCase{}
	mockDomainUC := &mockDomainUseCase{}

	handler := &DomainsHandlers{
		validationUseCase: mockValidationUC,
		domainUseCase:     mockDomainUC,
	}

	return handler, mockValidationUC, mockDomainUC
}

func TestValidateDomain_Success(t *testing.T) {
	handler, mockValidationUC, mockDomainUC := setupTestHandler()

	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	mockDomainUC.domain = &mockDomain{
		ID:     domainID,
		UserID: userID,
		Domain: "test.com",
	}

	mockValidationUC.validateResult = &validation.FullValidationResult{
		Domain:       "test.com",
		OverallValid: true,
		MXValidation: &validation.MXValidationResult{
			Domain: "test.com",
			Valid:  true,
		},
		TXTValidation: &validation.TXTValidationResult{
			Domain: "test.com",
			Valid:  true,
		},
		TotalTime: 200 * time.Millisecond,
	}

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/domains/%s/validate", domainID), nil)
	req = req.WithContext(context.WithValue(req.Context(), "userID", userID))

	rr := httptest.NewRecorder()

	// Setup chi router context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", domainID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.ValidateDomain(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result ValidationResult
	err := json.NewDecoder(rr.Body).Decode(&result)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !result.Success {
		t.Error("Expected validation to succeed")
	}

	if result.Domain != "test.com" {
		t.Errorf("Expected domain 'test.com', got '%s'", result.Domain)
	}

	// Verify use case was called
	if len(mockValidationUC.validateDomainCalls) != 1 {
		t.Errorf("Expected 1 ValidateDomain call, got %d", len(mockValidationUC.validateDomainCalls))
	}
}

func TestValidateDomain_InvalidDomainID(t *testing.T) {
	handler, _, _ := setupTestHandler()

	req := httptest.NewRequest(http.MethodPost, "/domains/invalid-uuid/validate", nil)
	req = req.WithContext(context.WithValue(req.Context(), "userID", uuid.Must(uuid.NewV4())))

	rr := httptest.NewRecorder()

	// Setup chi router context with invalid UUID
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.ValidateDomain(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestValidateDomain_DomainNotFound(t *testing.T) {
	handler, _, mockDomainUC := setupTestHandler()

	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	mockDomainUC.getError = errors.New("domain not found")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/domains/%s/validate", domainID), nil)
	req = req.WithContext(context.WithValue(req.Context(), "userID", userID))

	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", domainID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.ValidateDomain(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestValidateDomain_ValidationError(t *testing.T) {
	handler, mockValidationUC, mockDomainUC := setupTestHandler()

	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	mockDomainUC.domain = &mockDomain{
		ID:     domainID,
		UserID: userID,
		Domain: "test.com",
	}

	mockValidationUC.validateError = errors.New("DNS lookup failed")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/domains/%s/validate", domainID), nil)
	req = req.WithContext(context.WithValue(req.Context(), "userID", userID))

	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", domainID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.ValidateDomain(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestGetValidationStatus_Success(t *testing.T) {
	handler, mockValidationUC, mockDomainUC := setupTestHandler()

	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	mockDomainUC.domain = &mockDomain{
		ID:     domainID,
		UserID: userID,
		Domain: "test.com",
	}

	mockValidationUC.validationStatus = &validation.ValidationStatus{
		Status:        validation.VerificationStatusVerified,
		Attempts:      2,
		LastAttempt:   timePtr(time.Now().Add(-1 * time.Hour)),
		NextAttempt:   nil,
		Records:       []*validation.ValidationRecord{},
		SuccessRate:   100.0,
		AverageTime:   150 * time.Millisecond,
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/domains/%s/validation/status", domainID), nil)
	req = req.WithContext(context.WithValue(req.Context(), "userID", userID))

	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", domainID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetValidationStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var status ValidationStatusResult
	err := json.NewDecoder(rr.Body).Decode(&status)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if status.Status != string(validation.VerificationStatusVerified) {
		t.Errorf("Expected status '%s', got '%s'", validation.VerificationStatusVerified, status.Status)
	}

	if status.Attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", status.Attempts)
	}

	// Verify use case was called
	if len(mockValidationUC.getValidationStatusCalls) != 1 {
		t.Errorf("Expected 1 GetValidationStatus call, got %d", len(mockValidationUC.getValidationStatusCalls))
	}
}

func TestGetValidationInstructions_Success(t *testing.T) {
	handler, mockValidationUC, mockDomainUC := setupTestHandler()

	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	mockDomainUC.domain = &mockDomain{
		ID:     domainID,
		UserID: userID,
		Domain: "test.com",
	}

	mockValidationUC.validationInstructions = &validation.ValidationInstructions{
		Domain:    "test.com",
		MXRecords: []string{"mail.mailvault.sh", "mail2.mailvault.sh"},
		TXTRecord: "mailvault-verification=abc123",
		Steps: []string{
			"Add the MX records to your DNS",
			"Add the TXT record to your DNS",
			"Wait for DNS propagation",
		},
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/domains/%s/validation/instructions", domainID), nil)
	req = req.WithContext(context.WithValue(req.Context(), "userID", userID))

	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", domainID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetValidationInstructions(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var instructions ValidationInstructionsResult
	err := json.NewDecoder(rr.Body).Decode(&instructions)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if instructions.Domain != "test.com" {
		t.Errorf("Expected domain 'test.com', got '%s'", instructions.Domain)
	}

	if len(instructions.MXRecords) != 2 {
		t.Errorf("Expected 2 MX records, got %d", len(instructions.MXRecords))
	}

	if instructions.TXTRecord != "mailvault-verification=abc123" {
		t.Errorf("Expected TXT record 'mailvault-verification=abc123', got '%s'", instructions.TXTRecord)
	}

	// Verify use case was called
	if len(mockValidationUC.getValidationInstructionsCalls) != 1 {
		t.Errorf("Expected 1 GetValidationInstructions call, got %d", len(mockValidationUC.getValidationInstructionsCalls))
	}
}

func TestRetryValidation_Success(t *testing.T) {
	handler, mockValidationUC, mockDomainUC := setupTestHandler()

	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	mockDomainUC.domain = &mockDomain{
		ID:     domainID,
		UserID: userID,
		Domain: "test.com",
	}

	mockValidationUC.retryResult = &validation.FullValidationResult{
		Domain:       "test.com",
		OverallValid: true,
		MXValidation: &validation.MXValidationResult{
			Domain: "test.com",
			Valid:  true,
		},
		TXTValidation: &validation.TXTValidationResult{
			Domain: "test.com",
			Valid:  true,
		},
		TotalTime: 180 * time.Millisecond,
	}

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/domains/%s/validation/retry", domainID), nil)
	req = req.WithContext(context.WithValue(req.Context(), "userID", userID))

	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", domainID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.RetryValidation(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result ValidationResult
	err := json.NewDecoder(rr.Body).Decode(&result)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !result.Success {
		t.Error("Expected retry validation to succeed")
	}

	// Verify use case was called
	if len(mockValidationUC.retryValidationCalls) != 1 {
		t.Errorf("Expected 1 RetryValidation call, got %d", len(mockValidationUC.retryValidationCalls))
	}
}

func TestRetryValidation_TooSoon(t *testing.T) {
	handler, mockValidationUC, mockDomainUC := setupTestHandler()

	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	mockDomainUC.domain = &mockDomain{
		ID:     domainID,
		UserID: userID,
		Domain: "test.com",
	}

	mockValidationUC.retryError = errors.New("retry too soon")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/domains/%s/validation/retry", domainID), nil)
	req = req.WithContext(context.WithValue(req.Context(), "userID", userID))

	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", domainID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.RetryValidation(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestValidation_UnauthorizedUser(t *testing.T) {
	handler, _, mockDomainUC := setupTestHandler()

	domainID := uuid.Must(uuid.NewV4())
	domainUserID := uuid.Must(uuid.NewV4())
	requestUserID := uuid.Must(uuid.NewV4()) // Different user

	mockDomainUC.domain = &mockDomain{
		ID:     domainID,
		UserID: domainUserID, // Domain belongs to different user
		Domain: "test.com",
	}

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/domains/%s/validate", domainID), nil)
	req = req.WithContext(context.WithValue(req.Context(), "userID", requestUserID))

	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", domainID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.ValidateDomain(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestValidation_NoUserInContext(t *testing.T) {
	handler, _, _ := setupTestHandler()

	domainID := uuid.Must(uuid.NewV4())

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/domains/%s/validate", domainID), nil)
	// No userID in context

	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", domainID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.ValidateDomain(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

// Helper functions
func timePtr(t time.Time) *time.Time {
	return &t
}

// Benchmark tests
func BenchmarkValidateDomain(b *testing.B) {
	handler, mockValidationUC, mockDomainUC := setupTestHandler()

	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	mockDomainUC.domain = &mockDomain{
		ID:     domainID,
		UserID: userID,
		Domain: "test.com",
	}

	mockValidationUC.validateResult = &validation.FullValidationResult{
		Domain:       "test.com",
		OverallValid: true,
		TotalTime:    100 * time.Millisecond,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/domains/%s/validate", domainID), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userID))

		rr := httptest.NewRecorder()

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", domainID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		handler.ValidateDomain(rr, req)
	}
}

func BenchmarkGetValidationStatus(b *testing.B) {
	handler, mockValidationUC, mockDomainUC := setupTestHandler()

	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	mockDomainUC.domain = &mockDomain{
		ID:     domainID,
		UserID: userID,
		Domain: "test.com",
	}

	mockValidationUC.validationStatus = &validation.ValidationStatus{
		Status:   validation.VerificationStatusVerified,
		Attempts: 1,
		Records:  []*validation.ValidationRecord{},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/domains/%s/validation/status", domainID), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userID))

		rr := httptest.NewRecorder()

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", domainID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		handler.GetValidationStatus(rr, req)
	}
}
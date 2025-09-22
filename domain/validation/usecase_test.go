package validation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/guilhermebr/gox/logger"
)

// Mock repository for testing
type mockValidationRepository struct {
	domains               map[uuid.UUID]*DomainValidationInfo
	records               map[uuid.UUID][]*ValidationRecord
	createValidationError error
	getValidationError    error
	updateValidationError error
}

func newMockValidationRepository() *mockValidationRepository {
	return &mockValidationRepository{
		domains: make(map[uuid.UUID]*DomainValidationInfo),
		records: make(map[uuid.UUID][]*ValidationRecord),
	}
}

func (m *mockValidationRepository) CreateValidation(ctx context.Context, validation *DomainValidationInfo) error {
	if m.createValidationError != nil {
		return m.createValidationError
	}
	m.domains[validation.ID] = validation
	return nil
}

func (m *mockValidationRepository) GetValidationByDomainID(ctx context.Context, domainID uuid.UUID) (*DomainValidationInfo, error) {
	if m.getValidationError != nil {
		return nil, m.getValidationError
	}
	for _, domain := range m.domains {
		if domain.ID == domainID {
			return domain, nil
		}
	}
	return nil, errors.New("validation not found")
}

func (m *mockValidationRepository) UpdateValidation(ctx context.Context, validation *DomainValidationInfo) error {
	if m.updateValidationError != nil {
		return m.updateValidationError
	}
	m.domains[validation.ID] = validation
	return nil
}

func (m *mockValidationRepository) CreateValidationRecord(ctx context.Context, record *ValidationRecord) error {
	m.records[record.DomainID] = append(m.records[record.DomainID], record)
	return nil
}

func (m *mockValidationRepository) GetValidationRecords(ctx context.Context, domainValidationID uuid.UUID, limit int) ([]*ValidationRecord, error) {
	records := m.records[domainValidationID]
	if limit > 0 && len(records) > limit {
		return records[:limit], nil
	}
	return records, nil
}

func (m *mockValidationRepository) UpdateValidationRecord(ctx context.Context, record *ValidationRecord) error {
	records := m.records[record.DomainID]
	for i, r := range records {
		if r.ID == record.ID {
			records[i] = record
			break
		}
	}
	return nil
}

func (m *mockValidationRepository) GetPendingValidations(ctx context.Context, limit int) ([]*DomainValidationInfo, error) {
	var pending []*DomainValidationInfo
	for _, domain := range m.domains {
		if domain.CanRetry() {
			pending = append(pending, domain)
		}
	}
	if limit > 0 && len(pending) > limit {
		return pending[:limit], nil
	}
	return pending, nil
}

// Mock DNS validator for testing
type mockDNSValidator struct {
	validateMXResult   *MXValidationResult
	validateTXTResult  *TXTValidationResult
	validateFullResult *FullValidationResult
	validateError      error
}

func (m *mockDNSValidator) ValidateMXRecords(ctx context.Context, domain string, expectedServers []string) (*MXValidationResult, error) {
	if m.validateError != nil {
		return nil, m.validateError
	}
	if m.validateMXResult != nil {
		return m.validateMXResult, nil
	}
	return &MXValidationResult{
		Valid:        true,
		FoundRecords: []MXRecord{{Host: "mail.mailvault.sh", Priority: 10}},
		QueryTime:    100 * time.Millisecond,
	}, nil
}

func (m *mockDNSValidator) ValidateTXTRecord(ctx context.Context, domain string, expectedRecord string) (*TXTValidationResult, error) {
	if m.validateError != nil {
		return nil, m.validateError
	}
	if m.validateTXTResult != nil {
		return m.validateTXTResult, nil
	}
	return &TXTValidationResult{
		Valid:         true,
		FoundRecords:  []string{expectedRecord},
		QueryTime:     50 * time.Millisecond,
		RetryCount:    0,
	}, nil
}

func (m *mockDNSValidator) ValidateFullDomain(ctx context.Context, domain string, verificationToken string, config *ValidationConfig) (*FullValidationResult, error) {
	if m.validateError != nil {
		return nil, m.validateError
	}
	if m.validateFullResult != nil {
		return m.validateFullResult, nil
	}
	return &FullValidationResult{
		OverallValid:  true,
		MXValidation:  &MXValidationResult{Valid: true},
		TXTValidation: &TXTValidationResult{Valid: true},
		TotalTime:     150 * time.Millisecond,
	}, nil
}

func TestUseCase_CreateValidation(t *testing.T) {
	repo := newMockValidationRepository()
	validator := &mockDNSValidator{}
	logger, _ := logger.NewLogger("")
	uc := NewUseCase(repo, validator, logger)

	domainID := uuid.Must(uuid.NewV4())
	domain := "example.com"

	validation, err := uc.CreateValidation(context.Background(), domainID, domain)

	if err != nil {
		t.Fatalf("CreateValidation() error = %v", err)
	}

	if validation == nil {
		t.Fatal("CreateValidation() returned nil validation")
	}

	if validation.ID == uuid.Nil {
		t.Error("CreateValidation() should set validation ID")
	}

	if validation.Domain != domain {
		t.Errorf("CreateValidation() domain = %v, want %v", validation.Domain, domain)
	}

	if validation.VerificationStatus != VerificationStatusPending {
		t.Errorf("CreateValidation() status = %v, want %v", validation.VerificationStatus, VerificationStatusPending)
	}

	if validation.VerificationToken == nil || *validation.VerificationToken == "" {
		t.Error("CreateValidation() should generate verification token")
	}
}

func TestUseCase_CreateValidation_RepositoryError(t *testing.T) {
	repo := newMockValidationRepository()
	repo.createValidationError = errors.New("database error")
	validator := &mockDNSValidator{}
	logger, _ := logger.NewLogger("")
	uc := NewUseCase(repo, validator, logger)

	domainID := uuid.Must(uuid.NewV4())
	domain := "example.com"

	validation, err := uc.CreateValidation(context.Background(), domainID, domain)

	if err == nil {
		t.Error("CreateValidation() should return error when repository fails")
	}

	if validation != nil {
		t.Error("CreateValidation() should return nil validation on error")
	}
}

func TestUseCase_ValidateDomain_Success(t *testing.T) {
	repo := newMockValidationRepository()
	validator := &mockDNSValidator{}
	logger, _ := logger.NewLogger("")
	uc := NewUseCase(repo, validator, logger)

	// Create test validation
	domainID := uuid.Must(uuid.NewV4())
	validation := &DomainValidationInfo{
		ID:                 uuid.Must(uuid.NewV4()),
		Domain:             "example.com",
		VerificationStatus: VerificationStatusPending,
		VerificationToken:  stringPtr("test123"),
	}
	repo.domains[validation.ID] = validation

	result, err := uc.ValidateDomain(context.Background(), domainID)

	if err != nil {
		t.Fatalf("ValidateDomain() error = %v", err)
	}

	if result == nil {
		t.Fatal("ValidateDomain() returned nil result")
	}

	if !result.OverallValid {
		t.Error("ValidateDomain() should return valid result for successful validation")
	}

	// Check that validation was updated
	updatedValidation := repo.domains[validation.ID]
	if updatedValidation.VerificationStatus != VerificationStatusVerified {
		t.Errorf("ValidateDomain() should update status to verified, got %v", updatedValidation.VerificationStatus)
	}
}

func TestUseCase_ValidateDomain_Failed(t *testing.T) {
	repo := newMockValidationRepository()
	validator := &mockDNSValidator{
		validateFullResult: &FullValidationResult{
			Domain:       "example.com",
			OverallValid: false,
			MXValidation: &MXValidationResult{
				Domain: "example.com",
				Valid:  false,
				Error:  "MX records not found",
			},
			TXTValidation: &TXTValidationResult{
				Domain: "example.com",
				Valid:  false,
				Error:  "TXT record not found",
			},
			TotalTime: 200 * time.Millisecond,
		},
	}
	logger, _ := logger.NewLogger("")
	uc := NewUseCase(repo, validator, logger)

	// Create test validation
	domainID := uuid.Must(uuid.NewV4())
	validation := &DomainValidationInfo{
		ID:                 uuid.Must(uuid.NewV4()),
		Domain:             "example.com",
		VerificationStatus: VerificationStatusPending,
		VerificationToken:  stringPtr("test123"),
	}
	repo.domains[validation.ID] = validation

	result, err := uc.ValidateDomain(context.Background(), domainID)

	if err != nil {
		t.Fatalf("ValidateDomain() error = %v", err)
	}

	if result.OverallValid {
		t.Error("ValidateDomain() should return invalid result for failed validation")
	}

	// Check that validation was updated with failure
	updatedValidation := repo.domains[validation.ID]
	if updatedValidation.VerificationStatus != VerificationStatusFailed {
		t.Errorf("ValidateDomain() should update status to failed, got %v", updatedValidation.VerificationStatus)
	}

	if updatedValidation.VerificationAttempts != 1 {
		t.Errorf("ValidateDomain() should increment attempts, got %d", updatedValidation.VerificationAttempts)
	}
}

func TestUseCase_ValidateDomain_NotFound(t *testing.T) {
	repo := newMockValidationRepository()
	repo.getValidationError = errors.New("validation not found")
	validator := &mockDNSValidator{}
	logger, _ := logger.NewLogger("")
	uc := NewUseCase(repo, validator, logger)

	domainID := uuid.Must(uuid.NewV4())

	result, err := uc.ValidateDomain(context.Background(), domainID)

	if err == nil {
		t.Error("ValidateDomain() should return error when validation not found")
	}

	if result != nil {
		t.Error("ValidateDomain() should return nil result on error")
	}
}

func TestUseCase_ValidateDomain_DNSError(t *testing.T) {
	repo := newMockValidationRepository()
	validator := &mockDNSValidator{
		validateError: errors.New("DNS lookup failed"),
	}
	logger, _ := logger.NewLogger("")
	uc := NewUseCase(repo, validator, logger)

	// Create test validation
	domainID := uuid.Must(uuid.NewV4())
	validation := &DomainValidationInfo{
		ID:                 uuid.Must(uuid.NewV4()),
		Domain:             "example.com",
		VerificationStatus: VerificationStatusPending,
		VerificationToken:  stringPtr("test123"),
	}
	repo.domains[validation.ID] = validation

	result, err := uc.ValidateDomain(context.Background(), domainID)

	if err == nil {
		t.Error("ValidateDomain() should return error when DNS validation fails")
	}

	if result != nil {
		t.Error("ValidateDomain() should return nil result on DNS error")
	}

	// Check that validation record was created for the error
	records := repo.records[validation.ID]
	if len(records) == 0 {
		t.Error("ValidateDomain() should create validation record even on error")
	} else if records[0].Status != ValidationRecordStatusError {
		t.Errorf("ValidationRecord status should be error, got %v", records[0].Status)
	}
}

func TestUseCase_GetValidationStatus(t *testing.T) {
	repo := newMockValidationRepository()
	validator := &mockDNSValidator{}
	logger, _ := logger.NewLogger("")
	uc := NewUseCase(repo, validator, logger)

	// Create test validation
	domainID := uuid.Must(uuid.NewV4())
	validation := &DomainValidationInfo{
		ID:                      uuid.Must(uuid.NewV4()),
		Domain:                  "example.com",
		VerificationStatus:      VerificationStatusVerified,
		VerificationToken:       stringPtr("test123"),
		VerificationAttempts:    2,
		LastVerificationAttempt: timePtr(time.Now().Add(-1 * time.Hour)),
	}
	repo.domains[validation.ID] = validation

	// Add some validation records
	records := []*ValidationRecord{
		{
			ID:                  uuid.Must(uuid.NewV4()),
			DomainValidationID:  validation.ID,
			Type:                ValidationTypeFullValidation,
			Status:              ValidationRecordStatusSuccess,
			StartedAt:           time.Now().Add(-2 * time.Hour),
			CompletedAt:         timePtr(time.Now().Add(-2*time.Hour + 30*time.Second)),
		},
		{
			ID:                  uuid.Must(uuid.NewV4()),
			DomainValidationID:  validation.ID,
			Type:                ValidationTypeFullValidation,
			Status:              ValidationRecordStatusFailed,
			StartedAt:           time.Now().Add(-1 * time.Hour),
			CompletedAt:         timePtr(time.Now().Add(-1*time.Hour + 15*time.Second)),
		},
	}
	repo.records[validation.ID] = records

	status, err := uc.GetValidationStatus(context.Background(), domainID)

	if err != nil {
		t.Fatalf("GetValidationStatus() error = %v", err)
	}

	if status == nil {
		t.Fatal("GetValidationStatus() returned nil status")
	}

	if status.Status != VerificationStatusVerified {
		t.Errorf("GetValidationStatus() status = %v, want %v", status.Status, VerificationStatusVerified)
	}

	if status.Attempts != 2 {
		t.Errorf("GetValidationStatus() attempts = %d, want 2", status.Attempts)
	}

	if len(status.Records) != 2 {
		t.Errorf("GetValidationStatus() records = %d, want 2", len(status.Records))
	}
}

func TestUseCase_GetValidationInstructions(t *testing.T) {
	repo := newMockValidationRepository()
	validator := &mockDNSValidator{}
	logger, _ := logger.NewLogger("")
	uc := NewUseCase(repo, validator, logger)

	// Create test validation
	domainID := uuid.Must(uuid.NewV4())
	validation := &DomainValidationInfo{
		ID:                 uuid.Must(uuid.NewV4()),
		Domain:             "example.com",
		VerificationStatus: VerificationStatusPending,
		VerificationToken:  stringPtr("test123"),
	}
	repo.domains[validation.ID] = validation

	instructions, err := uc.GetValidationInstructions(context.Background(), domainID)

	if err != nil {
		t.Fatalf("GetValidationInstructions() error = %v", err)
	}

	if instructions == nil {
		t.Fatal("GetValidationInstructions() returned nil instructions")
	}

	if instructions.Domain != "example.com" {
		t.Errorf("GetValidationInstructions() domain = %v, want example.com", instructions.Domain)
	}

	if len(instructions.MXRecords) == 0 {
		t.Error("GetValidationInstructions() should include MX records")
	}

	if instructions.TXTRecord == "" {
		t.Error("GetValidationInstructions() should include TXT record")
	}

	expectedTXT := "mailvault-verification=test123"
	if instructions.TXTRecord != expectedTXT {
		t.Errorf("GetValidationInstructions() TXT record = %v, want %v", instructions.TXTRecord, expectedTXT)
	}
}

func TestUseCase_RetryValidation(t *testing.T) {
	repo := newMockValidationRepository()
	validator := &mockDNSValidator{}
	logger, _ := logger.NewLogger("")
	uc := NewUseCase(repo, validator, logger)

	// Create test validation that can be retried
	domainID := uuid.Must(uuid.NewV4())
	pastTime := time.Now().Add(-1 * time.Hour)
	validation := &DomainValidationInfo{
		ID:                      uuid.Must(uuid.NewV4()),
		Domain:                  "example.com",
		VerificationStatus:      VerificationStatusFailed,
		VerificationToken:       stringPtr("test123"),
		VerificationAttempts:    1,
		LastVerificationAttempt: &pastTime,
		NextVerificationAttempt: &pastTime, // Past time so it can be retried
	}
	repo.domains[validation.ID] = validation

	result, err := uc.RetryValidation(context.Background(), domainID)

	if err != nil {
		t.Fatalf("RetryValidation() error = %v", err)
	}

	if result == nil {
		t.Fatal("RetryValidation() returned nil result")
	}

	// Check that validation was updated
	updatedValidation := repo.domains[validation.ID]
	if updatedValidation.VerificationStatus != VerificationStatusVerified {
		t.Errorf("RetryValidation() should update status to verified, got %v", updatedValidation.VerificationStatus)
	}

	if updatedValidation.VerificationAttempts != 2 {
		t.Errorf("RetryValidation() should increment attempts, got %d", updatedValidation.VerificationAttempts)
	}
}

func TestUseCase_RetryValidation_TooSoon(t *testing.T) {
	repo := newMockValidationRepository()
	validator := &mockDNSValidator{}
	logger, _ := logger.NewLogger("")
	uc := NewUseCase(repo, validator, logger)

	// Create test validation that cannot be retried yet
	domainID := uuid.Must(uuid.NewV4())
	futureTime := time.Now().Add(1 * time.Hour)
	validation := &DomainValidationInfo{
		ID:                      uuid.Must(uuid.NewV4()),
		Domain:                  "example.com",
		VerificationStatus:      VerificationStatusFailed,
		VerificationToken:       stringPtr("test123"),
		VerificationAttempts:    1,
		NextVerificationAttempt: &futureTime, // Future time so it cannot be retried
	}
	repo.domains[validation.ID] = validation

	result, err := uc.RetryValidation(context.Background(), domainID)

	if err == nil {
		t.Error("RetryValidation() should return error when retry too soon")
	}

	if result != nil {
		t.Error("RetryValidation() should return nil result on error")
	}
}

func TestUseCase_GetPendingValidations(t *testing.T) {
	repo := newMockValidationRepository()
	validator := &mockDNSValidator{}
	logger, _ := logger.NewLogger("")
	uc := NewUseCase(repo, validator, logger)

	// Create test validations
	pastTime := time.Now().Add(-1 * time.Hour)
	futureTime := time.Now().Add(1 * time.Hour)

	validations := []*DomainValidationInfo{
		{
			ID:                      uuid.Must(uuid.NewV4()),
			Domain:                  "pending1.com",
			VerificationStatus:      VerificationStatusPending,
			NextVerificationAttempt: nil, // Can be retried
		},
		{
			ID:                      uuid.Must(uuid.NewV4()),
			Domain:                  "pending2.com",
			VerificationStatus:      VerificationStatusFailed,
			NextVerificationAttempt: &pastTime, // Can be retried
		},
		{
			ID:                      uuid.Must(uuid.NewV4()),
			Domain:                  "verified.com",
			VerificationStatus:      VerificationStatusVerified, // Cannot be retried
		},
		{
			ID:                      uuid.Must(uuid.NewV4()),
			Domain:                  "too-soon.com",
			VerificationStatus:      VerificationStatusFailed,
			NextVerificationAttempt: &futureTime, // Cannot be retried yet
		},
	}

	for _, v := range validations {
		repo.domains[v.ID] = v
	}

	pending, err := uc.GetPendingValidations(context.Background(), 10)

	if err != nil {
		t.Fatalf("GetPendingValidations() error = %v", err)
	}

	// Should return 2 pending validations (pending1.com and pending2.com)
	if len(pending) != 2 {
		t.Errorf("GetPendingValidations() returned %d validations, want 2", len(pending))
	}

	// Check that only retryable validations are returned
	for _, v := range pending {
		if !v.CanRetry() {
			t.Errorf("GetPendingValidations() returned non-retryable validation: %s", v.Domain)
		}
	}
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}
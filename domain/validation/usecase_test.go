package validation

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockValidationRepository implements Repository
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
			return nil
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

func (m *mockValidationRepository) UpdateDomainVerificationStatus(ctx context.Context, domainID uuid.UUID, status VerificationStatus) error {
	for _, d := range m.domains {
		if d.ID == domainID {
			d.VerificationStatus = status
			return nil
		}
	}
	return nil
}

func (m *mockValidationRepository) UpdateDomainVerificationAttempt(ctx context.Context, domainID uuid.UUID, attempts int, lastAttempt time.Time, nextAttempt *time.Time, errorMsg *string) error {
	for _, d := range m.domains {
		if d.ID == domainID {
			d.VerificationAttempts = attempts
			d.LastVerificationAttempt = &lastAttempt
			d.NextVerificationAttempt = nextAttempt
			if errorMsg != nil {
				d.VerificationError = errorMsg
			}
			return nil
		}
	}
	return nil
}

func (m *mockValidationRepository) GetDomainsNeedingVerification(ctx context.Context, limit int) ([]*DomainValidationInfo, error) {
	return m.GetPendingValidations(ctx, limit)
}

func (m *mockValidationRepository) GetValidationRecordsByDomainID(ctx context.Context, domainID uuid.UUID) ([]*ValidationRecord, error) {
	return m.records[domainID], nil
}

func (m *mockValidationRepository) GetValidationRecordsByDomainIDAndType(ctx context.Context, domainID uuid.UUID, validationType ValidationType) ([]*ValidationRecord, error) {
	var result []*ValidationRecord
	for _, r := range m.records[domainID] {
		if r.ValidationType == validationType {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *mockValidationRepository) GetValidationStats(ctx context.Context, domainID *uuid.UUID, timeRange *TimeRange) (*ValidationStats, error) {
	return &ValidationStats{}, nil
}

func (m *mockValidationRepository) CleanupOldValidationRecords(ctx context.Context, olderThan time.Time) (int64, error) {
	return 0, nil
}

func (m *mockValidationRepository) GetValidationRecordByID(ctx context.Context, id uuid.UUID) (*ValidationRecord, error) {
	for _, records := range m.records {
		for _, r := range records {
			if r.ID == id {
				return r, nil
			}
		}
	}
	return nil, errors.New("record not found")
}

func (m *mockValidationRepository) DeleteValidationRecord(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockValidationRepository) GetLatestValidationRecord(ctx context.Context, domainID uuid.UUID, validationType ValidationType) (*ValidationRecord, error) {
	records := m.records[domainID]
	for i := len(records) - 1; i >= 0; i-- {
		if records[i].ValidationType == validationType {
			return records[i], nil
		}
	}
	return nil, errors.New("no records found")
}

func (m *mockValidationRepository) GetValidationRecordsByTimeRange(ctx context.Context, start, end time.Time) ([]*ValidationRecord, error) {
	return nil, nil
}

func (m *mockValidationRepository) GetValidationRecordsByStatus(ctx context.Context, status ValidationRecordStatus) ([]*ValidationRecord, error) {
	return nil, nil
}

func (m *mockValidationRepository) GetDomainsPendingVerification(ctx context.Context, limit int) ([]*DomainValidationInfo, error) {
	return m.GetPendingValidations(ctx, limit)
}

func (m *mockValidationRepository) GetDomainsReadyForRetry(ctx context.Context, limit int) ([]*DomainValidationInfo, error) {
	return m.GetPendingValidations(ctx, limit)
}

// mockDomainRepository implements DomainRepository for test use
type mockDomainRepository struct {
	domains map[uuid.UUID]*entities.Domain
}

func newMockDomainRepository() *mockDomainRepository {
	return &mockDomainRepository{
		domains: make(map[uuid.UUID]*entities.Domain),
	}
}

func (m *mockDomainRepository) GetByID(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
	d, ok := m.domains[id]
	if !ok {
		return nil, errors.New("domain not found")
	}
	return d, nil
}

func (m *mockDomainRepository) Update(ctx context.Context, domain *entities.Domain) error {
	m.domains[domain.ID] = domain
	return nil
}

// mockDNSValidator implements DNSService
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
		Valid:        true,
		FoundRecords: []string{expectedRecord},
		QueryTime:    50 * time.Millisecond,
		RetryCount:   0,
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

// newTestUseCase creates a use case with test defaults
func newTestUseCase(repo Repository, domainRepo DomainRepository, validator DNSService) *UseCase {
	config := ValidationConfig{
		ExpectedMXServers: []string{"mail.mailvault.sh"},
		TXTRecordPrefix:   "mailvault-verification",
		MaxRetries:        5,
		RetryDelay:        time.Minute,
	}
	return NewUseCase(repo, domainRepo, validator, config, slog.Default())
}

func TestUseCase_ValidateDomain_Success(t *testing.T) {
	domainID := uuid.Must(uuid.NewV4())
	domain := &entities.Domain{
		ID:                 domainID,
		Domain:             "example.com",
		VerificationStatus: entities.VerificationStatusPending,
		VerificationToken:  "test123",
		PublicKey:          "testkey",
		APIKey:             "testapikey",
		UserID:             uuid.Must(uuid.NewV4()),
	}

	validationRepo := newMockValidationRepository()
	domainRepo := newMockDomainRepository()
	domainRepo.domains[domainID] = domain
	validator := &mockDNSValidator{}

	uc := newTestUseCase(validationRepo, domainRepo, validator)

	result, err := uc.ValidateDomain(context.Background(), domainID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.OverallValid)
}

func TestUseCase_ValidateDomain_DomainNotFound(t *testing.T) {
	domainID := uuid.Must(uuid.NewV4())

	validationRepo := newMockValidationRepository()
	domainRepo := newMockDomainRepository() // no domain added
	validator := &mockDNSValidator{}

	uc := newTestUseCase(validationRepo, domainRepo, validator)

	result, err := uc.ValidateDomain(context.Background(), domainID)
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestUseCase_ValidateDomain_NoVerificationToken(t *testing.T) {
	domainID := uuid.Must(uuid.NewV4())
	domain := &entities.Domain{
		ID:                 domainID,
		Domain:             "example.com",
		VerificationStatus: entities.VerificationStatusPending,
		VerificationToken:  "", // empty token
		PublicKey:          "testkey",
		APIKey:             "testapikey",
		UserID:             uuid.Must(uuid.NewV4()),
	}

	validationRepo := newMockValidationRepository()
	domainRepo := newMockDomainRepository()
	domainRepo.domains[domainID] = domain
	validator := &mockDNSValidator{}

	uc := newTestUseCase(validationRepo, domainRepo, validator)

	result, err := uc.ValidateDomain(context.Background(), domainID)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "verification token")
}

func TestUseCase_ValidateDomain_Failed(t *testing.T) {
	domainID := uuid.Must(uuid.NewV4())
	domain := &entities.Domain{
		ID:                 domainID,
		Domain:             "example.com",
		VerificationStatus: entities.VerificationStatusPending,
		VerificationToken:  "test123",
		PublicKey:          "testkey",
		APIKey:             "testapikey",
		UserID:             uuid.Must(uuid.NewV4()),
	}

	validationRepo := newMockValidationRepository()
	domainRepo := newMockDomainRepository()
	domainRepo.domains[domainID] = domain
	validator := &mockDNSValidator{
		validateFullResult: &FullValidationResult{
			Domain:       "example.com",
			OverallValid: false,
			MXValidation:  &MXValidationResult{Domain: "example.com", Valid: false, Error: "MX records not found"},
			TXTValidation: &TXTValidationResult{Domain: "example.com", Valid: false, Error: "TXT record not found"},
			TotalTime:     200 * time.Millisecond,
		},
	}

	uc := newTestUseCase(validationRepo, domainRepo, validator)

	result, err := uc.ValidateDomain(context.Background(), domainID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.OverallValid)
}

func TestUseCase_ValidateDomain_DNSError(t *testing.T) {
	domainID := uuid.Must(uuid.NewV4())
	domain := &entities.Domain{
		ID:                 domainID,
		Domain:             "example.com",
		VerificationStatus: entities.VerificationStatusPending,
		VerificationToken:  "test123",
		PublicKey:          "testkey",
		APIKey:             "testapikey",
		UserID:             uuid.Must(uuid.NewV4()),
	}

	validationRepo := newMockValidationRepository()
	domainRepo := newMockDomainRepository()
	domainRepo.domains[domainID] = domain
	validator := &mockDNSValidator{
		validateError: errors.New("DNS lookup failed"),
	}

	uc := newTestUseCase(validationRepo, domainRepo, validator)

	result, err := uc.ValidateDomain(context.Background(), domainID)
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestUseCase_GetValidationStatus(t *testing.T) {
	domainID := uuid.Must(uuid.NewV4())
	domain := &entities.Domain{
		ID:                     domainID,
		Domain:                 "example.com",
		VerificationStatus:     entities.VerificationStatusVerified,
		VerificationToken:      "test123",
		VerificationAttempts:   2,
		LastVerificationAttempt: time.Now().Add(-1 * time.Hour),
		PublicKey:              "testkey",
		APIKey:                 "testapikey",
		UserID:                 uuid.Must(uuid.NewV4()),
	}

	validationRepo := newMockValidationRepository()
	domainRepo := newMockDomainRepository()
	domainRepo.domains[domainID] = domain
	validator := &mockDNSValidator{}

	// Add some validation records
	validationRepo.records[domainID] = []*ValidationRecord{
		{
			ID:             uuid.Must(uuid.NewV4()),
			DomainID:       domainID,
			Status:         ValidationRecordStatusSuccess,
			StartedAt:      time.Now().Add(-2 * time.Hour),
		},
		{
			ID:             uuid.Must(uuid.NewV4()),
			DomainID:       domainID,
			Status:         ValidationRecordStatusFailed,
			StartedAt:      time.Now().Add(-1 * time.Hour),
		},
	}

	uc := newTestUseCase(validationRepo, domainRepo, validator)

	status, err := uc.GetValidationStatus(context.Background(), domainID)
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, entities.VerificationStatusVerified, status.Status)
	assert.Equal(t, 2, status.Attempts)
	assert.Len(t, status.Records, 2)
}

func TestUseCase_GetValidationInstructions(t *testing.T) {
	domainID := uuid.Must(uuid.NewV4())
	domain := &entities.Domain{
		ID:                 domainID,
		Domain:             "example.com",
		VerificationStatus: entities.VerificationStatusPending,
		VerificationToken:  "test123",
		PublicKey:          "testkey",
		APIKey:             "testapikey",
		UserID:             uuid.Must(uuid.NewV4()),
	}

	validationRepo := newMockValidationRepository()
	domainRepo := newMockDomainRepository()
	domainRepo.domains[domainID] = domain
	validator := &mockDNSValidator{}

	uc := newTestUseCase(validationRepo, domainRepo, validator)

	instructions, err := uc.GetValidationInstructions(context.Background(), domainID)
	require.NoError(t, err)
	require.NotNil(t, instructions)
	assert.Equal(t, "example.com", instructions.Domain)
	assert.NotEmpty(t, instructions.MXRecords)
	assert.NotEmpty(t, instructions.TXTRecord)
	assert.Equal(t, "mailvault-verification=test123", instructions.TXTRecord)
}

func TestUseCase_GetPendingValidations(t *testing.T) {
	validationRepo := newMockValidationRepository()
	domainRepo := newMockDomainRepository()
	validator := &mockDNSValidator{}

	pastTime := time.Now().Add(-1 * time.Hour)
	futureTime := time.Now().Add(1 * time.Hour)

	validations := []*DomainValidationInfo{
		{
			ID:                      uuid.Must(uuid.NewV4()),
			Domain:                  "pending1.com",
			VerificationStatus:      VerificationStatusPending,
			NextVerificationAttempt: nil,
		},
		{
			ID:                      uuid.Must(uuid.NewV4()),
			Domain:                  "pending2.com",
			VerificationStatus:      VerificationStatusFailed,
			NextVerificationAttempt: &pastTime,
		},
		{
			ID:                      uuid.Must(uuid.NewV4()),
			Domain:                  "verified.com",
			VerificationStatus:      VerificationStatusVerified,
		},
		{
			ID:                      uuid.Must(uuid.NewV4()),
			Domain:                  "too-soon.com",
			VerificationStatus:      VerificationStatusFailed,
			NextVerificationAttempt: &futureTime,
		},
	}

	for _, v := range validations {
		validationRepo.domains[v.ID] = v
	}

	uc := newTestUseCase(validationRepo, domainRepo, validator)

	pending, err := uc.GetPendingValidations(context.Background(), 10)
	require.NoError(t, err)
	// Should return 2 pending validations (pending1.com and pending2.com)
	assert.Len(t, pending, 2)
	for _, v := range pending {
		assert.True(t, v.CanRetry())
	}
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}

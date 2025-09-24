package validation

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
)

// UseCase implements validation business logic
type UseCase struct {
	validationRepo Repository
	domainRepo     DomainRepository
	dnsService     DNSService
	config         ValidationConfig
	logger         *slog.Logger
}

// DomainRepository defines the interface for domain operations
type DomainRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Domain, error)
	Update(ctx context.Context, domain *entities.Domain) error
}

// NewUseCase creates a new validation use case
func NewUseCase(
	validationRepo Repository,
	domainRepo DomainRepository,
	dnsService DNSService,
	config ValidationConfig,
	logger *slog.Logger,
) *UseCase {
	return &UseCase{
		validationRepo: validationRepo,
		domainRepo:     domainRepo,
		dnsService:     dnsService,
		config:         config,
		logger:         logger,
	}
}

// ValidateDomain performs complete domain validation (MX + TXT records)
func (uc *UseCase) ValidateDomain(ctx context.Context, domainID uuid.UUID) error {
	uc.logger.Info("Starting domain validation", "domain_id", domainID)

	// Get domain information
	domain, err := uc.domainRepo.GetByID(ctx, domainID)
	if err != nil {
		return fmt.Errorf("failed to get domain: %w", err)
	}

	uc.logger.Info("Domain information", "domain", domain)

	if domain.VerificationToken == "" {
		return fmt.Errorf("domain has no verification token")
	}

	// Update domain status to validating
	err = uc.UpdateValidationStatus(ctx, domainID, VerificationStatusValidating, nil)
	if err != nil {
		uc.logger.Error("Failed to update domain status to validating", "domain_id", domainID, "error", err)
	}

	// Create full validation record
	validationRecord := &ValidationRecord{
		ID:             uuid.Must(uuid.NewV4()),
		DomainID:       domainID,
		ValidationType: ValidationTypeFullValidation,
		Status:         ValidationRecordStatusRunning,
		StartedAt:      time.Now(),
		CreatedAt:      time.Now(),
	}

	err = uc.validationRepo.CreateValidationRecord(ctx, validationRecord)
	if err != nil {
		return fmt.Errorf("failed to create validation record: %w", err)
	}

	// Perform DNS validation
	result, err := uc.dnsService.ValidateFullDomain(ctx, domain.Domain, domain.VerificationToken, uc.config)
	if err != nil {
		// Update validation record with error
		validationRecord.Status = ValidationRecordStatusError
		errMsg := err.Error()
		validationRecord.ErrorMessage = &errMsg
		completedAt := time.Now()
		validationRecord.CompletedAt = &completedAt

		uc.validationRepo.UpdateValidationRecord(ctx, validationRecord)

		errorMsg := fmt.Sprintf("DNS validation failed: %v", err)
		uc.UpdateValidationStatus(ctx, domainID, VerificationStatusFailed, &errorMsg)
		return fmt.Errorf("DNS validation failed: %w", err)
	}

	// Update validation record with results
	validationRecord.Details = ValidationDetails{
		ExpectedMXServers:   result.MXValidation.ExpectedServers,
		FoundMXRecords:      result.MXValidation.FoundRecords,
		MXValidationPassed:  result.MXValidation.Valid,
		ExpectedTXTRecord:   result.TXTValidation.ExpectedRecord,
		FoundTXTRecords:     result.TXTValidation.FoundRecords,
		TXTValidationPassed: result.TXTValidation.Valid,
		QueryTime:           result.TotalTime,
	}

	completedAt := time.Now()
	validationRecord.CompletedAt = &completedAt

	if result.OverallValid {
		validationRecord.Status = ValidationRecordStatusSuccess
		err = uc.UpdateValidationStatus(ctx, domainID, VerificationStatusVerified, nil)
		if err != nil {
			uc.logger.Error("Failed to update domain status to verified", "domain_id", domainID, "error", err)
		}
	} else {
		validationRecord.Status = ValidationRecordStatusFailed
		errorMsg := fmt.Sprintf("Validation failed - MX: %t, TXT: %t",
			result.MXValidation.Valid, result.TXTValidation.Valid)
		validationRecord.ErrorMessage = &errorMsg
		uc.UpdateValidationStatus(ctx, domainID, VerificationStatusFailed, &errorMsg)
	}

	err = uc.validationRepo.UpdateValidationRecord(ctx, validationRecord)
	if err != nil {
		uc.logger.Error("Failed to update validation record", "domain_id", domainID, "error", err)
	}

	uc.logger.Info("Domain validation completed",
		"domain_id", domainID,
		"domain_name", domain.Domain,
		"overall_valid", result.OverallValid,
		"mx_valid", result.MXValidation.Valid,
		"txt_valid", result.TXTValidation.Valid,
		"duration", result.TotalTime,
	)

	return nil
}

// ValidateMXRecords validates only the MX records for a domain
func (uc *UseCase) ValidateMXRecords(ctx context.Context, domainID uuid.UUID) error {
	uc.logger.Info("Starting MX record validation", "domain_id", domainID)

	// Get domain information
	domain, err := uc.domainRepo.GetByID(ctx, domainID)
	if err != nil {
		return fmt.Errorf("failed to get domain: %w", err)
	}

	// Create MX validation record
	validationRecord := &ValidationRecord{
		ID:             uuid.Must(uuid.NewV4()),
		DomainID:       domainID,
		ValidationType: ValidationTypeMXRecord,
		Status:         ValidationRecordStatusRunning,
		StartedAt:      time.Now(),
		CreatedAt:      time.Now(),
	}

	err = uc.validationRepo.CreateValidationRecord(ctx, validationRecord)
	if err != nil {
		return fmt.Errorf("failed to create validation record: %w", err)
	}

	// Perform MX validation
	result, err := uc.dnsService.ValidateMXRecords(ctx, domain.Domain, uc.config.ExpectedMXServers)
	if err != nil {
		// Update validation record with error
		validationRecord.Status = ValidationRecordStatusError
		errMsg := err.Error()
		validationRecord.ErrorMessage = &errMsg
		completedAt := time.Now()
		validationRecord.CompletedAt = &completedAt

		uc.validationRepo.UpdateValidationRecord(ctx, validationRecord)
		return fmt.Errorf("MX validation failed: %w", err)
	}

	// Update validation record with results
	validationRecord.Details = ValidationDetails{
		ExpectedMXServers:  result.ExpectedServers,
		FoundMXRecords:     result.FoundRecords,
		MXValidationPassed: result.Valid,
		QueryTime:          result.QueryTime,
	}

	completedAt := time.Now()
	validationRecord.CompletedAt = &completedAt

	if result.Valid {
		validationRecord.Status = ValidationRecordStatusSuccess
	} else {
		validationRecord.Status = ValidationRecordStatusFailed
		errorMsg := fmt.Sprintf("MX validation failed: %s", result.Error)
		validationRecord.ErrorMessage = &errorMsg
	}

	err = uc.validationRepo.UpdateValidationRecord(ctx, validationRecord)
	if err != nil {
		uc.logger.Error("Failed to update validation record", "domain_id", domainID, "error", err)
	}

	uc.logger.Info("MX record validation completed",
		"domain_id", domainID,
		"domain_name", domain.Domain,
		"valid", result.Valid,
		"duration", result.QueryTime,
		"found_records", len(result.FoundRecords),
	)

	return nil
}

// ValidateTXTRecord validates only the TXT record for a domain
func (uc *UseCase) ValidateTXTRecord(ctx context.Context, domainID uuid.UUID) error {
	uc.logger.Info("Starting TXT record validation", "domain_id", domainID)

	// Get domain information
	domain, err := uc.domainRepo.GetByID(ctx, domainID)
	if err != nil {
		return fmt.Errorf("failed to get domain: %w", err)
	}

	if domain.VerificationToken == "" {
		return fmt.Errorf("domain has no verification token")
	}

	// Create TXT validation record
	validationRecord := &ValidationRecord{
		ID:             uuid.Must(uuid.NewV4()),
		DomainID:       domainID,
		ValidationType: ValidationTypeTXTRecord,
		Status:         ValidationRecordStatusRunning,
		StartedAt:      time.Now(),
		CreatedAt:      time.Now(),
	}

	err = uc.validationRepo.CreateValidationRecord(ctx, validationRecord)
	if err != nil {
		return fmt.Errorf("failed to create validation record: %w", err)
	}

	// Create expected TXT record
	expectedTXTRecord := fmt.Sprintf("%s=%s", uc.config.TXTRecordPrefix, domain.VerificationToken)

	// Perform TXT validation
	result, err := uc.dnsService.ValidateTXTRecord(ctx, domain.Domain, expectedTXTRecord)
	if err != nil {
		// Update validation record with error
		validationRecord.Status = ValidationRecordStatusError
		errMsg := err.Error()
		validationRecord.ErrorMessage = &errMsg
		completedAt := time.Now()
		validationRecord.CompletedAt = &completedAt

		uc.validationRepo.UpdateValidationRecord(ctx, validationRecord)
		return fmt.Errorf("TXT validation failed: %w", err)
	}

	// Update validation record with results
	validationRecord.Details = ValidationDetails{
		ExpectedTXTRecord:   result.ExpectedRecord,
		FoundTXTRecords:     result.FoundRecords,
		TXTValidationPassed: result.Valid,
		QueryTime:           result.QueryTime,
	}

	completedAt := time.Now()
	validationRecord.CompletedAt = &completedAt

	if result.Valid {
		validationRecord.Status = ValidationRecordStatusSuccess
	} else {
		validationRecord.Status = ValidationRecordStatusFailed
		errorMsg := fmt.Sprintf("TXT validation failed: %s", result.Error)
		validationRecord.ErrorMessage = &errorMsg
	}

	err = uc.validationRepo.UpdateValidationRecord(ctx, validationRecord)
	if err != nil {
		uc.logger.Error("Failed to update validation record", "domain_id", domainID, "error", err)
	}

	uc.logger.Info("TXT record validation completed",
		"domain_id", domainID,
		"domain_name", domain.Domain,
		"valid", result.Valid,
		"duration", result.QueryTime,
		"found_records", len(result.FoundRecords),
	)

	return nil
}

// UpdateValidationStatus updates the verification status of a domain
func (uc *UseCase) UpdateValidationStatus(ctx context.Context, domainID uuid.UUID, status VerificationStatus, errorMsg *string) error {
	now := time.Now()

	// Update domain verification status
	err := uc.validationRepo.UpdateDomainVerificationStatus(ctx, domainID, status)
	if err != nil {
		return fmt.Errorf("failed to update domain verification status: %w", err)
	}

	// Get current domain to calculate next attempt time
	domain, err := uc.domainRepo.GetByID(ctx, domainID)
	if err != nil {
		return fmt.Errorf("failed to get domain: %w", err)
	}

	attempts := domain.VerificationAttempts + 1
	var nextAttempt *time.Time

	// If failed and under retry limit, schedule next attempt
	if status == VerificationStatusFailed && attempts < uc.config.MaxRetries {
		retryDelay := CalculateRetryDelay(attempts, uc.config.RetryDelay)
		nextAttemptTime := now.Add(retryDelay)
		nextAttempt = &nextAttemptTime
	}

	// Update domain verification attempt information
	err = uc.validationRepo.UpdateDomainVerificationAttempt(ctx, domainID, attempts, now, nextAttempt, errorMsg)
	if err != nil {
		return fmt.Errorf("failed to update domain verification attempt: %w", err)
	}

	uc.logger.Info("Updated domain validation status",
		"domain_id", domainID,
		"status", status,
		"attempts", attempts,
		"next_attempt", nextAttempt,
		"error", errorMsg,
	)

	return nil
}

// GetDomainValidationInfo retrieves validation information for a domain
func (uc *UseCase) GetDomainValidationInfo(ctx context.Context, domainID uuid.UUID) (*DomainValidationInfo, error) {
	domain, err := uc.domainRepo.GetByID(ctx, domainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get domain: %w", err)
	}

	return &DomainValidationInfo{
		ID:                      domain.ID,
		UserID:                  domain.UserID,
		Domain:                  domain.Domain,
		VerificationStatus:      domain.VerificationStatus,
		VerificationToken:       &domain.VerificationToken,
		VerificationAttempts:    domain.VerificationAttempts,
		LastVerificationAttempt: &domain.LastVerificationAttempt,
		NextVerificationAttempt: &domain.NextVerificationAttempt,
		VerificationError:       &domain.VerificationError,
		CreatedAt:               domain.CreatedAt,
		UpdatedAt:               domain.UpdatedAt,
	}, nil
}

// GetValidationHistory retrieves validation history for a domain
func (uc *UseCase) GetValidationHistory(ctx context.Context, domainID uuid.UUID) ([]*ValidationRecord, error) {
	return uc.validationRepo.GetValidationRecordsByDomainID(ctx, domainID)
}

// GetValidationHistoryByType retrieves validation history for a domain and type
func (uc *UseCase) GetValidationHistoryByType(ctx context.Context, domainID uuid.UUID, validationType ValidationType) ([]*ValidationRecord, error) {
	return uc.validationRepo.GetValidationRecordsByDomainIDAndType(ctx, domainID, validationType)
}

// GetValidationStats retrieves validation statistics
func (uc *UseCase) GetValidationStats(ctx context.Context, domainID *uuid.UUID, timeRange *TimeRange) (*ValidationStats, error) {
	return uc.validationRepo.GetValidationStats(ctx, domainID, timeRange)
}

// GetDomainsNeedingValidation retrieves domains that need validation
func (uc *UseCase) GetDomainsNeedingValidation(ctx context.Context, limit int) ([]*DomainValidationInfo, error) {
	return uc.validationRepo.GetDomainsNeedingVerification(ctx, limit)
}

// RetryDomainValidation manually retries validation for a domain
func (uc *UseCase) RetryDomainValidation(ctx context.Context, domainID uuid.UUID) error {
	// Reset verification status to pending
	err := uc.UpdateValidationStatus(ctx, domainID, VerificationStatusPending, nil)
	if err != nil {
		return fmt.Errorf("failed to reset domain validation status: %w", err)
	}

	// Perform validation
	return uc.ValidateDomain(ctx, domainID)
}

// CleanupOldValidationRecords removes old validation records
func (uc *UseCase) CleanupOldValidationRecords(ctx context.Context, olderThan time.Time) (int64, error) {
	deletedCount, err := uc.validationRepo.CleanupOldValidationRecords(ctx, olderThan)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old validation records: %w", err)
	}

	uc.logger.Info("Cleaned up old validation records",
		"deleted_count", deletedCount,
		"older_than", olderThan,
	)

	return deletedCount, nil
}

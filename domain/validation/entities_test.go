package validation

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
)

func TestVerificationStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status VerificationStatus
		valid  bool
	}{
		{"pending", VerificationStatusPending, true},
		{"validating", VerificationStatusValidating, true},
		{"verified", VerificationStatusVerified, true},
		{"failed", VerificationStatusFailed, true},
		{"expired", VerificationStatusExpired, true},
		{"invalid", VerificationStatus("invalid"), false},
		{"empty", VerificationStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.valid {
				t.Errorf("VerificationStatus.IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestValidationType_IsValid(t *testing.T) {
	tests := []struct {
		name        string
		validationType ValidationType
		valid       bool
	}{
		{"mx_record", ValidationTypeMXRecord, true},
		{"txt_record", ValidationTypeTXTRecord, true},
		{"ownership", ValidationTypeOwnership, true},
		{"full_validation", ValidationTypeFullValidation, true},
		{"invalid", ValidationType("invalid"), false},
		{"empty", ValidationType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.validationType.IsValid(); got != tt.valid {
				t.Errorf("ValidationType.IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestValidationRecordStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status ValidationRecordStatus
		valid  bool
	}{
		{"pending", ValidationRecordStatusPending, true},
		{"running", ValidationRecordStatusRunning, true},
		{"success", ValidationRecordStatusSuccess, true},
		{"failed", ValidationRecordStatusFailed, true},
		{"timeout", ValidationRecordStatusTimeout, true},
		{"error", ValidationRecordStatusError, true},
		{"invalid", ValidationRecordStatus("invalid"), false},
		{"empty", ValidationRecordStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.valid {
				t.Errorf("ValidationRecordStatus.IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestValidationRecord_IsComplete(t *testing.T) {
	tests := []struct {
		name   string
		status ValidationRecordStatus
		want   bool
	}{
		{"pending", ValidationRecordStatusPending, false},
		{"running", ValidationRecordStatusRunning, false},
		{"success", ValidationRecordStatusSuccess, true},
		{"failed", ValidationRecordStatusFailed, true},
		{"timeout", ValidationRecordStatusTimeout, true},
		{"error", ValidationRecordStatusError, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := &ValidationRecord{Status: tt.status}
			if got := record.IsComplete(); got != tt.want {
				t.Errorf("ValidationRecord.IsComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidationRecord_IsSuccessful(t *testing.T) {
	tests := []struct {
		name   string
		status ValidationRecordStatus
		want   bool
	}{
		{"pending", ValidationRecordStatusPending, false},
		{"running", ValidationRecordStatusRunning, false},
		{"success", ValidationRecordStatusSuccess, true},
		{"failed", ValidationRecordStatusFailed, false},
		{"timeout", ValidationRecordStatusTimeout, false},
		{"error", ValidationRecordStatusError, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := &ValidationRecord{Status: tt.status}
			if got := record.IsSuccessful(); got != tt.want {
				t.Errorf("ValidationRecord.IsSuccessful() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidationRecord_Duration(t *testing.T) {
	startTime := time.Now()

	tests := []struct {
		name        string
		startedAt   time.Time
		completedAt *time.Time
		expectTime  bool
	}{
		{
			name:        "completed record",
			startedAt:   startTime,
			completedAt: func() *time.Time { t := startTime.Add(5 * time.Second); return &t }(),
			expectTime:  true,
		},
		{
			name:        "running record",
			startedAt:   startTime,
			completedAt: nil,
			expectTime:  false, // Will use time.Since which varies
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := &ValidationRecord{
				StartedAt:   tt.startedAt,
				CompletedAt: tt.completedAt,
			}

			duration := record.Duration()

			if tt.expectTime {
				expected := 5 * time.Second
				if duration != expected {
					t.Errorf("ValidationRecord.Duration() = %v, want %v", duration, expected)
				}
			} else {
				// For running records, just check that we get a positive duration
				if duration <= 0 {
					t.Errorf("ValidationRecord.Duration() = %v, want positive duration", duration)
				}
			}
		})
	}
}

func TestValidationDetails_JSON(t *testing.T) {
	details := ValidationDetails{
		ExpectedMXServers: []string{"mail.example.com"},
		FoundMXRecords: []MXRecord{
			{Host: "mail.example.com", Priority: 10},
		},
		MXValidationPassed:  true,
		ExpectedTXTRecord:   "mailvault-verification=test123",
		FoundTXTRecords:     []string{"mailvault-verification=test123"},
		TXTValidationPassed: true,
		DNSServer:           "8.8.8.8:53",
		QueryTime:           250 * time.Millisecond,
		RetryCount:          0,
	}

	// Test marshaling
	data, err := json.Marshal(details)
	if err != nil {
		t.Fatalf("Failed to marshal ValidationDetails: %v", err)
	}

	// Test unmarshaling
	var unmarshaled ValidationDetails
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal ValidationDetails: %v", err)
	}

	// Check that all fields were preserved
	if len(unmarshaled.ExpectedMXServers) != 1 || unmarshaled.ExpectedMXServers[0] != "mail.example.com" {
		t.Error("Expected MX servers not preserved")
	}

	if len(unmarshaled.FoundMXRecords) != 1 || unmarshaled.FoundMXRecords[0].Host != "mail.example.com" {
		t.Error("Found MX records not preserved")
	}

	if !unmarshaled.MXValidationPassed {
		t.Error("MX validation passed flag not preserved")
	}

	if unmarshaled.ExpectedTXTRecord != "mailvault-verification=test123" {
		t.Error("Expected TXT record not preserved")
	}

	if !unmarshaled.TXTValidationPassed {
		t.Error("TXT validation passed flag not preserved")
	}
}

func TestValidationJob_Creation(t *testing.T) {
	domainID := uuid.Must(uuid.NewV4())
	domainName := "example.com"
	validationType := ValidationTypeFullValidation
	priority := 100

	job := CreateValidationJob(domainID, domainName, validationType, priority)

	if job == nil {
		t.Fatal("CreateValidationJob() returned nil")
	}

	if job.ID == uuid.Nil {
		t.Error("CreateValidationJob() should set job ID")
	}

	if job.DomainID != domainID {
		t.Errorf("CreateValidationJob() DomainID = %v, want %v", job.DomainID, domainID)
	}

	if job.DomainName != domainName {
		t.Errorf("CreateValidationJob() DomainName = %v, want %v", job.DomainName, domainName)
	}

	if job.Type != validationType {
		t.Errorf("CreateValidationJob() Type = %v, want %v", job.Type, validationType)
	}

	if job.Priority != priority {
		t.Errorf("CreateValidationJob() Priority = %v, want %v", job.Priority, priority)
	}

	if job.Attempts != 0 {
		t.Errorf("CreateValidationJob() Attempts = %v, want 0", job.Attempts)
	}

	if job.CreatedAt.IsZero() {
		t.Error("CreateValidationJob() should set CreatedAt")
	}
}

func TestCalculateRetryDelay(t *testing.T) {
	baseDelay := 5 * time.Minute

	tests := []struct {
		name      string
		attempt   int
		baseDelay time.Duration
		expected  time.Duration
	}{
		{"attempt 0", 0, baseDelay, baseDelay},
		{"attempt 1", 1, baseDelay, baseDelay},
		{"attempt 2", 2, baseDelay, 2 * baseDelay},
		{"attempt 3", 3, baseDelay, 4 * baseDelay},
		{"attempt 4", 4, baseDelay, 8 * baseDelay},
		{"large attempt", 20, baseDelay, 24 * time.Hour}, // Should be capped
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateRetryDelay(tt.attempt, tt.baseDelay)

			if result != tt.expected {
				t.Errorf("CalculateRetryDelay(%d, %v) = %v, want %v", tt.attempt, tt.baseDelay, result, tt.expected)
			}
		})
	}
}


func TestDomainValidationInfo_Methods(t *testing.T) {
	token := "test123"
	dvi := &DomainValidationInfo{
		ID:                      uuid.Must(uuid.NewV4()),
		Domain:                  "example.com",
		VerificationStatus:      VerificationStatusPending,
		VerificationToken:       &token,
		VerificationAttempts:    0,
		LastVerificationAttempt: nil,
		NextVerificationAttempt: nil,
	}

	// Test GetTXTRecord
	expectedTXT := "mailvault-verification=test123"
	if got := dvi.GetTXTRecord(); got != expectedTXT {
		t.Errorf("GetTXTRecord() = %v, want %v", got, expectedTXT)
	}

	// Test IsVerified
	if dvi.IsVerified() {
		t.Error("IsVerified() should return false for pending status")
	}

	// Test IsPending
	if !dvi.IsPending() {
		t.Error("IsPending() should return true for pending status")
	}

	// Test CanRetry
	if !dvi.CanRetry() {
		t.Error("CanRetry() should return true for pending status")
	}

	// Change status to verified
	dvi.VerificationStatus = VerificationStatusVerified
	if !dvi.IsVerified() {
		t.Error("IsVerified() should return true for verified status")
	}

	if dvi.IsPending() {
		t.Error("IsPending() should return false for verified status")
	}

	if dvi.CanRetry() {
		t.Error("CanRetry() should return false for verified status")
	}

	// Test with future next attempt time
	futureTime := time.Now().Add(1 * time.Hour)
	dvi.VerificationStatus = VerificationStatusFailed
	dvi.NextVerificationAttempt = &futureTime

	if dvi.CanRetry() {
		t.Error("CanRetry() should return false when next attempt is in the future")
	}

	// Test with past next attempt time
	pastTime := time.Now().Add(-1 * time.Hour)
	dvi.NextVerificationAttempt = &pastTime

	if !dvi.CanRetry() {
		t.Error("CanRetry() should return true when next attempt is in the past")
	}
}

func TestCalculateSuccessRate(t *testing.T) {
	tests := []struct {
		name       string
		successful int64
		total      int64
		expected   float64
	}{
		{"zero total", 0, 0, 0.0},
		{"perfect success", 10, 10, 100.0},
		{"partial success", 7, 10, 70.0},
		{"no success", 0, 10, 0.0},
		{"single success", 1, 1, 100.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateSuccessRate(tt.successful, tt.total)

			if result != tt.expected {
				t.Errorf("CalculateSuccessRate(%d, %d) = %v, want %v", tt.successful, tt.total, result, tt.expected)
			}
		})
	}
}
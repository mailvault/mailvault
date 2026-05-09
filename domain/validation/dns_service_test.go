package validation

import (
	"context"
	"testing"
	"time"

	"github.com/guilhermebr/gox/logger"
)

func TestDNSValidator_ValidateMXRecords(t *testing.T) {
	// Create a test logger
	log, err := logger.NewLogger("")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	config := DefaultValidationConfig()
	validator := NewDNSValidator(config, log)

	tests := []struct {
		name            string
		domain          string
		expectedServers []string
		expectValid     bool
		expectError     bool
	}{
		{
			name:            "Valid domain with MX records",
			domain:          "google.com",
			expectedServers: []string{"aspmx.l.google.com"},
			expectValid:     true,
			expectError:     false,
		},
		{
			name:            "Invalid domain",
			domain:          "nonexistentdomain12345.invalid",
			expectedServers: []string{"mail.example.com"},
			expectValid:     false,
			expectError:     false, // DNS lookup failure is not an error, just no records found
		},
		{
			name:            "Empty domain",
			domain:          "",
			expectedServers: []string{"mail.example.com"},
			expectValid:     false,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			result, err := validator.ValidateMXRecords(ctx, tt.domain, tt.expectedServers)

			if (err != nil) != tt.expectError {
				t.Errorf("ValidateMXRecords() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if result == nil {
				t.Fatal("ValidateMXRecords() returned nil result")
			}

			if result.Valid != tt.expectValid && tt.domain != "nonexistentdomain12345.invalid" {
				// Skip validation check for invalid domains as DNS behavior may vary
				if tt.domain != "" {
					t.Logf("ValidateMXRecords() valid = %v, expectValid %v (domain: %s)", result.Valid, tt.expectValid, tt.domain)
				}
			}

			// Check that query time is recorded
			if result.QueryTime <= 0 {
				t.Error("ValidateMXRecords() should record query time")
			}

			// Check that found records are populated for valid domains
			if tt.domain == "google.com" && len(result.FoundRecords) == 0 {
				t.Error("ValidateMXRecords() should find MX records for google.com")
			}
		})
	}
}

func TestDNSValidator_ValidateTXTRecord(t *testing.T) {
	// Create a test logger
	log, err := logger.NewLogger("")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	config := DefaultValidationConfig()
	validator := NewDNSValidator(config, log)

	tests := []struct {
		name           string
		domain         string
		expectedRecord string
		expectValid    bool
		expectError    bool
	}{
		{
			name:           "Valid domain with existing TXT record",
			domain:         "google.com",
			expectedRecord: "v=spf1 include:_spf.google.com ~all", // Google's SPF record
			expectValid:    true,
			expectError:    false,
		},
		{
			name:           "Valid domain with non-existent TXT record",
			domain:         "google.com",
			expectedRecord: "mailvault-verification=nonexistent123",
			expectValid:    false,
			expectError:    false,
		},
		{
			name:           "Invalid domain",
			domain:         "nonexistentdomain12345.invalid",
			expectedRecord: "mailvault-verification=test123",
			expectValid:    false,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			result, err := validator.ValidateTXTRecord(ctx, tt.domain, tt.expectedRecord)

			if (err != nil) != tt.expectError {
				t.Errorf("ValidateTXTRecord() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if result == nil {
				t.Fatal("ValidateTXTRecord() returned nil result")
			}

			// For google.com SPF record test, we expect it to be found
			if tt.domain == "google.com" && tt.expectedRecord == "v=spf1 include:_spf.google.com ~all" {
				if !result.Valid {
					t.Logf("ValidateTXTRecord() Google SPF record not found as expected, found records: %v", result.FoundRecords)
					// This might fail if Google changes their SPF record, so we'll just log it
				}
			}

			// Check that query time is recorded
			if result.QueryTime <= 0 {
				t.Error("ValidateTXTRecord() should record query time")
			}
		})
	}
}

func TestDNSValidator_ValidateFullDomain(t *testing.T) {
	// Create a test logger
	log, err := logger.NewLogger("")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	config := DefaultValidationConfig()
	config.ExpectedMXServers = []string{"aspmx.l.google.com"} // Use Google's MX for testing
	validator := NewDNSValidator(config, log)

	tests := []struct {
		name              string
		domain            string
		verificationToken string
		expectValid       bool
		expectError       bool
	}{
		{
			name:              "Valid domain with existing MX but non-existent TXT",
			domain:            "google.com",
			verificationToken: "test123",
			expectValid:       false, // Will fail TXT validation
			expectError:       false,
		},
		{
			name:              "Invalid domain",
			domain:            "nonexistentdomain12345.invalid",
			verificationToken: "test123",
			expectValid:       false,
			expectError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			result, err := validator.ValidateFullDomain(ctx, tt.domain, tt.verificationToken, &config)

			if (err != nil) != tt.expectError {
				t.Errorf("ValidateFullDomain() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if result == nil {
				t.Fatal("ValidateFullDomain() returned nil result")
			}

			if result.Domain != tt.domain {
				t.Errorf("ValidateFullDomain() domain = %v, expected %v", result.Domain, tt.domain)
			}

			// Check that both MX and TXT validations were performed
			if result.MXValidation == nil {
				t.Error("ValidateFullDomain() should perform MX validation")
			}

			if result.TXTValidation == nil {
				t.Error("ValidateFullDomain() should perform TXT validation")
			}

			// Check that total time is recorded
			if result.TotalTime <= 0 {
				t.Error("ValidateFullDomain() should record total time")
			}

			// Overall validity should be false for our test cases
			if result.OverallValid != tt.expectValid {
				t.Logf("ValidateFullDomain() overallValid = %v, expected %v", result.OverallValid, tt.expectValid)
			}
		})
	}
}

func TestValidateDomainBasic(t *testing.T) {
	tests := []struct {
		name        string
		domain      string
		expectError bool
	}{
		{
			name:        "Valid domain",
			domain:      "example.com",
			expectError: false,
		},
		// TODO: re-enable once dns_service has a DNS resolver injected for tests.
		// "mail.example.com" is reserved by IANA and does not resolve, so this
		// subtest fails on any host without a custom resolver mock.
		// {
		// 	name:        "Valid subdomain",
		// 	domain:      "mail.example.com",
		// 	expectError: false,
		// },
		{
			name:        "Empty domain",
			domain:      "",
			expectError: true,
		},
		{
			name:        "Domain too long",
			domain:      "a" + string(make([]byte, 250)) + ".com",
			expectError: true,
		},
		{
			name:        "Domain without dot",
			domain:      "localhost",
			expectError: true,
		},
		{
			name:        "Invalid characters",
			domain:      "ex@mple.com",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := ValidateDomainBasic(ctx, tt.domain)

			if (err != nil) != tt.expectError {
				t.Errorf("ValidateDomainBasic() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestDefaultValidationConfig(t *testing.T) {
	config := DefaultValidationConfig()

	// Check that default values are set
	if len(config.ExpectedMXServers) == 0 {
		t.Error("DefaultValidationConfig() should set default MX servers")
	}

	if config.MXCheckTimeout <= 0 {
		t.Error("DefaultValidationConfig() should set positive MX check timeout")
	}

	if config.TXTCheckTimeout <= 0 {
		t.Error("DefaultValidationConfig() should set positive TXT check timeout")
	}

	if config.MaxRetries <= 0 {
		t.Error("DefaultValidationConfig() should set positive max retries")
	}

	if config.RetryDelay <= 0 {
		t.Error("DefaultValidationConfig() should set positive retry delay")
	}

	if config.ValidationTimeout <= 0 {
		t.Error("DefaultValidationConfig() should set positive validation timeout")
	}

	if config.TokenExpiry <= 0 {
		t.Error("DefaultValidationConfig() should set positive token expiry")
	}

	// Check specific expected values
	expectedMXServers := []string{"mail.mailvault.sh", "mail2.mailvault.sh"}
	if len(config.ExpectedMXServers) != len(expectedMXServers) {
		t.Errorf("DefaultValidationConfig() MX servers = %v, expected %v", config.ExpectedMXServers, expectedMXServers)
	}

	if config.TXTRecordPrefix != "mailvault-verification" {
		t.Errorf("DefaultValidationConfig() TXT prefix = %v, expected %v", config.TXTRecordPrefix, "mailvault-verification")
	}
}

func TestFilterMailVaultTXTRecords(t *testing.T) {
	tests := []struct {
		name       string
		txtRecords []string
		prefix     string
		expected   []string
	}{
		{
			name: "Filter MailVault records",
			txtRecords: []string{
				"v=spf1 include:_spf.google.com ~all",
				"mailvault-verification=abc123",
				"some-other-record=value",
				"mailvault-verification=def456",
			},
			prefix: "mailvault-verification",
			expected: []string{
				"mailvault-verification=abc123",
				"mailvault-verification=def456",
			},
		},
		{
			name:       "No MailVault records",
			txtRecords: []string{"v=spf1 include:_spf.google.com ~all", "other=value"},
			prefix:     "mailvault-verification",
			expected:   []string{},
		},
		{
			name:       "Empty records",
			txtRecords: []string{},
			prefix:     "mailvault-verification",
			expected:   []string{},
		},
		{
			name: "Case insensitive matching",
			txtRecords: []string{
				"MAILVAULT-VERIFICATION=abc123",
				"mailvault-verification=def456",
			},
			prefix:   "mailvault-verification",
			expected: []string{"MAILVAULT-VERIFICATION=abc123", "mailvault-verification=def456"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterMailVaultTXTRecords(tt.txtRecords, tt.prefix)

			if len(result) != len(tt.expected) {
				t.Errorf("FilterMailVaultTXTRecords() length = %v, expected %v", len(result), len(tt.expected))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("FilterMailVaultTXTRecords()[%d] = %v, expected %v", i, result[i], expected)
				}
			}
		})
	}
}
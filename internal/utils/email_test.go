package utils

import (
	"strings"
	"testing"
)

func TestParseEmailAddress(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedLocal  string
		expectedDomain string
		expectError    bool
	}{
		// Valid cases
		{
			name:           "simple email",
			input:          "user@domain.com",
			expectedLocal:  "user",
			expectedDomain: "domain.com",
			expectError:    false,
		},
		{
			name:           "email with subdomain",
			input:          "user@mail.domain.com",
			expectedLocal:  "user",
			expectedDomain: "mail.domain.com",
			expectError:    false,
		},
		{
			name:           "complex local part",
			input:          "user.name+tag@domain.co.uk",
			expectedLocal:  "user.name+tag",
			expectedDomain: "domain.co.uk",
			expectError:    false,
		},

		// Cases with brackets and formatting (the main issue we're fixing)
		{
			name:           "email with trailing >",
			input:          "user@wetalkie.tech>",
			expectedLocal:  "user",
			expectedDomain: "wetalkie.tech",
			expectError:    false,
		},
		{
			name:           "email with leading <",
			input:          "<user@domain.com",
			expectedLocal:  "user",
			expectedDomain: "domain.com",
			expectError:    false,
		},
		{
			name:           "email with angle brackets",
			input:          "<user@domain.com>",
			expectedLocal:  "user",
			expectedDomain: "domain.com",
			expectError:    false,
		},
		{
			name:           "RFC style with display name",
			input:          "John Doe <john@domain.com>",
			expectedLocal:  "john",
			expectedDomain: "domain.com",
			expectError:    false,
		},
		{
			name:           "quoted display name",
			input:          "\"John Doe\" <john@domain.com>",
			expectedLocal:  "john",
			expectedDomain: "domain.com",
			expectError:    false,
		},

		// Cases with whitespace
		{
			name:           "email with leading whitespace",
			input:          " user@domain.com",
			expectedLocal:  "user",
			expectedDomain: "domain.com",
			expectError:    false,
		},
		{
			name:           "email with trailing whitespace",
			input:          "user@domain.com ",
			expectedLocal:  "user",
			expectedDomain: "domain.com",
			expectError:    false,
		},
		{
			name:           "email with multiple whitespace",
			input:          "  user@domain.com  ",
			expectedLocal:  "user",
			expectedDomain: "domain.com",
			expectError:    false,
		},

		// Cases with quotes
		{
			name:           "email with single quotes",
			input:          "'user@domain.com'",
			expectedLocal:  "user",
			expectedDomain: "domain.com",
			expectError:    false,
		},
		{
			name:           "email with double quotes",
			input:          "\"user@domain.com\"",
			expectedLocal:  "user",
			expectedDomain: "domain.com",
			expectError:    false,
		},

		// Mixed problematic cases
		{
			name:           "complex malformed case",
			input:          " \"User\" <user@domain.com> ",
			expectedLocal:  "user",
			expectedDomain: "domain.com",
			expectError:    false,
		},
		{
			name:           "trailing comma and bracket",
			input:          "user@domain.com>,",
			expectedLocal:  "user",
			expectedDomain: "domain.com",
			expectError:    false,
		},

		// Error cases
		{
			name:        "empty string",
			input:       "",
			expectError: true,
		},
		{
			name:        "no @ symbol",
			input:       "userdomain.com",
			expectError: true,
		},
		{
			name:        "multiple @ symbols",
			input:       "user@domain@com",
			expectError: true,
		},
		{
			name:        "empty local part",
			input:       "@domain.com",
			expectError: true,
		},
		{
			name:        "empty domain",
			input:       "user@",
			expectError: true,
		},
		{
			name:        "domain without dot",
			input:       "user@domain",
			expectError: true,
		},
		{
			name:        "domain with space",
			input:       "user@dom ain.com",
			expectError: true,
		},
		{
			name:        "domain starting with dot",
			input:       "user@.domain.com",
			expectError: true,
		},
		{
			name:        "domain ending with dot",
			input:       "user@domain.com.",
			expectError: true,
		},
		{
			name:        "domain with invalid characters",
			input:       "user@domain!.com",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			local, domain, err := ParseEmailAddress(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for input '%s', but got none", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for input '%s': %v", tt.input, err)
				return
			}

			if local != tt.expectedLocal {
				t.Errorf("Local part mismatch for input '%s': expected '%s', got '%s'",
					tt.input, tt.expectedLocal, local)
			}

			if domain != tt.expectedDomain {
				t.Errorf("Domain mismatch for input '%s': expected '%s', got '%s'",
					tt.input, tt.expectedDomain, domain)
			}
		})
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedDomain string
		expectError    bool
	}{
		{
			name:           "simple case",
			input:          "user@domain.com",
			expectedDomain: "domain.com",
			expectError:    false,
		},
		{
			name:           "problematic case with trailing >",
			input:          "user@wetalkie.tech>",
			expectedDomain: "wetalkie.tech",
			expectError:    false,
		},
		{
			name:           "RFC style",
			input:          "User Name <user@domain.com>",
			expectedDomain: "domain.com",
			expectError:    false,
		},
		{
			name:        "invalid email",
			input:       "not-an-email",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain, err := ExtractDomain(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for input '%s', but got none", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for input '%s': %v", tt.input, err)
				return
			}

			if domain != tt.expectedDomain {
				t.Errorf("Domain mismatch for input '%s': expected '%s', got '%s'",
					tt.input, tt.expectedDomain, domain)
			}
		})
	}
}

func TestValidateEmailFormat(t *testing.T) {
	validEmails := []string{
		"user@domain.com",
		"user.name@domain.co.uk",
		"<user@domain.com>",
		"Display Name <user@domain.com>",
		"user@wetalkie.tech>", // This should be valid after cleaning
		" user@domain.com ",
	}

	invalidEmails := []string{
		"",
		"not-an-email",
		"user@",
		"@domain.com",
		"user@domain",
		"user@@domain.com",
		"user@domain!.com",
	}

	for _, email := range validEmails {
		t.Run("valid_"+email, func(t *testing.T) {
			err := ValidateEmailFormat(email)
			if err != nil {
				t.Errorf("Expected email '%s' to be valid, but got error: %v", email, err)
			}
		})
	}

	for _, email := range invalidEmails {
		t.Run("invalid_"+email, func(t *testing.T) {
			err := ValidateEmailFormat(email)
			if err == nil {
				t.Errorf("Expected email '%s' to be invalid, but validation passed", email)
			}
		})
	}
}

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple domain",
			input:    "Domain.Com",
			expected: "domain.com",
		},
		{
			name:     "domain with trailing >",
			input:    "wetalkie.tech>",
			expected: "wetalkie.tech",
		},
		{
			name:     "domain with whitespace",
			input:    " Domain.Com ",
			expected: "domain.com",
		},
		{
			name:     "domain with quotes",
			input:    "\"domain.com\"",
			expected: "domain.com",
		},
		{
			name:     "mixed problematic characters",
			input:    " \"Domain.Com\"> ",
			expected: "domain.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeDomain(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeDomain('%s'): expected '%s', got '%s'",
					tt.input, tt.expected, result)
			}
		})
	}
}

func TestSplitEmailAddressSafe(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedLocal  string
		expectedDomain string
	}{
		{
			name:           "simple email",
			input:          "user@domain.com",
			expectedLocal:  "user",
			expectedDomain: "domain.com",
		},
		{
			name:           "problematic email with >",
			input:          "user@wetalkie.tech>",
			expectedLocal:  "user",
			expectedDomain: "wetalkie.tech",
		},
		{
			name:           "RFC style email",
			input:          "User <user@domain.com>",
			expectedLocal:  "user",
			expectedDomain: "domain.com",
		},
		{
			name:           "invalid email - fallback behavior",
			input:          "not-an-email",
			expectedLocal:  "not-an-email",
			expectedDomain: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			local, domain := SplitEmailAddressSafe(tt.input)

			if local != tt.expectedLocal {
				t.Errorf("Local part mismatch for input '%s': expected '%s', got '%s'",
					tt.input, tt.expectedLocal, local)
			}

			if domain != tt.expectedDomain {
				t.Errorf("Domain mismatch for input '%s': expected '%s', got '%s'",
					tt.input, tt.expectedDomain, domain)
			}
		})
	}
}

// Benchmark the new parsing vs simple split
func BenchmarkParseEmailAddress(b *testing.B) {
	email := "user@domain.com"
	for i := 0; i < b.N; i++ {
		_, _, _ = ParseEmailAddress(email)
	}
}

func BenchmarkSimpleSplit(b *testing.B) {
	email := "user@domain.com"
	for i := 0; i < b.N; i++ {
		parts := strings.Split(email, "@")
		_ = parts[0]
		_ = parts[1]
	}
}

func BenchmarkSplitEmailAddressSafe(b *testing.B) {
	email := "user@domain.com"
	for i := 0; i < b.N; i++ {
		_, _ = SplitEmailAddressSafe(email)
	}
}

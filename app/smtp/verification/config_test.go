package verification

import (
	"encoding/json"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	assert.True(t, config.EnableSPF, "SPF should be enabled by default")
	assert.True(t, config.EnableDKIM, "DKIM should be enabled by default")
	assert.True(t, config.EnableDMARC, "DMARC should be enabled by default")
	assert.True(t, config.EnableReputation, "Reputation should be enabled by default")
	assert.True(t, config.EnableContent, "Content analysis should be enabled by default")
	assert.Equal(t, 0.7, config.SpamThreshold, "Default spam threshold should be 0.7")
	assert.False(t, config.RejectOnFail, "RejectOnFail should be false by default")
	assert.True(t, config.QuarantineMode, "QuarantineMode should be true by default")
}

func TestDefaultVerificationConfig(t *testing.T) {
	config := DefaultVerificationConfig()
	
	// Check verification settings
	assert.True(t, config.Verification.EnableSPF)
	assert.True(t, config.Verification.EnableDKIM)
	assert.True(t, config.Verification.EnableDMARC)
	assert.True(t, config.Verification.EnableReputation)
	assert.True(t, config.Verification.EnableContent)
	
	// Check DNS settings
	assert.Equal(t, "8.8.8.8:53", config.DNS.Resolver)
	assert.Equal(t, 5, config.DNS.Timeout)
	
	// Check blacklist settings
	assert.True(t, config.Blacklists.Enabled)
	assert.NotEmpty(t, config.Blacklists.IPLists)
	assert.NotEmpty(t, config.Blacklists.DomainLists)
	assert.Equal(t, 60, config.Blacklists.CacheTime)
	
	// Check specific blacklists
	assert.Contains(t, config.Blacklists.IPLists, "zen.spamhaus.org")
	assert.Contains(t, config.Blacklists.IPLists, "bl.spamcop.net")
	assert.Contains(t, config.Blacklists.DomainLists, "dbl.spamhaus.org")
	assert.Contains(t, config.Blacklists.DomainLists, "surbl.org")
}

func TestLoadConfigFromFile_NonExistentFile(t *testing.T) {
	config, err := LoadConfigFromFile("nonexistent.json")
	
	assert.NoError(t, err, "Should not error for non-existent file")
	assert.Equal(t, DefaultVerificationConfig(), config, "Should return default config")
}

func TestLoadConfigFromFile_EmptyFilename(t *testing.T) {
	config, err := LoadConfigFromFile("")
	
	assert.NoError(t, err, "Should not error for empty filename")
	assert.Equal(t, DefaultVerificationConfig(), config, "Should return default config")
}

func TestLoadConfigFromFile_ValidFile(t *testing.T) {
	// Create a temporary config file
	tempFile, err := os.CreateTemp("", "test_config_*.json")
	assert.NoError(t, err)
	defer os.Remove(tempFile.Name())
	
	testConfig := Config{
		Verification: VerificationConfig{
			EnableSPF:        false,
			EnableDKIM:       true,
			EnableDMARC:      true,
			EnableReputation: false,
			EnableContent:    true,
			SpamThreshold:    0.8,
			RejectOnFail:     true,
			QuarantineMode:   false,
		},
		DNS: DNSConfig{
			Resolver: "1.1.1.1:53",
			Timeout:  10,
		},
		Blacklists: BlacklistConfig{
			Enabled:   false,
			IPLists:   []string{"custom.blacklist.com"},
			CacheTime: 30,
		},
	}
	
	configJSON, err := json.MarshalIndent(testConfig, "", "  ")
	assert.NoError(t, err)
	
	err = os.WriteFile(tempFile.Name(), configJSON, 0644)
	assert.NoError(t, err)
	
	// Load the config
	loadedConfig, err := LoadConfigFromFile(tempFile.Name())
	assert.NoError(t, err)
	
	// Verify loaded config
	assert.False(t, loadedConfig.Verification.EnableSPF)
	assert.True(t, loadedConfig.Verification.EnableDKIM)
	assert.False(t, loadedConfig.Verification.EnableReputation)
	assert.Equal(t, 0.8, loadedConfig.Verification.SpamThreshold)
	assert.True(t, loadedConfig.Verification.RejectOnFail)
	assert.False(t, loadedConfig.Verification.QuarantineMode)
	assert.Equal(t, "1.1.1.1:53", loadedConfig.DNS.Resolver)
	assert.Equal(t, 10, loadedConfig.DNS.Timeout)
	assert.False(t, loadedConfig.Blacklists.Enabled)
	assert.Equal(t, 30, loadedConfig.Blacklists.CacheTime)
}

func TestLoadConfigFromFile_InvalidJSON(t *testing.T) {
	// Create a temporary invalid JSON file
	tempFile, err := os.CreateTemp("", "test_invalid_*.json")
	assert.NoError(t, err)
	defer os.Remove(tempFile.Name())
	
	err = os.WriteFile(tempFile.Name(), []byte("invalid json content {"), 0644)
	assert.NoError(t, err)
	
	// Try to load the invalid config
	_, err = LoadConfigFromFile(tempFile.Name())
	assert.Error(t, err, "Should error for invalid JSON")
}

func TestSaveConfigToFile(t *testing.T) {
	testConfig := Config{
		Verification: VerificationConfig{
			EnableSPF:     true,
			EnableDKIM:    false,
			SpamThreshold: 0.9,
		},
		DNS: DNSConfig{
			Resolver: "8.8.4.4:53",
			Timeout:  7,
		},
	}
	
	// Create temporary file
	tempFile, err := os.CreateTemp("", "test_save_*.json")
	assert.NoError(t, err)
	defer os.Remove(tempFile.Name())
	tempFile.Close() // Close so we can write to it
	
	// Save config
	err = SaveConfigToFile(testConfig, tempFile.Name())
	assert.NoError(t, err)
	
	// Load and verify
	loadedConfig, err := LoadConfigFromFile(tempFile.Name())
	assert.NoError(t, err)
	
	assert.True(t, loadedConfig.Verification.EnableSPF)
	assert.False(t, loadedConfig.Verification.EnableDKIM)
	assert.Equal(t, 0.9, loadedConfig.Verification.SpamThreshold)
	assert.Equal(t, "8.8.4.4:53", loadedConfig.DNS.Resolver)
	assert.Equal(t, 7, loadedConfig.DNS.Timeout)
}

func TestConfigFromEnvironment_Defaults(t *testing.T) {
	// Clear any existing environment variables
	envVars := []string{
		"MAILVAULT_DISABLE_SPF",
		"MAILVAULT_DISABLE_DKIM",
		"MAILVAULT_DISABLE_DMARC",
		"MAILVAULT_DISABLE_REPUTATION",
		"MAILVAULT_DISABLE_CONTENT",
		"MAILVAULT_REJECT_ON_FAIL",
		"MAILVAULT_QUARANTINE_MODE",
	}
	
	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
	
	config := ConfigFromEnvironment()
	
	// Should match defaults
	assert.True(t, config.EnableSPF)
	assert.True(t, config.EnableDKIM)
	assert.True(t, config.EnableDMARC)
	assert.True(t, config.EnableReputation)
	assert.True(t, config.EnableContent)
	assert.False(t, config.RejectOnFail)
	assert.True(t, config.QuarantineMode)
}

func TestConfigFromEnvironment_DisableFeatures(t *testing.T) {
	// Set environment variables to disable features
	os.Setenv("MAILVAULT_DISABLE_SPF", "true")
	os.Setenv("MAILVAULT_DISABLE_DKIM", "true")
	os.Setenv("MAILVAULT_DISABLE_DMARC", "true")
	os.Setenv("MAILVAULT_DISABLE_REPUTATION", "true")
	os.Setenv("MAILVAULT_DISABLE_CONTENT", "true")
	defer func() {
		os.Unsetenv("MAILVAULT_DISABLE_SPF")
		os.Unsetenv("MAILVAULT_DISABLE_DKIM")
		os.Unsetenv("MAILVAULT_DISABLE_DMARC")
		os.Unsetenv("MAILVAULT_DISABLE_REPUTATION")
		os.Unsetenv("MAILVAULT_DISABLE_CONTENT")
	}()
	
	config := ConfigFromEnvironment()
	
	assert.False(t, config.EnableSPF)
	assert.False(t, config.EnableDKIM)
	assert.False(t, config.EnableDMARC)
	assert.False(t, config.EnableReputation)
	assert.False(t, config.EnableContent)
}

func TestConfigFromEnvironment_EnableStrictMode(t *testing.T) {
	// Set environment variables for strict mode
	os.Setenv("MAILVAULT_REJECT_ON_FAIL", "true")
	os.Setenv("MAILVAULT_QUARANTINE_MODE", "false")
	defer func() {
		os.Unsetenv("MAILVAULT_REJECT_ON_FAIL")
		os.Unsetenv("MAILVAULT_QUARANTINE_MODE")
	}()
	
	config := ConfigFromEnvironment()
	
	assert.True(t, config.RejectOnFail)
	assert.False(t, config.QuarantineMode)
}

func TestNewPolicyManager(t *testing.T) {
	manager := NewPolicyManager()
	
	assert.NotNil(t, manager)
	assert.Empty(t, manager.policies)
	assert.Equal(t, "default", manager.defaultPolicy.Name)
	assert.Equal(t, DefaultConfig(), manager.defaultPolicy.Config)
}

func TestPolicyManager_AddPolicy(t *testing.T) {
	manager := NewPolicyManager()
	
	policy := Policy{
		Name:        "strict_policy",
		Description: "Strict verification for external emails",
		Config: VerificationConfig{
			EnableSPF:     true,
			EnableDKIM:    true,
			EnableDMARC:   true,
			RejectOnFail:  true,
			SpamThreshold: 0.5,
		},
		Priority: 1,
	}
	
	manager.AddPolicy(policy)
	
	assert.Len(t, manager.policies, 1)
	assert.Equal(t, "strict_policy", manager.policies[0].Name)
}

func TestPolicyManager_GetPolicyForEmail_Default(t *testing.T) {
	manager := NewPolicyManager()
	
	emailCtx := EmailContext{
		From: "test@example.com",
		To:   []string{"recipient@company.com"},
	}
	
	policy := manager.GetPolicyForEmail(emailCtx)
	
	assert.Equal(t, "default", policy.Name)
}

func TestPolicyManager_GetPolicyForEmail_MatchingPolicy(t *testing.T) {
	manager := NewPolicyManager()
	
	// Add a policy for external domains
	externalPolicy := Policy{
		Name:        "external_policy",
		Description: "Policy for external domains",
		Config: VerificationConfig{
			EnableSPF:     true,
			RejectOnFail:  true,
			SpamThreshold: 0.5,
		},
		Conditions: []PolicyCondition{
			{
				Field:    "from_domain",
				Operator: "equals",
				Value:    "external.com",
			},
		},
		Priority: 1,
	}
	
	manager.AddPolicy(externalPolicy)
	
	emailCtx := EmailContext{
		From: "sender@external.com",
		To:   []string{"recipient@company.com"},
	}
	
	policy := manager.GetPolicyForEmail(emailCtx)
	
	assert.Equal(t, "external_policy", policy.Name)
	assert.True(t, policy.Config.RejectOnFail)
}

func TestPolicyManager_EvaluateCondition_FromDomain(t *testing.T) {
	manager := NewPolicyManager()
	
	tests := []struct {
		name      string
		condition PolicyCondition
		emailCtx  EmailContext
		expected  bool
	}{
		{
			name: "From domain equals",
			condition: PolicyCondition{
				Field:    "from_domain",
				Operator: "equals",
				Value:    "example.com",
			},
			emailCtx: EmailContext{
				From: "user@example.com",
			},
			expected: true,
		},
		{
			name: "From domain not equals",
			condition: PolicyCondition{
				Field:    "from_domain",
				Operator: "equals",
				Value:    "example.com",
			},
			emailCtx: EmailContext{
				From: "user@different.com",
			},
			expected: false,
		},
		{
			name: "From domain contains",
			condition: PolicyCondition{
				Field:    "from_domain",
				Operator: "contains",
				Value:    "example",
			},
			emailCtx: EmailContext{
				From: "user@test.example.com",
			},
			expected: true,
		},
		{
			name: "From domain matches regex",
			condition: PolicyCondition{
				Field:    "from_domain",
				Operator: "matches",
				Value:    ".*\\.com$",
			},
			emailCtx: EmailContext{
				From: "user@example.com",
			},
			expected: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.evaluateCondition(tt.condition, tt.emailCtx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPolicyManager_EvaluateCondition_ToDomain(t *testing.T) {
	manager := NewPolicyManager()
	
	condition := PolicyCondition{
		Field:    "to_domain",
		Operator: "equals",
		Value:    "company.com",
	}
	
	emailCtx := EmailContext{
		From: "sender@external.com",
		To:   []string{"recipient@company.com"},
	}
	
	result := manager.evaluateCondition(condition, emailCtx)
	assert.True(t, result)
}

func TestPolicyManager_EvaluateCondition_SenderIP(t *testing.T) {
	manager := NewPolicyManager()
	
	condition := PolicyCondition{
		Field:    "sender_ip",
		Operator: "equals",
		Value:    "192.168.1.100",
	}
	
	emailCtx := EmailContext{
		From:     "sender@example.com",
		SenderIP: net.ParseIP("192.168.1.100"),
	}
	
	result := manager.evaluateCondition(condition, emailCtx)
	assert.True(t, result)
}

func TestPolicyManager_EvaluateCondition_Subject(t *testing.T) {
	manager := NewPolicyManager()
	
	condition := PolicyCondition{
		Field:    "subject",
		Operator: "contains",
		Value:    "urgent",
	}
	
	emailCtx := EmailContext{
		Subject: "Urgent: Please review this document",
	}
	
	result := manager.evaluateCondition(condition, emailCtx)
	assert.True(t, result)
}

func TestPolicyManager_EvaluateCondition_InvalidField(t *testing.T) {
	manager := NewPolicyManager()
	
	condition := PolicyCondition{
		Field:    "invalid_field",
		Operator: "equals",
		Value:    "test",
	}
	
	emailCtx := EmailContext{}
	
	result := manager.evaluateCondition(condition, emailCtx)
	assert.False(t, result)
}

func TestPolicyManager_EvaluateCondition_InvalidOperator(t *testing.T) {
	manager := NewPolicyManager()
	
	condition := PolicyCondition{
		Field:    "from_domain",
		Operator: "invalid_operator",
		Value:    "example.com",
	}
	
	emailCtx := EmailContext{
		From: "user@example.com",
	}
	
	result := manager.evaluateCondition(condition, emailCtx)
	assert.False(t, result)
}

func TestPolicyManager_MultipleConditions(t *testing.T) {
	manager := NewPolicyManager()
	
	// Policy with multiple conditions (both must match)
	policy := Policy{
		Name: "multi_condition_policy",
		Conditions: []PolicyCondition{
			{
				Field:    "from_domain",
				Operator: "equals",
				Value:    "external.com",
			},
			{
				Field:    "subject",
				Operator: "contains",
				Value:    "urgent",
			},
		},
		Priority: 1,
	}
	
	manager.AddPolicy(policy)
	
	// Email that matches both conditions
	emailCtx1 := EmailContext{
		From:    "user@external.com",
		Subject: "Urgent request",
	}
	
	result1 := manager.GetPolicyForEmail(emailCtx1)
	assert.Equal(t, "multi_condition_policy", result1.Name)
	
	// Email that matches only one condition
	emailCtx2 := EmailContext{
		From:    "user@external.com",
		Subject: "Regular request",
	}
	
	result2 := manager.GetPolicyForEmail(emailCtx2)
	assert.Equal(t, "default", result2.Name) // Should fall back to default
}

func TestPolicy_JSON_Marshaling(t *testing.T) {
	policy := Policy{
		Name:        "test_policy",
		Description: "Test policy for JSON marshaling",
		Config: VerificationConfig{
			EnableSPF:     true,
			EnableDKIM:    false,
			SpamThreshold: 0.8,
		},
		Conditions: []PolicyCondition{
			{
				Field:    "from_domain",
				Operator: "equals",
				Value:    "test.com",
			},
		},
		Priority: 5,
	}
	
	// Marshal to JSON
	jsonData, err := json.Marshal(policy)
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonData)
	
	// Unmarshal back
	var unmarshaledPolicy Policy
	err = json.Unmarshal(jsonData, &unmarshaledPolicy)
	assert.NoError(t, err)
	
	// Verify data integrity
	assert.Equal(t, policy.Name, unmarshaledPolicy.Name)
	assert.Equal(t, policy.Description, unmarshaledPolicy.Description)
	assert.Equal(t, policy.Config.EnableSPF, unmarshaledPolicy.Config.EnableSPF)
	assert.Equal(t, policy.Config.EnableDKIM, unmarshaledPolicy.Config.EnableDKIM)
	assert.Equal(t, policy.Config.SpamThreshold, unmarshaledPolicy.Config.SpamThreshold)
	assert.Len(t, unmarshaledPolicy.Conditions, 1)
	assert.Equal(t, policy.Conditions[0].Field, unmarshaledPolicy.Conditions[0].Field)
	assert.Equal(t, policy.Priority, unmarshaledPolicy.Priority)
}
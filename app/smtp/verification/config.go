package verification

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
)

// Config holds all verification configuration
type Config struct {
	Verification VerificationConfig `json:"verification"`
	DNS          DNSConfig          `json:"dns"`
	Blacklists   BlacklistConfig    `json:"blacklists"`
}

// DNSConfig holds DNS resolver configuration
type DNSConfig struct {
	Resolver string `json:"resolver"`
	Timeout  int    `json:"timeout_seconds"`
}

// BlacklistConfig holds blacklist configuration
type BlacklistConfig struct {
	Enabled    bool     `json:"enabled"`
	IPLists    []string `json:"ip_lists"`
	DomainLists []string `json:"domain_lists"`
	CacheTime  int      `json:"cache_time_minutes"`
}

// DefaultVerificationConfig returns default verification configuration
func DefaultVerificationConfig() Config {
	return Config{
		Verification: DefaultConfig(),
		DNS: DNSConfig{
			Resolver: "8.8.8.8:53",
			Timeout:  5,
		},
		Blacklists: BlacklistConfig{
			Enabled: true,
			IPLists: []string{
				"zen.spamhaus.org",
				"bl.spamcop.net",
				"dnsbl.sorbs.net",
				"b.barracudacentral.org",
				"cbl.abuseat.org",
				"psbl.surriel.com",
				"ubl.unsubscore.com",
			},
			DomainLists: []string{
				"dbl.spamhaus.org",
				"surbl.org",
				"uribl.com",
				"multi.surbl.org",
			},
			CacheTime: 60,
		},
	}
}

// LoadConfigFromFile loads verification configuration from JSON file
func LoadConfigFromFile(filename string) (Config, error) {
	config := DefaultVerificationConfig()
	
	if filename == "" {
		return config, nil
	}
	
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil // Use defaults if file doesn't exist
		}
		return config, err
	}
	
	err = json.Unmarshal(data, &config)
	if err != nil {
		return config, err
	}
	
	return config, nil
}

// SaveConfigToFile saves verification configuration to JSON file
func SaveConfigToFile(config Config, filename string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(filename, data, 0644)
}

// Policy represents a verification policy for different email types
type Policy struct {
	Name         string             `json:"name"`
	Description  string             `json:"description"`
	Config       VerificationConfig `json:"config"`
	Conditions   []PolicyCondition  `json:"conditions"`
	Priority     int                `json:"priority"`
}

// PolicyCondition defines when a policy should be applied
type PolicyCondition struct {
	Field    string `json:"field"`    // "from_domain", "to_domain", "sender_ip", "content_type"
	Operator string `json:"operator"` // "equals", "contains", "matches", "in_range"
	Value    string `json:"value"`
}

// PolicyManager manages verification policies
type PolicyManager struct {
	policies     []Policy
	defaultPolicy Policy
}

// NewPolicyManager creates a new policy manager
func NewPolicyManager() *PolicyManager {
	return &PolicyManager{
		policies: []Policy{},
		defaultPolicy: Policy{
			Name:        "default",
			Description: "Default verification policy",
			Config:      DefaultConfig(),
			Priority:    0,
		},
	}
}

// AddPolicy adds a new verification policy
func (pm *PolicyManager) AddPolicy(policy Policy) {
	pm.policies = append(pm.policies, policy)
}

// GetPolicyForEmail returns the appropriate policy for an email
func (pm *PolicyManager) GetPolicyForEmail(emailCtx EmailContext) Policy {
	// Sort policies by priority (higher priority first)
	for _, policy := range pm.policies {
		if pm.matchesPolicy(policy, emailCtx) {
			return policy
		}
	}
	
	return pm.defaultPolicy
}

// matchesPolicy checks if an email matches a policy's conditions
func (pm *PolicyManager) matchesPolicy(policy Policy, emailCtx EmailContext) bool {
	for _, condition := range policy.Conditions {
		if !pm.evaluateCondition(condition, emailCtx) {
			return false
		}
	}
	return true
}

// evaluateCondition evaluates a single policy condition
func (pm *PolicyManager) evaluateCondition(condition PolicyCondition, emailCtx EmailContext) bool {
	var fieldValue string
	
	switch condition.Field {
	case "from_domain":
		parts := strings.Split(emailCtx.From, "@")
		if len(parts) == 2 {
			fieldValue = parts[1]
		}
	case "to_domain":
		if len(emailCtx.To) > 0 {
			parts := strings.Split(emailCtx.To[0], "@")
			if len(parts) == 2 {
				fieldValue = parts[1]
			}
		}
	case "sender_ip":
		if emailCtx.SenderIP != nil {
			fieldValue = emailCtx.SenderIP.String()
		}
	case "subject":
		fieldValue = emailCtx.Subject
	default:
		return false
	}
	
	switch condition.Operator {
	case "equals":
		return fieldValue == condition.Value
	case "contains":
		return strings.Contains(strings.ToLower(fieldValue), strings.ToLower(condition.Value))
	case "matches":
		// Simple regex matching
		matched, _ := regexp.MatchString(condition.Value, fieldValue)
		return matched
	default:
		return false
	}
}

// Environment-based configuration
func ConfigFromEnvironment() VerificationConfig {
	config := DefaultConfig()
	
	// Allow environment variables to override defaults
	if os.Getenv("MAILVAULT_DISABLE_SPF") == "true" {
		config.EnableSPF = false
	}
	if os.Getenv("MAILVAULT_DISABLE_DKIM") == "true" {
		config.EnableDKIM = false
	}
	if os.Getenv("MAILVAULT_DISABLE_DMARC") == "true" {
		config.EnableDMARC = false
	}
	if os.Getenv("MAILVAULT_DISABLE_REPUTATION") == "true" {
		config.EnableReputation = false
	}
	if os.Getenv("MAILVAULT_DISABLE_CONTENT") == "true" {
		config.EnableContent = false
	}
	
	if os.Getenv("MAILVAULT_REJECT_ON_FAIL") == "true" {
		config.RejectOnFail = true
	}
	if os.Getenv("MAILVAULT_QUARANTINE_MODE") == "false" {
		config.QuarantineMode = false
	}
	
	return config
}
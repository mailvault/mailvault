package validation

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"strings"
	"time"
)

// DNSService provides DNS validation functionality
type DNSService interface {
	ValidateMXRecords(ctx context.Context, domain string, expectedServers []string) (*MXValidationResult, error)
	ValidateTXTRecord(ctx context.Context, domain string, expectedRecord string) (*TXTValidationResult, error)
	ValidateFullDomain(ctx context.Context, domain string, verificationToken string, config *ValidationConfig) (*FullValidationResult, error)
}

// DNSValidator implements DNSService
type DNSValidator struct {
	resolver *net.Resolver
	config   ValidationConfig
	logger   *slog.Logger
}

// NewDNSValidator creates a new DNS validator
func NewDNSValidator(config ValidationConfig, logger *slog.Logger) DNSService {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: config.ValidationTimeout,
			}
			return d.DialContext(ctx, network, config.DNSServer)
		},
	}

	return &DNSValidator{
		resolver: resolver,
		config:   config,
		logger:   logger,
	}
}

// MXValidationResult contains the result of MX record validation
type MXValidationResult struct {
	Domain          string        `json:"domain,omitempty"`
	Valid           bool          `json:"valid"`
	FoundRecords    []MXRecord    `json:"found_records"`
	ExpectedServers []string      `json:"expected_servers"`
	MissingServers  []string      `json:"missing_servers,omitempty"`
	ExtraServers    []string      `json:"extra_servers,omitempty"`
	QueryTime       time.Duration `json:"query_time"`
	Error           string        `json:"error,omitempty"`
}

// TXTValidationResult contains the result of TXT record validation
type TXTValidationResult struct {
	Domain         string        `json:"domain,omitempty"`
	Valid          bool          `json:"valid"`
	FoundRecords   []string      `json:"found_records"`
	ExpectedRecord string        `json:"expected_record"`
	MatchingRecord string        `json:"matching_record,omitempty"`
	QueryTime      time.Duration `json:"query_time"`
	RetryCount     int           `json:"retry_count,omitempty"`
	Error          string        `json:"error,omitempty"`
}

// FullValidationResult contains the complete validation result
type FullValidationResult struct {
	Domain           string               `json:"domain"`
	OverallValid     bool                 `json:"overall_valid"`
	MXValidation     *MXValidationResult  `json:"mx_validation"`
	TXTValidation    *TXTValidationResult `json:"txt_validation"`
	TotalTime        time.Duration        `json:"total_time"`
	ValidationErrors []string             `json:"validation_errors,omitempty"`
}

// ValidateMXRecords validates that the domain's MX records point to expected servers
func (v *DNSValidator) ValidateMXRecords(ctx context.Context, domain string, expectedServers []string) (*MXValidationResult, error) {
	startTime := time.Now()

	v.logger.Info("Starting MX record validation",
		"domain", domain,
		"expected_servers", expectedServers,
	)

	result := &MXValidationResult{
		ExpectedServers: expectedServers,
		QueryTime:       0,
	}

	// Add timeout to context
	timeoutCtx, cancel := context.WithTimeout(ctx, v.config.MXCheckTimeout)
	defer cancel()

	// Look up MX records
	mxRecords, err := v.resolver.LookupMX(timeoutCtx, domain)
	if err != nil {
		result.Error = fmt.Sprintf("failed to lookup MX records: %v", err)
		result.QueryTime = time.Since(startTime)
		v.logger.Error("MX record lookup failed",
			"domain", domain,
			"error", err,
			"query_time", result.QueryTime,
		)
		return result, nil
	}

	result.QueryTime = time.Since(startTime)

	// Convert to our MXRecord format
	for _, mx := range mxRecords {
		result.FoundRecords = append(result.FoundRecords, MXRecord{
			Host:     strings.TrimSuffix(mx.Host, "."),
			Priority: int(mx.Pref),
		})
	}

	// Check if expected servers are present
	foundServers := make(map[string]bool)
	for _, mx := range result.FoundRecords {
		foundServers[strings.ToLower(mx.Host)] = true
	}

	var missingServers []string
	allExpectedFound := true

	for _, expected := range expectedServers {
		normalizedExpected := strings.ToLower(expected)
		if !foundServers[normalizedExpected] {
			missingServers = append(missingServers, expected)
			allExpectedFound = false
		}
	}

	// Find extra servers (not in expected list)
	expectedMap := make(map[string]bool)
	for _, expected := range expectedServers {
		expectedMap[strings.ToLower(expected)] = true
	}

	var extraServers []string
	for _, mx := range result.FoundRecords {
		if !expectedMap[strings.ToLower(mx.Host)] {
			extraServers = append(extraServers, mx.Host)
		}
	}

	result.Valid = allExpectedFound && len(result.FoundRecords) > 0
	result.MissingServers = missingServers
	result.ExtraServers = extraServers

	v.logger.Info("MX record validation completed",
		"domain", domain,
		"valid", result.Valid,
		"found_count", len(result.FoundRecords),
		"missing_count", len(missingServers),
		"extra_count", len(extraServers),
		"query_time", result.QueryTime,
	)

	return result, nil
}

// ValidateTXTRecord validates that the domain has the expected TXT record for verification
func (v *DNSValidator) ValidateTXTRecord(ctx context.Context, domain string, expectedRecord string) (*TXTValidationResult, error) {
	startTime := time.Now()

	v.logger.Info("Starting TXT record validation",
		"domain", domain,
		"expected_record", expectedRecord,
	)

	result := &TXTValidationResult{
		ExpectedRecord: expectedRecord,
		QueryTime:      0,
	}

	// Add timeout to context
	timeoutCtx, cancel := context.WithTimeout(ctx, v.config.TXTCheckTimeout)
	defer cancel()

	// Look up TXT records
	txtRecords, err := v.resolver.LookupTXT(timeoutCtx, domain)
	if err != nil {
		result.Error = fmt.Sprintf("failed to lookup TXT records: %v", err)
		result.QueryTime = time.Since(startTime)
		v.logger.Error("TXT record lookup failed",
			"domain", domain,
			"error", err,
			"query_time", result.QueryTime,
		)
		return result, nil
	}

	result.QueryTime = time.Since(startTime)
	result.FoundRecords = txtRecords

	// Check if the expected record is present
	expectedLower := strings.ToLower(expectedRecord)
	for _, record := range txtRecords {
		if strings.ToLower(record) == expectedLower {
			result.Valid = true
			result.MatchingRecord = record
			break
		}
	}

	v.logger.Info("TXT record validation completed",
		"domain", domain,
		"valid", result.Valid,
		"found_count", len(txtRecords),
		"matching_record", result.MatchingRecord,
		"query_time", result.QueryTime,
	)

	return result, nil
}

// ValidateFullDomain performs complete domain validation including MX and TXT records
func (v *DNSValidator) ValidateFullDomain(ctx context.Context, domain string, verificationToken string, config *ValidationConfig) (*FullValidationResult, error) {
	startTime := time.Now()

	v.logger.Info("Starting full domain validation",
		"domain", domain,
		"verification_token", verificationToken,
	)

	result := &FullValidationResult{
		Domain:           domain,
		OverallValid:     false,
		ValidationErrors: []string{},
	}

	// Validate MX records
	mxResult, err := v.ValidateMXRecords(ctx, domain, config.ExpectedMXServers)
	if err != nil {
		result.ValidationErrors = append(result.ValidationErrors, fmt.Sprintf("MX validation error: %v", err))
	} else {
		result.MXValidation = mxResult
		if mxResult.Error != "" {
			result.ValidationErrors = append(result.ValidationErrors, fmt.Sprintf("MX validation: %s", mxResult.Error))
		}
	}

	// Create expected TXT record
	expectedTXTRecord := fmt.Sprintf("%s=%s", config.TXTRecordPrefix, verificationToken)

	// Validate TXT record
	txtResult, err := v.ValidateTXTRecord(ctx, domain, expectedTXTRecord)
	if err != nil {
		result.ValidationErrors = append(result.ValidationErrors, fmt.Sprintf("TXT validation error: %v", err))
	} else {
		result.TXTValidation = txtResult
		if txtResult.Error != "" {
			result.ValidationErrors = append(result.ValidationErrors, fmt.Sprintf("TXT validation: %s", txtResult.Error))
		}
	}

	// Determine overall validity
	result.OverallValid = (result.MXValidation != nil && result.MXValidation.Valid) &&
		(result.TXTValidation != nil && result.TXTValidation.Valid)

	result.TotalTime = time.Since(startTime)

	v.logger.Info("Full domain validation completed",
		"domain", domain,
		"overall_valid", result.OverallValid,
		"mx_valid", result.MXValidation != nil && result.MXValidation.Valid,
		"txt_valid", result.TXTValidation != nil && result.TXTValidation.Valid,
		"total_time", result.TotalTime,
		"error_count", len(result.ValidationErrors),
	)

	return result, nil
}

// ValidateDomainBasic performs basic domain validation (format and DNS resolution)
func ValidateDomainBasic(ctx context.Context, domain string) error {
	// Basic format validation
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}

	if len(domain) > 253 {
		return fmt.Errorf("domain too long (max 253 characters)")
	}

	if !strings.Contains(domain, ".") {
		return fmt.Errorf("domain must contain at least one dot")
	}

	// Check if domain resolves
	resolver := &net.Resolver{}
	_, err := resolver.LookupHost(ctx, domain)
	if err != nil {
		return fmt.Errorf("domain does not resolve: %v", err)
	}

	return nil
}

// GetDomainMXRecords returns the current MX records for a domain
func GetDomainMXRecords(ctx context.Context, domain string) ([]MXRecord, error) {
	resolver := &net.Resolver{}

	mxRecords, err := resolver.LookupMX(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup MX records: %v", err)
	}

	var results []MXRecord
	for _, mx := range mxRecords {
		results = append(results, MXRecord{
			Host:     strings.TrimSuffix(mx.Host, "."),
			Priority: int(mx.Pref),
		})
	}

	// Sort by priority (lower priority number = higher priority)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Priority < results[j].Priority
	})

	return results, nil
}

// GetDomainTXTRecords returns the current TXT records for a domain
func GetDomainTXTRecords(ctx context.Context, domain string) ([]string, error) {
	resolver := &net.Resolver{}

	txtRecords, err := resolver.LookupTXT(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup TXT records: %v", err)
	}

	return txtRecords, nil
}

// FilterMailVaultTXTRecords filters TXT records to only include MailVault verification records
func FilterMailVaultTXTRecords(txtRecords []string, prefix string) []string {
	var filtered []string
	for _, record := range txtRecords {
		if strings.HasPrefix(strings.ToLower(record), strings.ToLower(prefix+"=")) {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

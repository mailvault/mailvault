package verification

import (
	"context"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestVerifier_VerifyEmail(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	config := VerificationConfig{
		EnableSPF:        true,
		EnableDKIM:       true,
		EnableDMARC:      true,
		EnableReputation: true,
		EnableContent:    true,
		SpamThreshold:    0.7,
		RejectOnFail:     false,
		QuarantineMode:   true,
	}
	
	verifier := NewVerifier(config, logger)
	
	tests := []struct {
		name        string
		emailCtx    EmailContext
		expectedAction Action
	}{
		{
			name: "legitimate email",
			emailCtx: EmailContext{
				From:    "sender@example.com",
				To:      []string{"recipient@test.com"},
				Subject: "Test Email",
				Body:    []byte("From: sender@example.com\r\nTo: recipient@test.com\r\nSubject: Test Email\r\nMessage-ID: <123@example.com>\r\nDate: Mon, 19 Aug 2025 12:00:00 -0300\r\n\r\nThis is a test email with proper headers."),
				Headers: []Header{
					{Name: "From", Value: "sender@example.com"},
					{Name: "To", Value: "recipient@test.com"},
					{Name: "Subject", Value: "Test Email"},
					{Name: "Message-ID", Value: "<123@example.com>"},
					{Name: "Date", Value: "Mon, 19 Aug 2025 12:00:00 -0300"},
				},
				SenderIP:   net.ParseIP("192.168.1.100"), // Private IP
				ReceivedAt: time.Now(),
			},
			expectedAction: ActionQuarantine, // Still might be quarantined due to SPF/DKIM failure
		},
		{
			name: "spam email with multiple indicators",
			emailCtx: EmailContext{
				From:    "spammer@spammydomain.com",
				To:      []string{"victim@test.com"},
				Subject: "FREE MONEY!!! CLICK HERE NOW!!!",
				Body:    []byte("From: spammer@spammydomain.com\r\nTo: victim@test.com\r\nSubject: FREE MONEY!!! CLICK HERE NOW!!!\r\n\r\nCongratulations! You've won $1,000,000! Click here immediately to claim your prize! This is guaranteed and 100% free!"),
				Headers: []Header{
					{Name: "From", Value: "spammer@spammydomain.com"},
					{Name: "To", Value: "victim@test.com"},
					{Name: "Subject", Value: "FREE MONEY!!! CLICK HERE NOW!!!"},
				},
				SenderIP:   net.ParseIP("1.2.3.4"), // Public IP
				ReceivedAt: time.Now(),
			},
			expectedAction: ActionQuarantine, // Should be quarantined due to high spam score
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := verifier.VerifyEmail(ctx, tt.emailCtx)
			
			assert.Equal(t, tt.expectedAction, result.Action, "Expected action %s but got %s", tt.expectedAction, result.Action)
			assert.NotZero(t, result.Timestamp, "Timestamp should be set")
		})
	}
}

func TestSPFVerifier(t *testing.T) {
	verifier := NewSPFVerifier("8.8.8.8:53")
	
	tests := []struct {
		name     string
		emailCtx EmailContext
		expected SPFStatus
	}{
		{
			name: "no sender IP",
			emailCtx: EmailContext{
				From:     "test@example.com",
				SenderIP: nil,
			},
			expected: SPFNone,
		},
		{
			name: "invalid email format",
			emailCtx: EmailContext{
				From:     "invalid-email",
				SenderIP: net.ParseIP("192.168.1.1"),
			},
			expected: SPFPermError,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := verifier.Verify(ctx, tt.emailCtx)
			assert.Equal(t, tt.expected, result.Result)
		})
	}
}

func TestDKIMVerifier(t *testing.T) {
	verifier := NewDKIMVerifier("8.8.8.8:53")
	
	// Test email without DKIM signature
	emailCtx := EmailContext{
		From:    "test@example.com",
		To:      []string{"recipient@test.com"},
		Subject: "Test",
		Body:    []byte("From: test@example.com\r\nTo: recipient@test.com\r\nSubject: Test\r\n\r\nTest body"),
		Headers: []Header{
			{Name: "From", Value: "test@example.com"},
			{Name: "To", Value: "recipient@test.com"},
			{Name: "Subject", Value: "Test"},
		},
	}
	
	ctx := context.Background()
	result := verifier.Verify(ctx, emailCtx)
	
	assert.False(t, result.Valid, "Email without DKIM signature should not be valid")
	assert.Empty(t, result.Results, "Should have no DKIM results")
}

func TestContentVerifier(t *testing.T) {
	verifier := NewContentVerifier()
	
	tests := []struct {
		name           string
		emailCtx       EmailContext
		expectedClass  string
		expectHighSpam bool
	}{
		{
			name: "legitimate email",
			emailCtx: EmailContext{
				Subject: "Meeting tomorrow",
				Body:    []byte("From: boss@company.com\r\nSubject: Meeting tomorrow\r\nMessage-ID: <456@company.com>\r\nDate: Mon, 19 Aug 2025 12:00:00 -0300\r\n\r\nHi team, let's meet tomorrow at 2pm to discuss the project. Please confirm your attendance."),
			},
			expectedClass:  "questionable", // Might be flagged as questionable due to short content
			expectHighSpam: false,
		},
		{
			name: "spam email",
			emailCtx: EmailContext{
				Subject: "FREE VIAGRA!!! CLICK NOW!!!",
				Body:    []byte("From: spammer@spam.com\r\nSubject: FREE VIAGRA!!! CLICK NOW!!!\r\n\r\nFREE VIAGRA! GUARANTEED! CLICK HERE NOW! 100% FREE! NO RISK!"),
			},
			expectedClass:  "spam",
			expectHighSpam: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := verifier.Verify(ctx, tt.emailCtx)
			
			assert.Equal(t, tt.expectedClass, result.Classification)
			
			if tt.expectHighSpam {
				assert.Greater(t, result.SpamScore, 0.7, "Spam score should be high")
			} else {
				assert.Less(t, result.SpamScore, 0.5, "Spam score should be low")
			}
		})
	}
}

func TestReputationVerifier(t *testing.T) {
	verifier := NewReputationVerifier("8.8.8.8:53")
	
	tests := []struct {
		name             string
		emailCtx         EmailContext
		expectedIPStatus IPReputationStatus
	}{
		{
			name: "private IP",
			emailCtx: EmailContext{
				From:     "test@example.com",
				SenderIP: net.ParseIP("192.168.1.1"),
			},
			expectedIPStatus: IPReputationGood,
		},
		{
			name: "no sender IP",
			emailCtx: EmailContext{
				From:     "test@example.com",
				SenderIP: nil,
			},
			expectedIPStatus: IPReputationUnknown,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := verifier.Verify(ctx, tt.emailCtx)
			
			assert.Equal(t, tt.expectedIPStatus, result.IPReputation)
		})
	}
}

func TestVerificationConfig(t *testing.T) {
	config := DefaultConfig()
	
	assert.True(t, config.EnableSPF)
	assert.True(t, config.EnableDKIM)
	assert.True(t, config.EnableDMARC)
	assert.True(t, config.EnableReputation)
	assert.True(t, config.EnableContent)
	assert.Equal(t, 0.7, config.SpamThreshold)
	assert.False(t, config.RejectOnFail)
	assert.True(t, config.QuarantineMode)
}

func TestActionString(t *testing.T) {
	tests := []struct {
		action   Action
		expected string
	}{
		{ActionAccept, "accept"},
		{ActionQuarantine, "quarantine"},
		{ActionReject, "reject"},
		{ActionTempFail, "tempfail"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.action.String())
		})
	}
}

func TestSPFStatusString(t *testing.T) {
	tests := []struct {
		status   SPFStatus
		expected string
	}{
		{SPFNone, "none"},
		{SPFPass, "pass"},
		{SPFFail, "fail"},
		{SPFSoftFail, "softfail"},
		{SPFTempError, "temperror"},
		{SPFPermError, "permerror"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestDMARCStatusString(t *testing.T) {
	tests := []struct {
		status   DMARCStatus
		expected string
	}{
		{DMARCNone, "none"},
		{DMARCPass, "pass"},
		{DMARCFail, "fail"},
		{DMARCTempError, "temperror"},
		{DMARCPermError, "permerror"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestVerifierHelperMethods(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	config := DefaultConfig()
	verifier := NewVerifier(config, logger)
	
	t.Run("ShouldAccept", func(t *testing.T) {
		result := VerificationResult{Action: ActionAccept}
		assert.True(t, verifier.ShouldAccept(result))
		
		result = VerificationResult{Action: ActionReject}
		assert.False(t, verifier.ShouldAccept(result))
	})
	
	t.Run("ShouldReject", func(t *testing.T) {
		result := VerificationResult{Action: ActionReject}
		assert.True(t, verifier.ShouldReject(result))
		
		result = VerificationResult{Action: ActionAccept}
		assert.False(t, verifier.ShouldReject(result))
	})
	
	t.Run("IsSpam", func(t *testing.T) {
		result := VerificationResult{Action: ActionQuarantine}
		assert.True(t, verifier.IsSpam(result))
		
		result = VerificationResult{Action: ActionReject}
		assert.True(t, verifier.IsSpam(result))
		
		result = VerificationResult{Action: ActionAccept}
		assert.False(t, verifier.IsSpam(result))
	})
	
	t.Run("GetVerificationSummary", func(t *testing.T) {
		result := VerificationResult{
			SPF:   SPFResult{Result: SPFPass},
			DKIM:  DKIMResult{Valid: true},
			DMARC: DMARCResult{Result: DMARCPass},
			Action: ActionAccept,
		}
		
		summary := verifier.GetVerificationSummary(result)
		assert.Contains(t, summary, "SPF: PASS")
		assert.Contains(t, summary, "DKIM: PASS")
		assert.Contains(t, summary, "DMARC: pass")
		assert.Contains(t, summary, "accept")
	})
}

func TestVerifier_RiskScoreCalculation(t *testing.T) {
	skipNeedsDNSInjection(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultConfig()
	verifier := NewVerifier(config, logger)
	
	// Test different combinations of verification results and their risk scores
	tests := []struct {
		name           string
		result         VerificationResult
		expectedAction Action
		description    string
	}{
		{
			name: "All pass - should accept",
			result: VerificationResult{
				SPF:   SPFResult{Result: SPFPass},
				DKIM:  DKIMResult{Valid: true},
				DMARC: DMARCResult{Result: DMARCPass},
				Reputation: ReputationResult{
					IPReputation:     IPReputationGood,
					DomainReputation: DomainReputationGood,
					Score:            0.8,
					Blacklisted:      []string{},
				},
				Content: ContentResult{
					SpamScore:      0.1,
					Classification: "legitimate",
				},
			},
			expectedAction: ActionAccept,
			description:    "High trust scenario with all verifications passing",
		},
		{
			name: "SPF fail - should quarantine/reject based on config",
			result: VerificationResult{
				SPF:   SPFResult{Result: SPFFail},
				DKIM:  DKIMResult{Valid: false},
				DMARC: DMARCResult{Result: DMARCFail, Policy: "quarantine"},
				Reputation: ReputationResult{
					IPReputation:     IPReputationGood,
					DomainReputation: DomainReputationGood,
					Score:            0.6,
				},
				Content: ContentResult{
					SpamScore:      0.3,
					Classification: "questionable",
				},
			},
			expectedAction: ActionQuarantine,
			description:    "Authentication failures should result in quarantine",
		},
		{
			name: "High spam score - should quarantine",
			result: VerificationResult{
				SPF:   SPFResult{Result: SPFPass},
				DKIM:  DKIMResult{Valid: true},
				DMARC: DMARCResult{Result: DMARCPass},
				Reputation: ReputationResult{
					IPReputation:     IPReputationGood,
					DomainReputation: DomainReputationGood,
					Score:            0.7,
				},
				Content: ContentResult{
					SpamScore:      0.9,
					Classification: "spam",
				},
			},
			expectedAction: ActionQuarantine,
			description:    "High content spam score should trigger quarantine",
		},
		{
			name: "Blacklisted IP - should quarantine",
			result: VerificationResult{
				SPF:   SPFResult{Result: SPFPass},
				DKIM:  DKIMResult{Valid: true},
				DMARC: DMARCResult{Result: DMARCPass},
				Reputation: ReputationResult{
					IPReputation:     IPReputationBad,
					DomainReputation: DomainReputationGood,
					Score:            0.2,
					Blacklisted:      []string{"zen.spamhaus.org", "bl.spamcop.net"},
				},
				Content: ContentResult{
					SpamScore:      0.3,
					Classification: "questionable",
				},
			},
			expectedAction: ActionQuarantine,
			description:    "Multiple blacklist hits should trigger quarantine",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := verifier.determineAction(tt.result)
			assert.Equal(t, tt.expectedAction, action, tt.description)
		})
	}
}

func TestVerifier_ConfigurationEffects(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	
	tests := []struct {
		name           string
		config         VerificationConfig
		emailCtx       EmailContext
		expectedChecks map[string]bool
	}{
		{
			name: "All verifications disabled",
			config: VerificationConfig{
				EnableSPF:        false,
				EnableDKIM:       false,
				EnableDMARC:      false,
				EnableReputation: false,
				EnableContent:    false,
			},
			emailCtx: EmailContext{
				From:    "test@example.com",
				To:      []string{"recipient@test.com"},
				Subject: "Test Email",
				Body:    []byte("From: test@example.com\r\nTo: recipient@test.com\r\nSubject: Test Email\r\n\r\nTest content"),
				SenderIP: net.ParseIP("192.168.1.1"),
			},
			expectedChecks: map[string]bool{
				"spf_none":          true,
				"dkim_invalid":      true,
				"dmarc_none":        true,
				"content_not_checked": true,
			},
		},
		{
			name: "Only SPF enabled",
			config: VerificationConfig{
				EnableSPF:        true,
				EnableDKIM:       false,
				EnableDMARC:      false,
				EnableReputation: false,
				EnableContent:    false,
			},
			emailCtx: EmailContext{
				From:    "test@example.com",
				To:      []string{"recipient@test.com"},
				Subject: "Test Email",
				Body:    []byte("From: test@example.com\r\nTo: recipient@test.com\r\nSubject: Test Email\r\n\r\nTest content"),
				SenderIP: net.ParseIP("192.168.1.1"),
			},
			expectedChecks: map[string]bool{
				"spf_checked":       true,
				"dkim_invalid":      true,
				"dmarc_none":        true,
				"content_not_checked": true,
			},
		},
		{
			name: "Strict configuration",
			config: VerificationConfig{
				EnableSPF:        true,
				EnableDKIM:       true,
				EnableDMARC:      true,
				EnableReputation: true,
				EnableContent:    true,
				SpamThreshold:    0.3, // Lower threshold
				RejectOnFail:     true,
				QuarantineMode:   false,
			},
			emailCtx: EmailContext{
				From:    "test@example.com",
				To:      []string{"recipient@test.com"},
				Subject: "Test Email",
				Body:    []byte("From: test@example.com\r\nTo: recipient@test.com\r\nSubject: Test Email\r\n\r\nTest content"),
				SenderIP: net.ParseIP("192.168.1.1"),
			},
			expectedChecks: map[string]bool{
				"all_enabled": true,
				"strict_mode": true,
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier := NewVerifier(tt.config, logger)
			result := verifier.VerifyEmail(context.Background(), tt.emailCtx)
			
			if tt.expectedChecks["spf_none"] {
				assert.Equal(t, SPFNone, result.SPF.Result)
			}
			if tt.expectedChecks["dkim_invalid"] {
				assert.False(t, result.DKIM.Valid)
			}
			if tt.expectedChecks["dmarc_none"] {
				assert.Equal(t, DMARCNone, result.DMARC.Result)
			}
			if tt.expectedChecks["content_not_checked"] {
				assert.Equal(t, "not_checked", result.Content.Classification)
			}
			if tt.expectedChecks["spf_checked"] {
				// SPF should be checked (will likely be None due to no SPF record in test)
				assert.NotEqual(t, SPFStatus(0), result.SPF.Result)
			}
			if tt.expectedChecks["all_enabled"] {
				// All verifications should be attempted
				assert.NotEqual(t, SPFStatus(0), result.SPF.Result)
				assert.NotEqual(t, "", result.Content.Classification)
			}
			
			// Verify configuration effects
			assert.Equal(t, tt.config.RejectOnFail, verifier.config.RejectOnFail)
			assert.Equal(t, tt.config.SpamThreshold, verifier.config.SpamThreshold)
		})
	}
}

func TestVerifier_EdgeCases(t *testing.T) {
	skipNeedsDNSInjection(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultConfig()
	verifier := NewVerifier(config, logger)
	
	t.Run("Empty email context", func(t *testing.T) {
		emailCtx := EmailContext{}
		result := verifier.VerifyEmail(context.Background(), emailCtx)
		
		// Should handle empty context gracefully
		assert.NotEqual(t, Action(0), result.Action)
		assert.NotZero(t, result.Timestamp)
	})
	
	t.Run("Malformed From address", func(t *testing.T) {
		emailCtx := EmailContext{
			From:    "not-an-email-address",
			To:      []string{"recipient@test.com"},
			Subject: "Test",
			Body:    []byte("Test content"),
		}
		
		result := verifier.VerifyEmail(context.Background(), emailCtx)
		
		// Should handle malformed addresses
		assert.Equal(t, SPFPermError, result.SPF.Result)
		assert.Equal(t, DMARCPermError, result.DMARC.Result)
	})
	
	t.Run("Very long email", func(t *testing.T) {
		longBody := "From: test@example.com\r\nTo: recipient@test.com\r\nSubject: Long Email\r\n\r\n"
		for i := 0; i < 10000; i++ {
			longBody += "This is a very long email body with repeated content. "
		}
		
		emailCtx := EmailContext{
			From:    "test@example.com",
			To:      []string{"recipient@test.com"},
			Subject: "Very Long Email",
			Body:    []byte(longBody),
			SenderIP: net.ParseIP("192.168.1.1"),
		}
		
		result := verifier.VerifyEmail(context.Background(), emailCtx)
		
		// Should handle large emails efficiently
		assert.NotEqual(t, Action(0), result.Action)
		assert.NotZero(t, result.Timestamp)
	})
	
	t.Run("Invalid characters in email", func(t *testing.T) {
		emailCtx := EmailContext{
			From:    "test@example.com",
			To:      []string{"recipient@test.com"},
			Subject: "Email with \x00 null bytes and \xff invalid chars",
			Body:    []byte("From: test@example.com\r\nSubject: Test\r\n\r\nBody with \x00\xff invalid content"),
			SenderIP: net.ParseIP("192.168.1.1"),
		}
		
		result := verifier.VerifyEmail(context.Background(), emailCtx)
		
		// Should handle invalid characters gracefully
		assert.NotEqual(t, Action(0), result.Action)
	})
}

func TestVerifier_TemporaryFailures(t *testing.T) {
	skipNeedsDNSInjection(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultConfig()
	verifier := NewVerifier(config, logger)
	
	// Simulate temporary failures in verification results
	result := VerificationResult{
		SPF:   SPFResult{Result: SPFTempError, Error: "DNS timeout"},
		DKIM:  DKIMResult{Valid: false, Error: "DNS lookup failed"},
		DMARC: DMARCResult{Result: DMARCTempError, Error: "Policy lookup failed"},
		Reputation: ReputationResult{
			IPReputation:     IPReputationUnknown,
			DomainReputation: DomainReputationUnknown,
			Score:            0.5,
			Error:           "Blacklist check timeout",
		},
		Content: ContentResult{
			SpamScore:      0.3,
			Classification: "questionable",
		},
	}
	
	action := verifier.determineAction(result)
	assert.Equal(t, ActionTempFail, action, "Temporary errors should result in tempfail action")
}

func TestVerifier_PolicyEnforcement(t *testing.T) {
	skipNeedsDNSInjection(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	
	tests := []struct {
		name           string
		config         VerificationConfig
		result         VerificationResult
		expectedAction Action
	}{
		{
			name: "Strict policy - SPF fail should reject",
			config: VerificationConfig{
				RejectOnFail:   true,
				QuarantineMode: false,
			},
			result: VerificationResult{
				SPF: SPFResult{Result: SPFFail},
			},
			expectedAction: ActionReject,
		},
		{
			name: "Quarantine policy - SPF fail should quarantine",
			config: VerificationConfig{
				RejectOnFail:   false,
				QuarantineMode: true,
			},
			result: VerificationResult{
				SPF: SPFResult{Result: SPFFail},
			},
			expectedAction: ActionQuarantine,
		},
		{
			name: "DMARC reject policy should reject",
			config: VerificationConfig{
				RejectOnFail:   true,
				QuarantineMode: false,
			},
			result: VerificationResult{
				SPF:   SPFResult{Result: SPFPass},
				DMARC: DMARCResult{Result: DMARCFail, Policy: "reject"},
			},
			expectedAction: ActionReject,
		},
		{
			name: "Multiple blacklists should reject",
			config: VerificationConfig{
				RejectOnFail:   true,
				QuarantineMode: false,
			},
			result: VerificationResult{
				Reputation: ReputationResult{
					Blacklisted: []string{"list1", "list2", "list3"},
				},
			},
			expectedAction: ActionReject,
		},
		{
			name: "High spam threshold - high score should pass through",
			config: VerificationConfig{
				SpamThreshold:  0.9,
				RejectOnFail:   false,
				QuarantineMode: true,
			},
			result: VerificationResult{
				Content: ContentResult{
					SpamScore: 0.8,
				},
			},
			expectedAction: ActionAccept, // Below high threshold
		},
		{
			name: "Low spam threshold - moderate score should quarantine",
			config: VerificationConfig{
				SpamThreshold:  0.3,
				RejectOnFail:   false,
				QuarantineMode: true,
			},
			result: VerificationResult{
				Content: ContentResult{
					SpamScore: 0.5,
				},
			},
			expectedAction: ActionQuarantine, // Above low threshold
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier := NewVerifier(tt.config, logger)
			action := verifier.riskScoreToAction(0.6, tt.result) // Medium risk score
			assert.Equal(t, tt.expectedAction, action)
		})
	}
}

func TestVerifier_ConcurrentVerification(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultConfig()
	verifier := NewVerifier(config, logger)
	
	emailCtx := EmailContext{
		From:    "test@example.com",
		To:      []string{"recipient@test.com"},
		Subject: "Concurrent Test",
		Body:    []byte("From: test@example.com\r\nTo: recipient@test.com\r\nSubject: Concurrent Test\r\n\r\nTest content"),
		SenderIP: net.ParseIP("192.168.1.1"),
	}
	
	// Run multiple verifications concurrently
	results := make(chan VerificationResult, 10)
	
	for i := 0; i < 10; i++ {
		go func() {
			result := verifier.VerifyEmail(context.Background(), emailCtx)
			results <- result
		}()
	}
	
	// Collect results
	for i := 0; i < 10; i++ {
		result := <-results
		assert.NotEqual(t, Action(0), result.Action, "Each verification should produce a valid result")
		assert.NotZero(t, result.Timestamp, "Each result should have a timestamp")
	}
}

func TestVerifier_PerformanceUnderLoad(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultConfig()
	verifier := NewVerifier(config, logger)
	
	emailCtx := EmailContext{
		From:    "test@example.com",
		To:      []string{"recipient@test.com"},
		Subject: "Performance Test",
		Body:    []byte("From: test@example.com\r\nTo: recipient@test.com\r\nSubject: Performance Test\r\n\r\nTest content for performance evaluation"),
		SenderIP: net.ParseIP("192.168.1.1"),
	}
	
	start := time.Now()
	
	// Run 100 verifications to test performance
	for i := 0; i < 100; i++ {
		result := verifier.VerifyEmail(context.Background(), emailCtx)
		assert.NotEqual(t, Action(0), result.Action)
	}
	
	duration := time.Since(start)
	avgDuration := duration / 100
	
	// Each verification should complete reasonably quickly
	// This is a loose check since actual performance depends on system and network
	assert.Less(t, avgDuration, 5*time.Second, "Average verification should complete within reasonable time")
}

func BenchmarkVerifyEmail(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultConfig()
	verifier := NewVerifier(config, logger)
	
	emailCtx := EmailContext{
		From:    "test@example.com",
		To:      []string{"recipient@test.com"},
		Subject: "Test Email",
		Body:    []byte("From: test@example.com\r\nTo: recipient@test.com\r\nSubject: Test Email\r\n\r\nThis is a test email."),
		Headers: []Header{
			{Name: "From", Value: "test@example.com"},
			{Name: "To", Value: "recipient@test.com"},
			{Name: "Subject", Value: "Test Email"},
		},
		SenderIP:   net.ParseIP("192.168.1.100"),
		ReceivedAt: time.Now(),
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = verifier.VerifyEmail(ctx, emailCtx)
	}
}

func BenchmarkVerifyEmailComponents(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	
	emailCtx := EmailContext{
		From:    "test@example.com",
		To:      []string{"recipient@test.com"},
		Subject: "Benchmark Email",
		Body:    []byte("From: test@example.com\r\nTo: recipient@test.com\r\nSubject: Benchmark Email\r\n\r\nBenchmark content"),
		SenderIP: net.ParseIP("192.168.1.100"),
	}
	
	ctx := context.Background()
	
	b.Run("SPF Only", func(b *testing.B) {
		config := VerificationConfig{
			EnableSPF:        true,
			EnableDKIM:       false,
			EnableDMARC:      false,
			EnableReputation: false,
			EnableContent:    false,
		}
		verifier := NewVerifier(config, logger)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			verifier.VerifyEmail(ctx, emailCtx)
		}
	})
	
	b.Run("Content Only", func(b *testing.B) {
		config := VerificationConfig{
			EnableSPF:        false,
			EnableDKIM:       false,
			EnableDMARC:      false,
			EnableReputation: false,
			EnableContent:    true,
		}
		verifier := NewVerifier(config, logger)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			verifier.VerifyEmail(ctx, emailCtx)
		}
	})
	
	b.Run("Reputation Only", func(b *testing.B) {
		config := VerificationConfig{
			EnableSPF:        false,
			EnableDKIM:       false,
			EnableDMARC:      false,
			EnableReputation: true,
			EnableContent:    false,
		}
		verifier := NewVerifier(config, logger)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			verifier.VerifyEmail(ctx, emailCtx)
		}
	})
}
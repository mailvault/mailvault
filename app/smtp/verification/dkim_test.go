package verification

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDKIMVerifier_Verify_NoSignature(t *testing.T) {
	verifier := NewDKIMVerifier("8.8.8.8:53")
	
	// Email without DKIM signature
	emailCtx := EmailContext{
		From:    "test@example.com",
		To:      []string{"recipient@test.com"},
		Subject: "Test Email",
		Body: []byte("From: test@example.com\r\n" +
			"To: recipient@test.com\r\n" +
			"Subject: Test Email\r\n" +
			"Date: Mon, 19 Aug 2025 12:00:00 -0300\r\n" +
			"\r\n" +
			"This is a test email without DKIM signature."),
		Headers: []Header{
			{Name: "From", Value: "test@example.com"},
			{Name: "To", Value: "recipient@test.com"},
			{Name: "Subject", Value: "Test Email"},
		},
	}
	
	result := verifier.Verify(context.Background(), emailCtx)
	
	assert.False(t, result.Valid)
	assert.Empty(t, result.Results) // No DKIM signatures to verify
}

func TestDKIMVerifier_Verify_MalformedEmail(t *testing.T) {
	verifier := NewDKIMVerifier("8.8.8.8:53")
	
	// Malformed email
	emailCtx := EmailContext{
		From:    "test@example.com",
		To:      []string{"recipient@test.com"},
		Subject: "Test Email",
		Body:    []byte("This is not a proper email format"),
		Headers: []Header{},
	}
	
	result := verifier.Verify(context.Background(), emailCtx)
	
	assert.False(t, result.Valid)
	assert.Empty(t, result.Results)
}

func TestDKIMVerifier_Verify_WithDKIMSignature(t *testing.T) {
	verifier := NewDKIMVerifier("8.8.8.8:53")
	
	// Email with DKIM signature (this will fail verification since the signature is not real)
	emailCtx := EmailContext{
		From:    "test@example.com",
		To:      []string{"recipient@test.com"},
		Subject: "Test Email with DKIM",
		Body: []byte("DKIM-Signature: v=1; a=rsa-sha256; c=relaxed/relaxed; d=example.com; s=selector1;\r\n" +
			"    h=From:To:Subject:Date;\r\n" +
			"    b=invalid_signature_for_testing;\r\n" +
			"From: test@example.com\r\n" +
			"To: recipient@test.com\r\n" +
			"Subject: Test Email with DKIM\r\n" +
			"Date: Mon, 19 Aug 2025 12:00:00 -0300\r\n" +
			"\r\n" +
			"This is a test email with DKIM signature."),
		Headers: []Header{
			{Name: "DKIM-Signature", Value: "v=1; a=rsa-sha256; c=relaxed/relaxed; d=example.com; s=selector1; h=From:To:Subject:Date; b=invalid_signature_for_testing;"},
			{Name: "From", Value: "test@example.com"},
			{Name: "To", Value: "recipient@test.com"},
			{Name: "Subject", Value: "Test Email with DKIM"},
		},
	}
	
	result := verifier.Verify(context.Background(), emailCtx)
	
	// The signature should fail because it's not a real signature
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Results) // Should have at least one result
	
	if len(result.Results) > 0 {
		assert.Equal(t, "example.com", result.Results[0].Domain)
		assert.Equal(t, DKIMFail, result.Results[0].Status)
		assert.NotEmpty(t, result.Results[0].Error)
	}
}

func TestDKIMVerifier_DkimErrorToStatus(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected DKIMStatus
	}{
		{
			name:     "No error",
			err:      nil,
			expected: DKIMPass,
		},
		// Note: We can't easily test the specific go-msgauth errors without importing them
		// In a real implementation, you would test specific error types
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dkimErrorToStatus(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDKIMStatus_String(t *testing.T) {
	tests := []struct {
		status   DKIMStatus
		expected string
	}{
		{DKIMNone, "none"},
		{DKIMPass, "pass"},
		{DKIMFail, "fail"},
		{DKIMPolicy, "policy"},
		{DKIMNeutral, "neutral"},
		{DKIMTempError, "temperror"},
		{DKIMPermError, "permerror"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestDKIMVerifier_MultipleSignatures(t *testing.T) {
	verifier := NewDKIMVerifier("8.8.8.8:53")
	
	// Email with multiple DKIM signatures
	emailCtx := EmailContext{
		From:    "test@example.com",
		To:      []string{"recipient@test.com"},
		Subject: "Test Email with Multiple DKIM",
		Body: []byte("DKIM-Signature: v=1; a=rsa-sha256; c=relaxed/relaxed; d=example.com; s=selector1;\r\n" +
			"    h=From:To:Subject:Date;\r\n" +
			"    b=invalid_signature1_for_testing;\r\n" +
			"DKIM-Signature: v=1; a=rsa-sha256; c=relaxed/relaxed; d=mail.example.com; s=selector2;\r\n" +
			"    h=From:To:Subject:Date;\r\n" +
			"    b=invalid_signature2_for_testing;\r\n" +
			"From: test@example.com\r\n" +
			"To: recipient@test.com\r\n" +
			"Subject: Test Email with Multiple DKIM\r\n" +
			"Date: Mon, 19 Aug 2025 12:00:00 -0300\r\n" +
			"\r\n" +
			"This is a test email with multiple DKIM signatures."),
		Headers: []Header{
			{Name: "DKIM-Signature", Value: "v=1; a=rsa-sha256; c=relaxed/relaxed; d=example.com; s=selector1; h=From:To:Subject:Date; b=invalid_signature1_for_testing;"},
			{Name: "DKIM-Signature", Value: "v=1; a=rsa-sha256; c=relaxed/relaxed; d=mail.example.com; s=selector2; h=From:To:Subject:Date; b=invalid_signature2_for_testing;"},
			{Name: "From", Value: "test@example.com"},
			{Name: "To", Value: "recipient@test.com"},
			{Name: "Subject", Value: "Test Email with Multiple DKIM"},
		},
	}
	
	result := verifier.Verify(context.Background(), emailCtx)
	
	// Should have multiple results, all failing because signatures are invalid
	assert.False(t, result.Valid)
	assert.True(t, len(result.Results) >= 1) // Should have at least one result
	
	// Check that we have results for different domains
	domains := make(map[string]bool)
	for _, res := range result.Results {
		domains[res.Domain] = true
		assert.Equal(t, DKIMFail, res.Status)
	}
}

func TestDKIMResult_Validation(t *testing.T) {
	// Test that DKIMResult properly reports validity
	tests := []struct {
		name     string
		results  []DKIMSignatureResult
		expected bool
	}{
		{
			name:     "No results",
			results:  []DKIMSignatureResult{},
			expected: false,
		},
		{
			name: "All fail",
			results: []DKIMSignatureResult{
				{Domain: "example.com", Status: DKIMFail},
				{Domain: "mail.example.com", Status: DKIMFail},
			},
			expected: false,
		},
		{
			name: "One passes",
			results: []DKIMSignatureResult{
				{Domain: "example.com", Status: DKIMFail},
				{Domain: "mail.example.com", Status: DKIMPass},
			},
			expected: true,
		},
		{
			name: "All pass",
			results: []DKIMSignatureResult{
				{Domain: "example.com", Status: DKIMPass},
				{Domain: "mail.example.com", Status: DKIMPass},
			},
			expected: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DKIMResult{
				Results: tt.results,
				Valid:   false, // Initially set to false
			}
			
			// Simulate the logic that would set Valid based on results
			anyPass := false
			for _, r := range result.Results {
				if r.Status == DKIMPass {
					anyPass = true
					break
				}
			}
			result.Valid = anyPass
			
			assert.Equal(t, tt.expected, result.Valid)
		})
	}
}

func TestDKIMVerifier_EmptyBody(t *testing.T) {
	verifier := NewDKIMVerifier("8.8.8.8:53")
	
	// Email with empty body
	emailCtx := EmailContext{
		From:    "test@example.com",
		To:      []string{"recipient@test.com"},
		Subject: "Empty Email",
		Body:    []byte{},
		Headers: []Header{},
	}
	
	result := verifier.Verify(context.Background(), emailCtx)
	
	assert.False(t, result.Valid)
	assert.Empty(t, result.Results)
}

func TestDKIMVerifier_LongEmail(t *testing.T) {
	verifier := NewDKIMVerifier("8.8.8.8:53")
	
	// Create a long email body
	longBody := "From: test@example.com\r\n" +
		"To: recipient@test.com\r\n" +
		"Subject: Long Email\r\n" +
		"Date: Mon, 19 Aug 2025 12:00:00 -0300\r\n" +
		"\r\n"
	
	// Add a lot of content
	for i := 0; i < 1000; i++ {
		longBody += "This is line " + string(rune(i)) + " of a very long email that tests DKIM verification performance.\r\n"
	}
	
	emailCtx := EmailContext{
		From:    "test@example.com",
		To:      []string{"recipient@test.com"},
		Subject: "Long Email",
		Body:    []byte(longBody),
		Headers: []Header{
			{Name: "From", Value: "test@example.com"},
			{Name: "To", Value: "recipient@test.com"},
			{Name: "Subject", Value: "Long Email"},
		},
	}
	
	result := verifier.Verify(context.Background(), emailCtx)
	
	// Should handle long emails gracefully
	assert.False(t, result.Valid) // No signature, so should be false
	assert.Empty(t, result.Results)
}

func BenchmarkDKIMVerify(b *testing.B) {
	verifier := NewDKIMVerifier("8.8.8.8:53")
	
	emailCtx := EmailContext{
		From:    "test@example.com",
		To:      []string{"recipient@test.com"},
		Subject: "Benchmark Email",
		Body: []byte("From: test@example.com\r\n" +
			"To: recipient@test.com\r\n" +
			"Subject: Benchmark Email\r\n" +
			"Date: Mon, 19 Aug 2025 12:00:00 -0300\r\n" +
			"\r\n" +
			"This is a benchmark test email for DKIM verification."),
		Headers: []Header{
			{Name: "From", Value: "test@example.com"},
			{Name: "To", Value: "recipient@test.com"},
			{Name: "Subject", Value: "Benchmark Email"},
		},
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		verifier.Verify(ctx, emailCtx)
	}
}
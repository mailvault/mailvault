package verification

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockReputationExchanger implements a mock DNS client for reputation testing
type MockReputationExchanger struct {
	mock.Mock
}

func (m *MockReputationExchanger) ExchangeContext(ctx context.Context, msg *dns.Msg, addr string) (*dns.Msg, time.Duration, error) {
	args := m.Called(ctx, msg, addr)
	return args.Get(0).(*dns.Msg), args.Get(1).(time.Duration), args.Error(2)
}

// MockReputationVerifier extends ReputationVerifier to use mock DNS client
type MockReputationVerifier struct {
	*ReputationVerifier
	mockExchanger *MockReputationExchanger
}

func NewMockReputationVerifier() *MockReputationVerifier {
	mockExchanger := &MockReputationExchanger{}
	verifier := NewReputationVerifier("8.8.8.8:53")
	
	return &MockReputationVerifier{
		ReputationVerifier: verifier,
		mockExchanger:      mockExchanger,
	}
}

// Override client methods to use mock exchanger
func (m *MockReputationVerifier) checkIPBlacklist(ctx context.Context, ip net.IP, blacklist string) bool {
	reversedIP := m.reverseIP(ip)
	if reversedIP == "" {
		return false
	}
	
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(reversedIP+"."+blacklist), dns.TypeA)
	
	resp, _, err := m.mockExchanger.ExchangeContext(ctx, msg, m.resolver)
	if err != nil {
		return false
	}
	
	return resp.Rcode == dns.RcodeSuccess && len(resp.Answer) > 0
}

func TestReputationVerifier_Verify_NoSenderIP(t *testing.T) {
	verifier := NewReputationVerifier("8.8.8.8:53")
	
	emailCtx := EmailContext{
		From:     "test@example.com",
		SenderIP: nil,
	}
	
	result := verifier.Verify(context.Background(), emailCtx)
	
	assert.Equal(t, IPReputationUnknown, result.IPReputation)
	assert.Equal(t, DomainReputationGood, result.DomainReputation) // Domain should still be checked
	assert.Equal(t, 0.5, result.Score) // Base neutral score
}

func TestReputationVerifier_Verify_PrivateIP(t *testing.T) {
	verifier := NewReputationVerifier("8.8.8.8:53")
	
	emailCtx := EmailContext{
		From:     "test@example.com",
		SenderIP: net.ParseIP("192.168.1.100"),
	}
	
	result := verifier.Verify(context.Background(), emailCtx)
	
	assert.Equal(t, IPReputationGood, result.IPReputation)
	assert.Greater(t, result.Score, 0.5) // Should be slightly positive
}

func TestReputationVerifier_Verify_BlacklistedIP(t *testing.T) {
	mockVerifier := NewMockReputationVerifier()
	
	// Mock DNS responses for blacklist queries (all return positive)
	positiveResp := &dns.Msg{
		MsgHdr: dns.MsgHdr{Rcode: dns.RcodeSuccess},
		Answer: []dns.RR{
			&dns.A{Hdr: dns.RR_Header{Name: "test", Rrtype: dns.TypeA}, A: net.ParseIP("127.0.0.2")},
		},
	}
	
	mockVerifier.mockExchanger.On("ExchangeContext", mock.Anything, mock.Anything, mock.Anything).
		Return(positiveResp, time.Duration(0), nil)
	
	emailCtx := EmailContext{
		From:     "test@example.com",
		SenderIP: net.ParseIP("1.2.3.4"), // Public IP
	}
	
	result := mockVerifier.Verify(context.Background(), emailCtx)
	
	assert.Equal(t, IPReputationBad, result.IPReputation)
	assert.NotEmpty(t, result.Blacklisted)
	assert.Less(t, result.Score, 0.5) // Should be negative
	mockVerifier.mockExchanger.AssertExpectations(t)
}

func TestReputationVerifier_Verify_CleanIP(t *testing.T) {
	mockVerifier := NewMockReputationVerifier()
	
	// Mock DNS responses for blacklist queries (all return negative)
	negativeResp := &dns.Msg{
		MsgHdr: dns.MsgHdr{Rcode: dns.RcodeNameError},
		Answer: []dns.RR{},
	}
	
	mockVerifier.mockExchanger.On("ExchangeContext", mock.Anything, mock.Anything, mock.Anything).
		Return(negativeResp, time.Duration(0), nil)
	
	emailCtx := EmailContext{
		From:     "test@example.com",
		SenderIP: net.ParseIP("8.8.8.8"), // Google DNS, should be clean
	}
	
	result := mockVerifier.Verify(context.Background(), emailCtx)
	
	assert.Equal(t, IPReputationGood, result.IPReputation)
	assert.Empty(t, result.Blacklisted)
	assert.Greater(t, result.Score, 0.5) // Should be positive
	mockVerifier.mockExchanger.AssertExpectations(t)
}

func TestReputationVerifier_IsPrivateIP(t *testing.T) {
	verifier := NewReputationVerifier("8.8.8.8:53")
	
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"Private 192.168", "192.168.1.1", true},
		{"Private 10.x", "10.0.0.1", true},
		{"Private 172.16", "172.16.0.1", true},
		{"Loopback", "127.0.0.1", true},
		{"Link-local", "169.254.1.1", true},
		{"Public Google DNS", "8.8.8.8", false},
		{"Public Cloudflare DNS", "1.1.1.1", false},
		{"IPv6 loopback", "::1", false}, // Not handled in simplified implementation
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			assert.NotNil(t, ip, "Should parse IP successfully")
			
			result := verifier.isPrivateIP(ip)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReputationVerifier_ReverseIP(t *testing.T) {
	verifier := NewReputationVerifier("8.8.8.8:53")
	
	tests := []struct {
		name     string
		ip       string
		expected string
	}{
		{"IPv4 simple", "1.2.3.4", "4.3.2.1"},
		{"IPv4 Google DNS", "8.8.8.8", "8.8.8.8"},
		{"IPv4 private", "192.168.1.100", "100.1.168.192"},
		{"IPv6", "2001:db8::1", ""}, // Not implemented in simplified version
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			assert.NotNil(t, ip, "Should parse IP successfully")
			
			result := verifier.reverseIP(ip)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReputationVerifier_HasSuspiciousTLD(t *testing.T) {
	verifier := NewReputationVerifier("8.8.8.8:53")
	
	tests := []struct {
		domain   string
		expected bool
	}{
		{"example.com", false},
		{"google.com", false},
		{"test.org", false},
		{"suspicious.tk", true},
		{"spam.ml", true},
		{"malware.ga", true},
		{"phishing.cf", true},
		{"hidden.onion", true},
		{"crypto.bit", true},
		{"UPPERCASE.TK", true}, // Should handle case insensitivity
	}
	
	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			result := verifier.hasSuspiciousTLD(tt.domain)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReputationVerifier_CheckIPReputation_Gradual(t *testing.T) {
	mockVerifier := NewMockReputationVerifier()
	
	tests := []struct {
		name              string
		blacklistCount    int
		expectedStatus    IPReputationStatus
		expectNegativeScore bool
	}{
		{
			name:              "No blacklists",
			blacklistCount:    0,
			expectedStatus:    IPReputationGood,
			expectNegativeScore: false,
		},
		{
			name:              "One blacklist",
			blacklistCount:    1,
			expectedStatus:    IPReputationSuspicious,
			expectNegativeScore: true,
		},
		{
			name:              "Two blacklists",
			blacklistCount:    2,
			expectedStatus:    IPReputationSuspicious,
			expectNegativeScore: true,
		},
		{
			name:              "Three blacklists",
			blacklistCount:    3,
			expectedStatus:    IPReputationBad,
			expectNegativeScore: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock for each test
			mockVerifier.mockExchanger = &MockReputationExchanger{}
			
			callCount := 0
			mockVerifier.mockExchanger.On("ExchangeContext", mock.Anything, mock.Anything, mock.Anything).
				Return(func(ctx context.Context, msg *dns.Msg, addr string) *dns.Msg {
					callCount++
					if callCount <= tt.blacklistCount {
						// Return positive response (blacklisted)
						return &dns.Msg{
							MsgHdr: dns.MsgHdr{Rcode: dns.RcodeSuccess},
							Answer: []dns.RR{
								&dns.A{Hdr: dns.RR_Header{Name: "test", Rrtype: dns.TypeA}, A: net.ParseIP("127.0.0.2")},
							},
						}
					} else {
						// Return negative response (not blacklisted)
						return &dns.Msg{
							MsgHdr: dns.MsgHdr{Rcode: dns.RcodeNameError},
							Answer: []dns.RR{},
						}
					}
				}, time.Duration(0), nil)
			
			ip := net.ParseIP("1.2.3.4")
			result := mockVerifier.checkIPReputation(context.Background(), ip)
			
			assert.Equal(t, tt.expectedStatus, result.status)
			assert.Len(t, result.blacklists, tt.blacklistCount)
			
			if tt.expectNegativeScore {
				assert.Less(t, result.scoreAdjustment, 0.0)
			} else {
				assert.Greater(t, result.scoreAdjustment, 0.0)
			}
		})
	}
}

func TestReputationVerifier_CheckDomainReputation(t *testing.T) {
	mockVerifier := NewMockReputationVerifier()
	
	// Mock all DNS queries to return negative (clean)
	negativeResp := &dns.Msg{
		MsgHdr: dns.MsgHdr{Rcode: dns.RcodeNameError},
		Answer: []dns.RR{},
	}
	
	mockVerifier.mockExchanger.On("ExchangeContext", mock.Anything, mock.Anything, mock.Anything).
		Return(negativeResp, time.Duration(0), nil)
	
	result := mockVerifier.checkDomainReputation(context.Background(), "example.com")
	
	assert.Equal(t, DomainReputationGood, result.status)
	assert.Empty(t, result.blacklists)
	mockVerifier.mockExchanger.AssertExpectations(t)
}

func TestReputationVerifier_ExtractDomainFromEmail(t *testing.T) {
	tests := []struct {
		email    string
		expected string
	}{
		{"test@example.com", "example.com"},
		{"user@subdomain.example.com", "subdomain.example.com"},
		{"Name <user@example.com>", "example.com"},
		{"invalid-email", ""},
		{"", ""},
	}
	
	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			result := extractDomainFromEmail(tt.email)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReputationVerifier_ScoreNormalization(t *testing.T) {
	verifier := NewReputationVerifier("8.8.8.8:53")
	
	// Test with extreme values to ensure score normalization
	emailCtx := EmailContext{
		From:     "test@example.com",
		SenderIP: net.ParseIP("192.168.1.1"), // Private IP, will add 0.1
	}
	
	result := verifier.Verify(context.Background(), emailCtx)
	
	// Score should be normalized to 0-1 range
	assert.GreaterOrEqual(t, result.Score, 0.0)
	assert.LessOrEqual(t, result.Score, 1.0)
}

func TestIPReputationStatus_String(t *testing.T) {
	tests := []struct {
		status   IPReputationStatus
		expected string
	}{
		{IPReputationUnknown, "unknown"},
		{IPReputationGood, "good"},
		{IPReputationSuspicious, "suspicious"},
		{IPReputationBad, "bad"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestDomainReputationStatus_String(t *testing.T) {
	tests := []struct {
		status   DomainReputationStatus
		expected string
	}{
		{DomainReputationUnknown, "unknown"},
		{DomainReputationGood, "good"},
		{DomainReputationSuspicious, "suspicious"},
		{DomainReputationBad, "bad"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestReputationVerifier_CheckDomainAge_MXRecords(t *testing.T) {
	mockVerifier := NewMockReputationVerifier()
	
	// Mock MX record response
	mxResp := &dns.Msg{
		MsgHdr: dns.MsgHdr{Rcode: dns.RcodeSuccess},
		Answer: []dns.RR{
			&dns.MX{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeMX}, Mx: "mail.example.com.", Preference: 10},
		},
	}
	
	// Mock A record response (single A record)
	aResp := &dns.Msg{
		MsgHdr: dns.MsgHdr{Rcode: dns.RcodeSuccess},
		Answer: []dns.RR{
			&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA}, A: net.ParseIP("1.2.3.4")},
		},
	}
	
	mockVerifier.mockExchanger.On("ExchangeContext", mock.Anything, mock.MatchedBy(func(msg *dns.Msg) bool {
		return msg.Question[0].Qtype == dns.TypeMX
	}), mock.Anything).Return(mxResp, time.Duration(0), nil)
	
	mockVerifier.mockExchanger.On("ExchangeContext", mock.Anything, mock.MatchedBy(func(msg *dns.Msg) bool {
		return msg.Question[0].Qtype == dns.TypeA
	}), mock.Anything).Return(aResp, time.Duration(0), nil)
	
	score := mockVerifier.checkDomainAge(context.Background(), "example.com")
	
	// Should get positive score for having MX records
	assert.Greater(t, score, 0.0)
	mockVerifier.mockExchanger.AssertExpectations(t)
}

func TestReputationVerifier_CheckDomainAge_SuspiciousTLD(t *testing.T) {
	mockVerifier := NewMockReputationVerifier()
	
	// Mock no MX records
	noMXResp := &dns.Msg{
		MsgHdr: dns.MsgHdr{Rcode: dns.RcodeNameError},
		Answer: []dns.RR{},
	}
	
	// Mock single A record
	aResp := &dns.Msg{
		MsgHdr: dns.MsgHdr{Rcode: dns.RcodeSuccess},
		Answer: []dns.RR{
			&dns.A{Hdr: dns.RR_Header{Name: "suspicious.tk.", Rrtype: dns.TypeA}, A: net.ParseIP("1.2.3.4")},
		},
	}
	
	mockVerifier.mockExchanger.On("ExchangeContext", mock.Anything, mock.Anything, mock.Anything).
		Return(func(ctx context.Context, msg *dns.Msg, addr string) *dns.Msg {
			if msg.Question[0].Qtype == dns.TypeMX {
				return noMXResp
			}
			return aResp
		}, time.Duration(0), nil)
	
	score := mockVerifier.checkDomainAge(context.Background(), "suspicious.tk")
	
	// Should get negative score for suspicious TLD
	assert.Less(t, score, 0.0)
	mockVerifier.mockExchanger.AssertExpectations(t)
}

func TestReputationVerifier_HasMultipleARecords(t *testing.T) {
	mockVerifier := NewMockReputationVerifier()
	
	// Mock multiple A records response
	multipleAResp := &dns.Msg{
		MsgHdr: dns.MsgHdr{Rcode: dns.RcodeSuccess},
		Answer: []dns.RR{
			&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA}, A: net.ParseIP("1.2.3.4")},
			&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA}, A: net.ParseIP("5.6.7.8")},
		},
	}
	
	mockVerifier.mockExchanger.On("ExchangeContext", mock.Anything, mock.Anything, mock.Anything).
		Return(multipleAResp, time.Duration(0), nil)
	
	result := mockVerifier.hasMultipleARecords(context.Background(), "example.com")
	
	assert.True(t, result)
	mockVerifier.mockExchanger.AssertExpectations(t)
}

func TestReputationVerifier_Error_Handling(t *testing.T) {
	verifier := NewReputationVerifier("8.8.8.8:53")
	
	emailCtx := EmailContext{
		From:     "invalid-email-format",
		SenderIP: net.ParseIP("1.2.3.4"),
	}
	
	// Should handle invalid email gracefully
	result := verifier.Verify(context.Background(), emailCtx)
	
	// Should handle gracefully without crashing, but might not have specific error
	assert.Equal(t, DomainReputationUnknown, result.DomainReputation)
}

func BenchmarkReputationVerify(b *testing.B) {
	mockVerifier := NewMockReputationVerifier()
	
	// Mock all responses to be negative (clean)
	negativeResp := &dns.Msg{
		MsgHdr: dns.MsgHdr{Rcode: dns.RcodeNameError},
		Answer: []dns.RR{},
	}
	
	mockVerifier.mockExchanger.On("ExchangeContext", mock.Anything, mock.Anything, mock.Anything).
		Return(negativeResp, time.Duration(0), nil)
	
	emailCtx := EmailContext{
		From:     "test@example.com",
		SenderIP: net.ParseIP("8.8.8.8"),
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mockVerifier.Verify(ctx, emailCtx)
	}
}
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

// MockDNSExchanger implements a mock DNS client for testing
type MockDNSExchanger struct {
	mock.Mock
}

func (m *MockDNSExchanger) ExchangeContext(ctx context.Context, msg *dns.Msg, addr string) (*dns.Msg, time.Duration, error) {
	args := m.Called(ctx, msg, addr)
	return args.Get(0).(*dns.Msg), args.Get(1).(time.Duration), args.Error(2)
}

// newSPFVerifierWithMock returns an SPFVerifier wired to a fresh mock DNS
// exchanger. Tests configure expectations via the returned mock and call
// verifier methods normally.
func newSPFVerifierWithMock() (*SPFVerifier, *MockDNSExchanger) {
	mockExchanger := &MockDNSExchanger{}
	verifier := NewSPFVerifier("8.8.8.8:53")
	verifier.client = mockExchanger
	return verifier, mockExchanger
}

func TestSPFVerifier_Verify_NoSenderIP(t *testing.T) {
	verifier := NewSPFVerifier("8.8.8.8:53")

	emailCtx := EmailContext{
		From:     "test@example.com",
		SenderIP: nil,
	}

	result := verifier.Verify(context.Background(), emailCtx)

	assert.Equal(t, SPFNone, result.Result)
	assert.Contains(t, result.Error, "no sender IP provided")
}

func TestSPFVerifier_Verify_InvalidEmail(t *testing.T) {
	verifier := NewSPFVerifier("8.8.8.8:53")

	emailCtx := EmailContext{
		From:     "invalid-email-format",
		SenderIP: net.ParseIP("192.168.1.1"),
	}

	result := verifier.Verify(context.Background(), emailCtx)

	assert.Equal(t, SPFPermError, result.Result)
	assert.Contains(t, result.Error, "invalid sender email format")
}

func TestSPFVerifier_Verify_NoSPFRecord(t *testing.T) {
	verifier, mockExchanger := newSPFVerifierWithMock()

	// Mock DNS response with no SPF record
	resp := &dns.Msg{
		MsgHdr: dns.MsgHdr{Rcode: dns.RcodeSuccess},
		Answer: []dns.RR{},
	}

	mockExchanger.On("ExchangeContext", mock.Anything, mock.Anything, mock.Anything).
		Return(resp, time.Duration(0), nil)

	emailCtx := EmailContext{
		From:     "test@example.com",
		SenderIP: net.ParseIP("192.168.1.1"),
	}

	result := verifier.Verify(context.Background(), emailCtx)

	assert.Equal(t, SPFNone, result.Result)
	mockExchanger.AssertExpectations(t)
}

func TestSPFVerifier_Verify_SPFPass(t *testing.T) {
	verifier, mockExchanger := newSPFVerifierWithMock()

	// Mock DNS response with SPF record that passes
	txtRecord := &dns.TXT{
		Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeTXT},
		Txt: []string{"v=spf1 ip4:192.168.1.0/24 ~all"},
	}
	resp := &dns.Msg{
		MsgHdr: dns.MsgHdr{Rcode: dns.RcodeSuccess},
		Answer: []dns.RR{txtRecord},
	}

	mockExchanger.On("ExchangeContext", mock.Anything, mock.Anything, mock.Anything).
		Return(resp, time.Duration(0), nil)

	emailCtx := EmailContext{
		From:     "test@example.com",
		SenderIP: net.ParseIP("192.168.1.100"),
	}

	result := verifier.Verify(context.Background(), emailCtx)

	assert.Equal(t, SPFPass, result.Result)
	assert.Equal(t, "ip4:192.168.1.0/24", result.Mechanism)
	mockExchanger.AssertExpectations(t)
}

func TestSPFVerifier_Verify_SPFFail(t *testing.T) {
	verifier, mockExchanger := newSPFVerifierWithMock()

	// Mock DNS response with SPF record that fails
	txtRecord := &dns.TXT{
		Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeTXT},
		Txt: []string{"v=spf1 ip4:10.0.0.0/24 -all"},
	}
	resp := &dns.Msg{
		MsgHdr: dns.MsgHdr{Rcode: dns.RcodeSuccess},
		Answer: []dns.RR{txtRecord},
	}

	mockExchanger.On("ExchangeContext", mock.Anything, mock.Anything, mock.Anything).
		Return(resp, time.Duration(0), nil)

	emailCtx := EmailContext{
		From:     "test@example.com",
		SenderIP: net.ParseIP("192.168.1.100"),
	}

	result := verifier.Verify(context.Background(), emailCtx)

	assert.Equal(t, SPFFail, result.Result)
	assert.Equal(t, "all:", result.Mechanism)
	mockExchanger.AssertExpectations(t)
}

func TestSPFVerifier_Verify_SPFSoftFail(t *testing.T) {
	verifier, mockExchanger := newSPFVerifierWithMock()

	// Mock DNS response with SPF record that soft fails
	txtRecord := &dns.TXT{
		Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeTXT},
		Txt: []string{"v=spf1 ip4:10.0.0.0/24 ~all"},
	}
	resp := &dns.Msg{
		MsgHdr: dns.MsgHdr{Rcode: dns.RcodeSuccess},
		Answer: []dns.RR{txtRecord},
	}

	mockExchanger.On("ExchangeContext", mock.Anything, mock.Anything, mock.Anything).
		Return(resp, time.Duration(0), nil)

	emailCtx := EmailContext{
		From:     "test@example.com",
		SenderIP: net.ParseIP("192.168.1.100"),
	}

	result := verifier.Verify(context.Background(), emailCtx)

	assert.Equal(t, SPFSoftFail, result.Result)
	assert.Equal(t, "all:", result.Mechanism)
	mockExchanger.AssertExpectations(t)
}

func TestSPFVerifier_ParseMechanism(t *testing.T) {
	verifier := NewSPFVerifier("8.8.8.8:53")

	tests := []struct {
		input     string
		qualifier string
		mechanism string
		value     string
	}{
		{"all", "+", "all", ""},
		{"+all", "+", "all", ""},
		{"-all", "-", "all", ""},
		{"~all", "~", "all", ""},
		{"?all", "?", "all", ""},
		{"ip4:192.168.1.0/24", "+", "ip4", "192.168.1.0/24"},
		{"-ip4:10.0.0.0/8", "-", "ip4", "10.0.0.0/8"},
		{"include:_spf.google.com", "+", "include", "_spf.google.com"},
		{"a:mail.example.com", "+", "a", "mail.example.com"},
		{"mx:example.com/24", "+", "mx", "example.com/24"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			mech := verifier.parseMechanism(tt.input)
			assert.Equal(t, tt.qualifier, mech.qualifier)
			assert.Equal(t, tt.mechanism, mech.mechanism)
			assert.Equal(t, tt.value, mech.value)
		})
	}
}

func TestSPFVerifier_EvaluateIP4(t *testing.T) {
	verifier := NewSPFVerifier("8.8.8.8:53")

	tests := []struct {
		cidr        string
		senderIP    string
		shouldMatch bool
	}{
		{"192.168.1.0/24", "192.168.1.100", true},
		{"192.168.1.0/24", "192.168.2.100", false},
		{"10.0.0.1", "10.0.0.1", true},
		{"10.0.0.1", "10.0.0.2", false},
		{"0.0.0.0/0", "8.8.8.8", true},
	}

	for _, tt := range tests {
		t.Run(tt.cidr+"_vs_"+tt.senderIP, func(t *testing.T) {
			matches, err := verifier.evaluateIP4(tt.cidr, net.ParseIP(tt.senderIP))
			assert.NoError(t, err)
			assert.Equal(t, tt.shouldMatch, matches)
		})
	}
}

func TestSPFVerifier_EvaluateIP6(t *testing.T) {
	verifier := NewSPFVerifier("8.8.8.8:53")

	tests := []struct {
		cidr        string
		senderIP    string
		shouldMatch bool
	}{
		{"2001:db8::/32", "2001:db8::1", true},
		{"2001:db8::/32", "2001:db9::1", false},
		{"::1", "::1", true},
		{"::1", "::2", false},
	}

	for _, tt := range tests {
		t.Run(tt.cidr+"_vs_"+tt.senderIP, func(t *testing.T) {
			matches, err := verifier.evaluateIP6(tt.cidr, net.ParseIP(tt.senderIP))
			assert.NoError(t, err)
			assert.Equal(t, tt.shouldMatch, matches)
		})
	}
}

func TestSPFVerifier_QualifierToResult(t *testing.T) {
	verifier := NewSPFVerifier("8.8.8.8:53")

	tests := []struct {
		qualifier string
		expected  SPFStatus
	}{
		{"+", SPFPass},
		{"-", SPFFail},
		{"~", SPFSoftFail},
		{"?", SPFNeutral},
		{"", SPFNeutral}, // default
	}

	for _, tt := range tests {
		t.Run(tt.qualifier, func(t *testing.T) {
			result := verifier.qualifierToResult(tt.qualifier)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractDomainFromEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"test@example.com", "example.com"},
		{"user.name@sub.domain.com", "sub.domain.com"},
		{"Name Lastname <user@example.com>", "example.com"},
		{"\"Quoted Name\" <user@example.com>", "example.com"},
		{"user@example.com (Comment)", "example.com"},
		{"<user@example.com>", "example.com"},
		{"invalid-email", ""},
		// Production extracts the domain from a missing-local-part address
		// because SPF/DKIM/DMARC verification is best-effort against any
		// recognisable domain. Reject this only at the SMTP envelope layer.
		{"@example.com", "example.com"},
		{"user@", ""},
		{"", ""},
		{"user@EXAMPLE.COM", "example.com"},  // Should be lowercase
		{"user@example.com.", "example.com"}, // Trailing dot removed
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractDomainFromEmail(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSPFVerifier_ParseSPFRecord(t *testing.T) {
	verifier := NewSPFVerifier("8.8.8.8:53")

	record := "v=spf1 include:_spf.google.com ip4:192.168.1.0/24 mx a ~all"
	mechanisms := verifier.parseSPFRecord(record)

	assert.Len(t, mechanisms, 5)

	// Check each mechanism
	assert.Equal(t, "include", mechanisms[0].mechanism)
	assert.Equal(t, "_spf.google.com", mechanisms[0].value)

	assert.Equal(t, "ip4", mechanisms[1].mechanism)
	assert.Equal(t, "192.168.1.0/24", mechanisms[1].value)

	assert.Equal(t, "mx", mechanisms[2].mechanism)
	assert.Equal(t, "", mechanisms[2].value)

	assert.Equal(t, "a", mechanisms[3].mechanism)
	assert.Equal(t, "", mechanisms[3].value)

	assert.Equal(t, "all", mechanisms[4].mechanism)
	assert.Equal(t, "~", mechanisms[4].qualifier)
}

func TestSPFVerifier_ExpandMacros(t *testing.T) {
	verifier := NewSPFVerifier("8.8.8.8:53")

	tests := []struct {
		domain       string
		senderDomain string
		sender       string
		expected     string
	}{
		{"example.%{d}", "test.com", "user@test.com", "example.test.com"},
		{"%{s}.exists", "test.com", "user@test.com", "user@test.com.exists"},
		{"fixed.domain", "test.com", "user@test.com", "fixed.domain"},
		{"%{d}.%{s}", "test.com", "user@test.com", "test.com.user@test.com"},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			result := verifier.expandMacros(tt.domain, tt.senderDomain, tt.sender)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func BenchmarkSPFVerify(b *testing.B) {
	verifier, mockExchanger := newSPFVerifierWithMock()

	// Mock DNS response
	txtRecord := &dns.TXT{
		Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeTXT},
		Txt: []string{"v=spf1 include:_spf.google.com ip4:192.168.1.0/24 mx a ~all"},
	}
	resp := &dns.Msg{
		MsgHdr: dns.MsgHdr{Rcode: dns.RcodeSuccess},
		Answer: []dns.RR{txtRecord},
	}

	mockExchanger.On("ExchangeContext", mock.Anything, mock.Anything, mock.Anything).
		Return(resp, time.Duration(0), nil)

	emailCtx := EmailContext{
		From:     "test@example.com",
		SenderIP: net.ParseIP("192.168.1.100"),
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		verifier.Verify(ctx, emailCtx)
	}
}

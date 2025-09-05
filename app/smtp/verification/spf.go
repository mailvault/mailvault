package verification

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/mail"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// SPFVerifier handles SPF record verification
type SPFVerifier struct {
	client   *dns.Client
	resolver string
	timeout  time.Duration
}

// NewSPFVerifier creates a new SPF verifier
func NewSPFVerifier(resolver string) *SPFVerifier {
	if resolver == "" {
		resolver = "8.8.8.8:53"
	}

	return &SPFVerifier{
		client: &dns.Client{
			Timeout: 5 * time.Second,
		},
		resolver: resolver,
		timeout:  5 * time.Second,
	}
}

// Verify performs SPF verification for the given context
func (v *SPFVerifier) Verify(ctx context.Context, emailCtx EmailContext) SPFResult {
	if emailCtx.SenderIP == nil {
		return SPFResult{
			Result: SPFNone,
			Error:  "no sender IP provided",
		}
	}

	// Extract sender domain from email address
	senderDomain := extractDomainFromEmail(emailCtx.From)
	if senderDomain == "" {
		return SPFResult{
			Result: SPFPermError,
			Error:  "invalid sender email format",
		}
	}
	slog.Info("SPFVerifier", "senderDomain", senderDomain)

	// Get SPF record for the domain
	spfRecord, err := v.getSPFRecord(ctx, senderDomain)
	slog.Info("SPFVerifier", "spfRecord", spfRecord)
	if err != nil {
		slog.Error("SPFVerifier", "error", err)
		return SPFResult{
			Result: SPFTempError,
			Error:  fmt.Sprintf("failed to get SPF record: %v", err),
		}
	}

	if spfRecord == "" {
		return SPFResult{
			Result: SPFNone,
		}
	}

	// Parse and evaluate SPF record
	return v.evaluateSPF(ctx, spfRecord, emailCtx.SenderIP, senderDomain, emailCtx.From, 0)
}

// getSPFRecord retrieves the SPF record for a domain
func (v *SPFVerifier) getSPFRecord(ctx context.Context, domain string) (string, error) {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeTXT)

	resp, _, err := v.client.ExchangeContext(ctx, msg, v.resolver)
	if err != nil {
		return "", fmt.Errorf("DNS query failed: %w", err)
	}

	if resp.Rcode != dns.RcodeSuccess {
		return "", fmt.Errorf("DNS query returned code: %d", resp.Rcode)
	}

	// Look for SPF record in TXT records
	for _, rr := range resp.Answer {
		slog.Info("SPFVerifier", "rr", rr)
		if txt, ok := rr.(*dns.TXT); ok {
			slog.Info("SPFVerifier", "txt", txt.Txt)

			record := strings.Join(txt.Txt, "")
			if strings.HasPrefix(record, "v=spf1") {
				return record, nil
			}
		}
	}

	return "", nil
}

// evaluateSPF evaluates an SPF record against the sender IP
func (v *SPFVerifier) evaluateSPF(ctx context.Context, record string, senderIP net.IP, domain, sender string, depth int) SPFResult {
	// Prevent infinite recursion
	if depth > 10 {
		return SPFResult{
			Result: SPFPermError,
			Error:  "SPF evaluation depth exceeded",
		}
	}

	// Parse SPF record into mechanisms
	mechanisms := v.parseSPFRecord(record)

	for _, mechanism := range mechanisms {
		result := v.evaluateMechanism(ctx, mechanism, senderIP, domain, sender, depth)

		// If we get a definitive result (not neutral), return it
		if result.Result != SPFNeutral {
			return result
		}
	}

	// If no mechanisms matched, default is neutral
	return SPFResult{
		Result: SPFNeutral,
	}
}

// spfMechanism represents a parsed SPF mechanism
type spfMechanism struct {
	qualifier string // +, -, ~, ?
	mechanism string // all, include, a, mx, ip4, ip6, exists
	value     string // optional value/domain
}

// parseSPFRecord parses an SPF record into mechanisms
func (v *SPFVerifier) parseSPFRecord(record string) []spfMechanism {
	var mechanisms []spfMechanism

	parts := strings.Fields(record)
	for i := 1; i < len(parts); i++ { // Skip "v=spf1"
		part := parts[i]

		// Skip modifiers for now (redirect=, exp=)
		if strings.Contains(part, "=") && !strings.HasPrefix(part, "include:") &&
			!strings.HasPrefix(part, "ip4:") && !strings.HasPrefix(part, "ip6:") &&
			!strings.HasPrefix(part, "exists:") {
			continue
		}

		mechanism := v.parseMechanism(part)
		mechanisms = append(mechanisms, mechanism)
	}

	return mechanisms
}

// parseMechanism parses a single SPF mechanism
func (v *SPFVerifier) parseMechanism(part string) spfMechanism {
	qualifier := "+"
	mechanism := part

	// Check for qualifier prefix
	if len(part) > 0 {
		first := part[0]
		if first == '+' || first == '-' || first == '~' || first == '?' {
			qualifier = string(first)
			mechanism = part[1:]
		}
	}

	// Split mechanism and value
	var value string
	if colonIdx := strings.Index(mechanism, ":"); colonIdx != -1 {
		value = mechanism[colonIdx+1:]
		mechanism = mechanism[:colonIdx]
	}

	return spfMechanism{
		qualifier: qualifier,
		mechanism: mechanism,
		value:     value,
	}
}

// evaluateMechanism evaluates a single SPF mechanism
func (v *SPFVerifier) evaluateMechanism(ctx context.Context, mech spfMechanism, senderIP net.IP, domain, sender string, depth int) SPFResult {
	var matches bool
	var err error

	switch mech.mechanism {
	case "all":
		matches = true

	case "include":
		if mech.value == "" {
			return SPFResult{Result: SPFPermError, Error: "include mechanism missing domain"}
		}
		return v.evaluateInclude(ctx, mech.value, senderIP, domain, sender, depth)

	case "a":
		matches, err = v.evaluateA(ctx, mech.value, domain, senderIP)

	case "mx":
		matches, err = v.evaluateMX(ctx, mech.value, domain, senderIP)

	case "ip4":
		matches, err = v.evaluateIP4(mech.value, senderIP)

	case "ip6":
		matches, err = v.evaluateIP6(mech.value, senderIP)

	case "exists":
		matches, err = v.evaluateExists(ctx, mech.value, domain, sender)

	default:
		// Unknown mechanism
		slog.Error("SPFVerifier", "unknown mechanism", mech.mechanism)
		return SPFResult{Result: SPFNeutral}
	}

	if err != nil {
		return SPFResult{
			Result: SPFTempError,
			Error:  fmt.Sprintf("mechanism %s evaluation failed: %v", mech.mechanism, err),
		}
	}

	if matches {
		return SPFResult{
			Result:    v.qualifierToResult(mech.qualifier),
			Mechanism: mech.mechanism + ":" + mech.value,
		}
	}

	return SPFResult{Result: SPFNeutral}
}

// evaluateInclude handles include mechanisms
func (v *SPFVerifier) evaluateInclude(ctx context.Context, includeDomain string, senderIP net.IP, domain, sender string, depth int) SPFResult {
	spfRecord, err := v.getSPFRecord(ctx, includeDomain)
	if err != nil {
		return SPFResult{Result: SPFTempError, Error: fmt.Sprintf("include lookup failed: %v", err)}
	}

	if spfRecord == "" {
		return SPFResult{Result: SPFPermError, Error: "included domain has no SPF record"}
	}

	result := v.evaluateSPF(ctx, spfRecord, senderIP, includeDomain, sender, depth+1)

	// Convert include results according to SPF spec
	switch result.Result {
	case SPFPass:
		return SPFResult{Result: SPFPass, Mechanism: "include:" + includeDomain}
	case SPFFail, SPFSoftFail, SPFNeutral:
		return SPFResult{Result: SPFNeutral}
	default:
		return result
	}
}

// evaluateA handles A record mechanisms
func (v *SPFVerifier) evaluateA(ctx context.Context, domain, defaultDomain string, senderIP net.IP) (bool, error) {
	if domain == "" {
		domain = defaultDomain
	}

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeA)

	resp, _, err := v.client.ExchangeContext(ctx, msg, v.resolver)
	if err != nil {
		return false, err
	}

	for _, rr := range resp.Answer {
		if a, ok := rr.(*dns.A); ok {
			if a.A.Equal(senderIP) {
				return true, nil
			}
		}
	}

	return false, nil
}

// evaluateMX handles MX record mechanisms
func (v *SPFVerifier) evaluateMX(ctx context.Context, domain, defaultDomain string, senderIP net.IP) (bool, error) {
	if domain == "" {
		domain = defaultDomain
	}

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeMX)

	resp, _, err := v.client.ExchangeContext(ctx, msg, v.resolver)
	if err != nil {
		return false, err
	}

	// Get A records for each MX record
	for _, rr := range resp.Answer {
		if mx, ok := rr.(*dns.MX); ok {
			matches, err := v.evaluateA(ctx, mx.Mx, "", senderIP)
			if err != nil {
				continue
			}
			if matches {
				return true, nil
			}
		}
	}

	return false, nil
}

// evaluateIP4 handles IPv4 CIDR mechanisms
func (v *SPFVerifier) evaluateIP4(cidr string, senderIP net.IP) (bool, error) {
	if !strings.Contains(cidr, "/") {
		cidr += "/32"
	}

	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return false, err
	}

	return network.Contains(senderIP), nil
}

// evaluateIP6 handles IPv6 CIDR mechanisms
func (v *SPFVerifier) evaluateIP6(cidr string, senderIP net.IP) (bool, error) {
	if !strings.Contains(cidr, "/") {
		cidr += "/128"
	}

	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return false, err
	}

	return network.Contains(senderIP), nil
}

// evaluateExists handles exists mechanisms
func (v *SPFVerifier) evaluateExists(ctx context.Context, domain, senderDomain, sender string) (bool, error) {
	// Expand macros in domain (simplified implementation)
	expandedDomain := v.expandMacros(domain, senderDomain, sender)

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(expandedDomain), dns.TypeA)

	resp, _, err := v.client.ExchangeContext(ctx, msg, v.resolver)
	if err != nil {
		return false, err
	}

	return len(resp.Answer) > 0, nil
}

// expandMacros performs basic SPF macro expansion
func (v *SPFVerifier) expandMacros(domain, senderDomain, sender string) string {
	// Basic macro expansion - in production, this should be more comprehensive
	domain = strings.ReplaceAll(domain, "%{d}", senderDomain)
	domain = strings.ReplaceAll(domain, "%{s}", sender)
	return domain
}

// qualifierToResult converts SPF qualifier to result
func (v *SPFVerifier) qualifierToResult(qualifier string) SPFStatus {
	switch qualifier {
	case "+":
		return SPFPass
	case "-":
		return SPFFail
	case "~":
		return SPFSoftFail
	case "?":
		return SPFNeutral
	default:
		return SPFNeutral
	}
}

// extractDomainFromEmail extracts the domain part from a possibly decorated email string.
// It supports inputs like "Name Lastname <mail@example.com>" and returns "example.com".
func extractDomainFromEmail(input string) string {
	s := strings.TrimSpace(input)
	if s == "" {
		return ""
	}

	// Prefer robust parsing using net/mail
	if addr, err := mail.ParseAddress(s); err == nil && addr != nil {
		s = addr.Address
	} else {
		// Fallback: extract address within angle brackets if present
		if lt := strings.LastIndex(s, "<"); lt != -1 {
			if gtRel := strings.Index(s[lt:], ">"); gtRel != -1 {
				s = s[lt+1 : lt+gtRel]
			}
		}
		s = strings.TrimSpace(strings.Trim(s, "<>"))
	}

	at := strings.LastIndex(s, "@")
	if at == -1 || at == len(s)-1 {
		return ""
	}

	domain := s[at+1:]
	// Remove any trailing punctuation from raw header contexts
	domain = strings.TrimSpace(domain)
	domain = strings.TrimRight(domain, ">.,;:\"'")
	if domain == "" {
		return ""
	}
	return strings.ToLower(domain)
}

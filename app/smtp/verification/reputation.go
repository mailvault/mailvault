package verification

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// ReputationVerifier handles IP and domain reputation checks
type ReputationVerifier struct {
	client     DNSExchanger
	httpClient *http.Client
	resolver   string
	timeout    time.Duration
	blacklists []string
}

// NewReputationVerifier creates a new reputation verifier
func NewReputationVerifier(resolver string) *ReputationVerifier {
	if resolver == "" {
		resolver = "8.8.8.8:53"
	}
	
	return &ReputationVerifier{
		client: &dns.Client{
			Timeout: 5 * time.Second,
		},
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		resolver: resolver,
		timeout:  5 * time.Second,
		blacklists: []string{
			"zen.spamhaus.org",      // Spamhaus ZEN (combines SBL, CSS, PBL)
			"bl.spamcop.net",        // SpamCop
			"dnsbl.sorbs.net",       // SORBS
			"b.barracudacentral.org", // Barracuda
			"cbl.abuseat.org",       // Composite Blocking List
			"psbl.surriel.com",      // Passive Spam Block List
			"ubl.unsubscore.com",    // Lashback UBL
		},
	}
}

// Verify performs reputation checks on IP and domain
func (v *ReputationVerifier) Verify(ctx context.Context, emailCtx EmailContext) ReputationResult {
	result := ReputationResult{
		IPReputation:     IPReputationUnknown,
		DomainReputation: DomainReputationUnknown,
		Score:            0.5, // Neutral score
	}

	// Check IP reputation if sender IP is available
	if emailCtx.SenderIP != nil {
		ipResult := v.checkIPReputation(ctx, emailCtx.SenderIP)
		result.IPReputation = ipResult.status
		result.Blacklisted = append(result.Blacklisted, ipResult.blacklists...)
		result.Score += ipResult.scoreAdjustment
	}

	// Check domain reputation
	senderDomain := extractDomainFromEmail(emailCtx.From)
	if senderDomain != "" {
		domainResult := v.checkDomainReputation(ctx, senderDomain)
		result.DomainReputation = domainResult.status
		result.Blacklisted = append(result.Blacklisted, domainResult.blacklists...)
		result.Score += domainResult.scoreAdjustment
	}

	// Normalize score to 0-1 range
	if result.Score < 0 {
		result.Score = 0
	} else if result.Score > 1 {
		result.Score = 1
	}

	return result
}

// ipReputationResult holds IP reputation check results
type ipReputationResult struct {
	status          IPReputationStatus
	blacklists      []string
	scoreAdjustment float64
}

// domainReputationResult holds domain reputation check results
type domainReputationResult struct {
	status          DomainReputationStatus
	blacklists      []string
	scoreAdjustment float64
}

// checkIPReputation checks IP against various blacklists
func (v *ReputationVerifier) checkIPReputation(ctx context.Context, ip net.IP) ipReputationResult {
	result := ipReputationResult{
		status:          IPReputationGood,
		scoreAdjustment: 0,
	}

	// Check private/local IPs
	if v.isPrivateIP(ip) {
		result.status = IPReputationGood
		result.scoreAdjustment = 0.1 // Slightly positive for internal IPs
		return result
	}

	blacklistCount := 0
	totalChecked := 0

	// Check each blacklist
	for _, blacklist := range v.blacklists {
		totalChecked++
		if v.checkIPBlacklist(ctx, ip, blacklist) {
			result.blacklists = append(result.blacklists, blacklist)
			blacklistCount++
		}
	}

	// Determine reputation based on blacklist results
	if blacklistCount == 0 {
		result.status = IPReputationGood
		result.scoreAdjustment = 0.1
	} else if blacklistCount <= 2 {
		result.status = IPReputationSuspicious
		result.scoreAdjustment = -0.2
	} else {
		result.status = IPReputationBad
		result.scoreAdjustment = -0.5
	}

	return result
}

// checkDomainReputation checks domain against reputation sources
func (v *ReputationVerifier) checkDomainReputation(ctx context.Context, domain string) domainReputationResult {
	result := domainReputationResult{
		status:          DomainReputationGood,
		scoreAdjustment: 0,
	}

	blacklistCount := 0
	
	// Check domain blacklists
	domainBlacklists := []string{
		"dbl.spamhaus.org",    // Spamhaus Domain Block List
		"surbl.org",           // SURBL
		"uribl.com",           // URIBL
		"multi.surbl.org",     // Multi SURBL
	}

	for _, blacklist := range domainBlacklists {
		if v.checkDomainBlacklist(ctx, domain, blacklist) {
			result.blacklists = append(result.blacklists, blacklist)
			blacklistCount++
		}
	}

	// Check domain age and other heuristics
	ageScore := v.checkDomainAge(ctx, domain)
	result.scoreAdjustment += ageScore

	// Determine reputation based on checks
	if blacklistCount == 0 {
		if ageScore >= 0 {
			result.status = DomainReputationGood
			result.scoreAdjustment += 0.1
		} else {
			result.status = DomainReputationSuspicious
		}
	} else if blacklistCount <= 1 {
		result.status = DomainReputationSuspicious
		result.scoreAdjustment -= 0.2
	} else {
		result.status = DomainReputationBad
		result.scoreAdjustment -= 0.5
	}

	return result
}

// checkIPBlacklist checks if an IP is listed in a DNS blacklist
func (v *ReputationVerifier) checkIPBlacklist(ctx context.Context, ip net.IP, blacklist string) bool {
	// Reverse IP for DNS lookup
	reversedIP := v.reverseIP(ip)
	if reversedIP == "" {
		return false
	}

	// Query DNS
	query := fmt.Sprintf("%s.%s", reversedIP, blacklist)
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(query), dns.TypeA)

	resp, _, err := v.client.ExchangeContext(ctx, msg, v.resolver)
	if err != nil {
		return false
	}

	// If we get a response, the IP is likely blacklisted
	return resp.Rcode == dns.RcodeSuccess && len(resp.Answer) > 0
}

// checkDomainBlacklist checks if a domain is listed in a DNS blacklist
func (v *ReputationVerifier) checkDomainBlacklist(ctx context.Context, domain, blacklist string) bool {
	query := fmt.Sprintf("%s.%s", domain, blacklist)
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(query), dns.TypeA)

	resp, _, err := v.client.ExchangeContext(ctx, msg, v.resolver)
	if err != nil {
		return false
	}

	return resp.Rcode == dns.RcodeSuccess && len(resp.Answer) > 0
}

// checkDomainAge estimates domain age and reputation based on creation time
func (v *ReputationVerifier) checkDomainAge(ctx context.Context, domain string) float64 {
	// This is a simplified implementation
	// In production, you might query WHOIS data or use domain intelligence APIs
	
	// For now, we'll do some basic heuristics:
	// - Check if domain has MX records (established email setup)
	// - Check if domain has multiple A records (load balancing/CDN)
	
	score := 0.0
	
	// Check MX records
	if v.hasMXRecords(ctx, domain) {
		score += 0.1
	}
	
	// Check multiple A records
	if v.hasMultipleARecords(ctx, domain) {
		score += 0.05
	}
	
	// Check for suspicious TLDs
	if v.hasSuspiciousTLD(domain) {
		score -= 0.2
	}
	
	return score
}

// hasMXRecords checks if domain has MX records
func (v *ReputationVerifier) hasMXRecords(ctx context.Context, domain string) bool {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeMX)

	resp, _, err := v.client.ExchangeContext(ctx, msg, v.resolver)
	if err != nil {
		return false
	}

	return resp.Rcode == dns.RcodeSuccess && len(resp.Answer) > 0
}

// hasMultipleARecords checks if domain has multiple A records
func (v *ReputationVerifier) hasMultipleARecords(ctx context.Context, domain string) bool {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeA)

	resp, _, err := v.client.ExchangeContext(ctx, msg, v.resolver)
	if err != nil {
		return false
	}

	return resp.Rcode == dns.RcodeSuccess && len(resp.Answer) > 1
}

// hasSuspiciousTLD checks if domain uses a suspicious top-level domain
func (v *ReputationVerifier) hasSuspiciousTLD(domain string) bool {
	suspiciousTLDs := []string{
		".tk", ".ml", ".ga", ".cf",  // Free TLDs often used for spam
		".bit", ".onion",            // Alternative DNS/dark web
	}
	
	domain = strings.ToLower(domain)
	for _, tld := range suspiciousTLDs {
		if strings.HasSuffix(domain, tld) {
			return true
		}
	}
	
	return false
}

// reverseIP reverses an IP address for DNS blacklist queries
func (v *ReputationVerifier) reverseIP(ip net.IP) string {
	if ip.To4() != nil {
		// IPv4
		octets := strings.Split(ip.String(), ".")
		if len(octets) != 4 {
			return ""
		}
		return fmt.Sprintf("%s.%s.%s.%s", octets[3], octets[2], octets[1], octets[0])
	} else {
		// IPv6 - more complex, simplified implementation
		return ""
	}
}

// isPrivateIP checks if an IP is in private/local ranges
func (v *ReputationVerifier) isPrivateIP(ip net.IP) bool {
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",    // Loopback
		"169.254.0.0/16", // Link-local
	}
	
	for _, rangeStr := range privateRanges {
		_, cidr, err := net.ParseCIDR(rangeStr)
		if err != nil {
			continue
		}
		if cidr.Contains(ip) {
			return true
		}
	}
	
	return false
}
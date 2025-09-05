package verification

import (
	"context"
	"fmt"
	"strings"
	"time"

	msdmarc "github.com/emersion/go-msgauth/dmarc"
	"github.com/miekg/dns"
)

// DMARCVerifier handles DMARC policy verification
type DMARCVerifier struct {
	client   *dns.Client
	resolver string
	timeout  time.Duration
}

// NewDMARCVerifier creates a new DMARC verifier
func NewDMARCVerifier(resolver string) *DMARCVerifier {
	if resolver == "" {
		resolver = "8.8.8.8:53"
	}

	return &DMARCVerifier{
		client: &dns.Client{
			Timeout: 5 * time.Second,
		},
		resolver: resolver,
		timeout:  5 * time.Second,
	}
}

// Verify performs DMARC verification using SPF and DKIM results
func (v *DMARCVerifier) Verify(ctx context.Context, emailCtx EmailContext, spfResult SPFResult, dkimResult DKIMResult) DMARCResult {
	// Extract domain from From header
	fromDomain := extractDomainFromEmail(emailCtx.From)
	if fromDomain == "" {
		return DMARCResult{
			Result: DMARCPermError,
			Error:  "invalid From header format",
		}
	}

	// Get DMARC policy for the domain via go-msgauth
	policy, err := v.getDMARCPolicy(ctx, fromDomain)
	if err != nil {
		if msdmarc.IsTempFail(err) {
			return DMARCResult{Result: DMARCTempError, Error: err.Error()}
		}
		if err == msdmarc.ErrNoPolicy {
			return DMARCResult{Result: DMARCNone}
		}
		return DMARCResult{Result: DMARCPermError, Error: fmt.Sprintf("failed to get DMARC policy: %v", err)}
	}

	if policy == nil {
		return DMARCResult{Result: DMARCNone}
	}

	// Check SPF alignment
	spfAligned := v.checkSPFAlignment(emailCtx, spfResult, policy, fromDomain)

	// Check DKIM alignment
	dkimAligned := v.checkDKIMAlignment(dkimResult, policy, fromDomain)

	// Determine DMARC result based on policy and alignment
	result := v.evaluateDMARCPolicy(policy, spfResult, dkimResult, spfAligned, dkimAligned)

	percentage := 100
	if policy.Percent != nil {
		percentage = *policy.Percent
	}
	return DMARCResult{
		Result:     result,
		Policy:     string(policy.Policy),
		Percentage: percentage,
		SPFAlign:   spfAligned,
		DKIMAlign:  dkimAligned,
	}
}

// getDMARCPolicy retrieves the DMARC policy for a domain using go-msgauth with our DNS client
func (v *DMARCVerifier) getDMARCPolicy(ctx context.Context, domain string) (*msdmarc.Record, error) {
	options := &msdmarc.LookupOptions{
		LookupTXT: func(name string) ([]string, error) {
			msg := new(dns.Msg)
			msg.SetQuestion(dns.Fqdn(name), dns.TypeTXT)
			resp, _, err := v.client.ExchangeContext(ctx, msg, v.resolver)
			if err != nil {
				return nil, err
			}
			if resp.Rcode != dns.RcodeSuccess {
				return nil, fmt.Errorf("DNS query returned code: %d", resp.Rcode)
			}
			var txts []string
			for _, rr := range resp.Answer {
				if txt, ok := rr.(*dns.TXT); ok {
					txts = append(txts, strings.Join(txt.Txt, ""))
				}
			}
			return txts, nil
		},
	}
	return msdmarc.LookupWithOptions(domain, options)
}

// Removed custom DMARC parser in favor of go-msgauth

// checkSPFAlignment checks if SPF passes with proper alignment
func (v *DMARCVerifier) checkSPFAlignment(emailCtx EmailContext, spfResult SPFResult, policy *msdmarc.Record, fromDomain string) bool {
	// SPF must pass first
	if spfResult.Result != SPFPass {
		return false
	}

	// Extract Return-Path domain for alignment check
	returnPathDomain := v.getReturnPathDomain(emailCtx)
	if returnPathDomain == "" {
		return false
	}

	// Check alignment based on policy
	if policy.SPFAlignment == msdmarc.AlignmentStrict {
		// Strict alignment: domains must exactly match
		return strings.ToLower(returnPathDomain) == strings.ToLower(fromDomain)
	} else {
		// Relaxed alignment: organizational domains must match
		return v.organizationalDomainsMatch(returnPathDomain, fromDomain)
	}
}

// checkDKIMAlignment checks if DKIM passes with proper alignment
func (v *DMARCVerifier) checkDKIMAlignment(dkimResult DKIMResult, policy *msdmarc.Record, fromDomain string) bool {
	// At least one DKIM signature must pass
	if !dkimResult.Valid {
		return false
	}

	// Check if any valid DKIM signature has proper alignment
	for _, sigResult := range dkimResult.Results {
		if sigResult.Status == DKIMPass {
			if policy.DKIMAlignment == msdmarc.AlignmentStrict {
				// Strict alignment: domains must exactly match
				if strings.ToLower(sigResult.Domain) == strings.ToLower(fromDomain) {
					return true
				}
			} else {
				// Relaxed alignment: organizational domains must match
				if v.organizationalDomainsMatch(sigResult.Domain, fromDomain) {
					return true
				}
			}
		}
	}

	return false
}

// getReturnPathDomain extracts domain from Return-Path or envelope sender
func (v *DMARCVerifier) getReturnPathDomain(emailCtx EmailContext) string {
	// In SMTP context, this would be the envelope sender (MAIL FROM)
	// For now, we'll extract from the From field as a fallback
	return extractDomainFromEmail(emailCtx.From)
}

// organizationalDomainsMatch checks if two domains share the same organizational domain
func (v *DMARCVerifier) organizationalDomainsMatch(domain1, domain2 string) bool {
	// Simplified implementation - in production, use a Public Suffix List
	org1 := v.getOrganizationalDomain(domain1)
	org2 := v.getOrganizationalDomain(domain2)

	return strings.ToLower(org1) == strings.ToLower(org2)
}

// getOrganizationalDomain extracts the organizational domain (simplified)
func (v *DMARCVerifier) getOrganizationalDomain(domain string) string {
	// Simplified implementation - just take the last two parts
	// In production, use a proper Public Suffix List implementation
	parts := strings.Split(domain, ".")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], ".")
	}
	return domain
}

// evaluateDMARCPolicy determines the final DMARC result based on policy and alignment
func (v *DMARCVerifier) evaluateDMARCPolicy(policy *msdmarc.Record, spfResult SPFResult, dkimResult DKIMResult, spfAligned, dkimAligned bool) DMARCStatus {
	// DMARC passes if either SPF or DKIM is aligned and passes
	dmarcPass := (spfAligned && spfResult.Result == SPFPass) ||
		(dkimAligned && dkimResult.Valid)

	if dmarcPass {
		return DMARCPass
	}

	// DMARC fails - the action depends on the policy
	// But for verification purposes, we just return fail
	return DMARCFail
}

// GetPolicyAction determines what action to take based on DMARC policy
func (v *DMARCVerifier) GetPolicyAction(policy *msdmarc.Record, dmarcResult DMARCStatus) Action {
	if dmarcResult == DMARCPass {
		return ActionAccept
	}

	if dmarcResult != DMARCFail {
		return ActionAccept // For none, temperror, permerror
	}

	// Apply percentage policy
	pct := 100
	if policy.Percent != nil {
		pct = *policy.Percent
	}
	if pct < 100 {
		// In production, implement proper random selection based on percentage
		// For now, we'll assume the percentage applies
	}

	switch policy.Policy {
	case msdmarc.PolicyReject:
		return ActionReject
	case msdmarc.PolicyQuarantine:
		return ActionQuarantine
	case msdmarc.PolicyNone:
		return ActionAccept
	default:
		return ActionAccept
	}
}

package verification

import (
	"context"
	"log/slog"
	"time"
)

// Verifier coordinates all spam verification checks
type Verifier struct {
	spfVerifier        *SPFVerifier
	dkimVerifier       *DKIMVerifier
	dmarcVerifier      *DMARCVerifier
	reputationVerifier *ReputationVerifier
	contentVerifier    *ContentVerifier
	config             VerificationConfig
	logger             *slog.Logger
}

// NewVerifier creates a new spam verifier with all components
func NewVerifier(config VerificationConfig, logger *slog.Logger) *Verifier {
	resolver := "8.8.8.8:53" // Default DNS resolver

	return &Verifier{
		spfVerifier:        NewSPFVerifier(resolver),
		dkimVerifier:       NewDKIMVerifier(resolver),
		dmarcVerifier:      NewDMARCVerifier(resolver),
		reputationVerifier: NewReputationVerifier(resolver),
		contentVerifier:    NewContentVerifier(),
		config:             config,
		logger:             logger,
	}
}

// VerifyEmail performs comprehensive spam verification on an email
func (v *Verifier) VerifyEmail(ctx context.Context, emailCtx EmailContext) VerificationResult {
	startTime := time.Now()

	v.logger.Info("Starting email verification",
		"from", emailCtx.From,
		"to", emailCtx.To,
		"subject", emailCtx.Subject,
		"sender_ip", emailCtx.SenderIP,
	)

	result := VerificationResult{
		Timestamp: time.Now(),
		Action:    ActionAccept, // Default to accept
	}

	// Run verifications based on configuration
	if v.config.EnableSPF {
		result.SPF = v.spfVerifier.Verify(ctx, emailCtx)
		v.logger.Debug("SPF verification completed",
			"result", result.SPF.Result.String(),
			"mechanism", result.SPF.Mechanism,
		)
	} else {
		result.SPF = SPFResult{Result: SPFNone}
	}

	if v.config.EnableDKIM {
		result.DKIM = v.dkimVerifier.Verify(ctx, emailCtx)
		v.logger.Debug("DKIM verification completed",
			"valid", result.DKIM.Valid,
			"signatures", len(result.DKIM.Results),
		)
	} else {
		result.DKIM = DKIMResult{Valid: false}
	}

	if v.config.EnableDMARC {
		result.DMARC = v.dmarcVerifier.Verify(ctx, emailCtx, result.SPF, result.DKIM)
		v.logger.Debug("DMARC verification completed",
			"result", result.DMARC.Result.String(),
			"policy", result.DMARC.Policy,
			"spf_aligned", result.DMARC.SPFAlign,
			"dkim_aligned", result.DMARC.DKIMAlign,
		)
	} else {
		result.DMARC = DMARCResult{Result: DMARCNone}
	}

	if v.config.EnableReputation {
		result.Reputation = v.reputationVerifier.Verify(ctx, emailCtx)
		v.logger.Debug("Reputation verification completed",
			"ip_reputation", result.Reputation.IPReputation.String(),
			"domain_reputation", result.Reputation.DomainReputation.String(),
			"score", result.Reputation.Score,
			"blacklisted", result.Reputation.Blacklisted,
		)
	} else {
		result.Reputation = ReputationResult{
			IPReputation:     IPReputationUnknown,
			DomainReputation: DomainReputationUnknown,
			Score:            0.5,
		}
	}

	if v.config.EnableContent {
		result.Content = v.contentVerifier.Verify(ctx, emailCtx)
		v.logger.Debug("Content verification completed",
			"spam_score", result.Content.SpamScore,
			"classification", result.Content.Classification,
			"indicators", result.Content.SpamIndicators,
		)
	} else {
		result.Content = ContentResult{
			SpamScore:      0.0,
			Classification: "not_checked",
		}
	}

	// Determine final action based on all verification results
	result.Action = v.determineAction(result)

	duration := time.Since(startTime)
	v.logger.Info("Email verification completed",
		"duration", duration,
		"action", result.Action.String(),
		"spf", result.SPF.Result.String(),
		"dkim_valid", result.DKIM.Valid,
		"dmarc", result.DMARC.Result.String(),
		"reputation_score", result.Reputation.Score,
		"content_score", result.Content.SpamScore,
	)

	return result
}

// determineAction determines the final action based on all verification results
func (v *Verifier) determineAction(result VerificationResult) Action {
	// Start with a base score
	riskScore := 0.0

	// SPF evaluation
	switch result.SPF.Result {
	case SPFFail:
		riskScore += 0.3
	case SPFSoftFail:
		riskScore += 0.1
	case SPFTempError, SPFPermError:
		riskScore += 0.05
	}

	// DKIM evaluation
	if v.config.EnableDKIM && !result.DKIM.Valid {
		riskScore += 0.2
	}

	// DMARC evaluation
	switch result.DMARC.Result {
	case DMARCFail:
		riskScore += 0.4
	case DMARCTempError, DMARCPermError:
		riskScore += 0.1
	}

	// Reputation evaluation
	switch result.Reputation.IPReputation {
	case IPReputationBad:
		riskScore += 0.5
	case IPReputationSuspicious:
		riskScore += 0.2
	}

	switch result.Reputation.DomainReputation {
	case DomainReputationBad:
		riskScore += 0.4
	case DomainReputationSuspicious:
		riskScore += 0.15
	}

	// Add reputation score (inverted, as lower score means worse reputation)
	riskScore += (1.0 - result.Reputation.Score) * 0.3

	// Content evaluation
	if v.config.EnableContent {
		riskScore += result.Content.SpamScore * 0.4
	}

	// Apply blacklist penalty
	if len(result.Reputation.Blacklisted) > 0 {
		riskScore += float64(len(result.Reputation.Blacklisted)) * 0.2
	}

	// Determine action based on risk score and configuration
	return v.riskScoreToAction(riskScore, result)
}

// riskScoreToAction converts risk score to action based on configuration
func (v *Verifier) riskScoreToAction(riskScore float64, result VerificationResult) Action {
	v.logger.Debug("Calculating action from risk score",
		"risk_score", riskScore,
		"spam_threshold", v.config.SpamThreshold,
		"reject_on_fail", v.config.RejectOnFail,
		"quarantine_mode", v.config.QuarantineMode,
	)

	// Critical failures that should always be rejected (if configured)
	if v.config.RejectOnFail {
		// Hard SPF fail
		if result.SPF.Result == SPFFail {
			return ActionReject
		}

		// DMARC fail with reject policy
		if result.DMARC.Result == DMARCFail && result.DMARC.Policy == "reject" {
			return ActionReject
		}

		// Multiple blacklists
		if len(result.Reputation.Blacklisted) >= 3 {
			return ActionReject
		}
	}

	// Apply spam threshold
	if riskScore >= v.config.SpamThreshold {
		if v.config.QuarantineMode {
			return ActionQuarantine
		} else if v.config.RejectOnFail {
			return ActionReject
		} else {
			return ActionQuarantine
		}
	}

	// Moderate risk - quarantine
	if riskScore >= 0.5 {
		return ActionQuarantine
	}

	// Temporary failures
	if result.SPF.Result == SPFTempError ||
		result.DMARC.Result == DMARCTempError ||
		result.Reputation.Error != "" {
		return ActionTempFail
	}

	// Default to accept
	return ActionAccept
}

// GetVerificationSummary returns a human-readable summary of verification results
func (v *Verifier) GetVerificationSummary(result VerificationResult) string {
	summary := ""

	// SPF summary
	switch result.SPF.Result {
	case SPFPass:
		summary += "SPF: PASS"
	case SPFFail:
		summary += "SPF: FAIL"
	case SPFSoftFail:
		summary += "SPF: SOFTFAIL"
	case SPFNone:
		summary += "SPF: NONE"
	default:
		summary += "SPF: " + result.SPF.Result.String()
	}

	// DKIM summary
	if result.DKIM.Valid {
		summary += ", DKIM: PASS"
	} else {
		summary += ", DKIM: FAIL"
	}

	// DMARC summary
	summary += ", DMARC: " + result.DMARC.Result.String()

	// Reputation summary
	if len(result.Reputation.Blacklisted) > 0 {
		summary += ", BLACKLISTED"
	}

	// Content summary
	if result.Content.SpamScore >= 0.7 {
		summary += ", HIGH SPAM SCORE"
	}

	// Final action
	summary += " -> " + result.Action.String()

	return summary
}

// IsSpam returns true if the email is classified as spam based on verification results
func (v *Verifier) IsSpam(result VerificationResult) bool {
	return result.Action == ActionReject || result.Action == ActionQuarantine
}

// ShouldAccept returns true if the email should be accepted
func (v *Verifier) ShouldAccept(result VerificationResult) bool {
	return result.Action == ActionAccept
}

// ShouldReject returns true if the email should be rejected
func (v *Verifier) ShouldReject(result VerificationResult) bool {
	return result.Action == ActionReject
}

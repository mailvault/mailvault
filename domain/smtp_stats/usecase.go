package smtp_stats

import (
	"context"
	"fmt"
	"net"
	"time"

	"mailvault/app/smtp/verification"
	"mailvault/domain/entities"
	"mailvault/internal/utils"

	"github.com/gofrs/uuid/v5"
)

// UseCase defines the business logic for SMTP statistics operations
type UseCase struct {
	repo Repository
}

// NewUseCase creates a new SMTP statistics use case
func NewUseCase(repo Repository) *UseCase {
	return &UseCase{
		repo: repo,
	}
}

// RecordVerificationResult stores a verification result as a statistic
func (uc *UseCase) RecordVerificationResult(
	ctx context.Context,
	domainID, emailAddressID uuid.UUID,
	senderIP net.IP,
	fromAddress string,
	verificationResult *verification.VerificationResult,
	finalAction verification.Action,
) error {
	// Extract sender domain from email address
	senderDomain := extractDomainFromEmail(fromAddress)

	// Create the statistic record
	stat := &entities.SMTPVerificationStat{
		ID:             uuid.Must(uuid.NewV4()),
		DomainID:       domainID,
		EmailAddressID: emailAddressID,
		VerifiedAt:     time.Now(),
		SenderIP:       senderIP,
		SenderDomain:   senderDomain,
		FromAddress:    fromAddress,

		// SPF results
		SPFResult:    verificationResult.SPF.Result.String(),
		SPFMechanism: verificationResult.SPF.Mechanism,

		// DKIM results
		DKIMValid:    verificationResult.DKIM.Valid,
		DKIMDomain:   extractDKIMDomain(verificationResult.DKIM),
		DKIMSelector: extractDKIMSelector(verificationResult.DKIM),

		// DMARC results
		DMARCResult:        verificationResult.DMARC.Result.String(),
		DMARCPolicy:        verificationResult.DMARC.Policy,
		DMARCAlignmentSPF:  verificationResult.DMARC.SPFAlign,
		DMARCAlignmentDKIM: verificationResult.DMARC.DKIMAlign,

		// Content analysis
		SpamScore:      verificationResult.Content.SpamScore,
		ContentVerdict: determineContentVerdict(verificationResult.Content.SpamScore),

		// Reputation
		ReputationScore: verificationResult.Reputation.Score,
		IsBlacklisted:   len(verificationResult.Reputation.Blacklisted) > 0,

		// Final action
		FinalAction:   finalAction.String(),
		IsQuarantined: finalAction == verification.ActionQuarantine,
	}

	return uc.repo.CreateStat(ctx, stat)
}

// GetDomainStats retrieves statistics for a specific domain
func (uc *UseCase) GetDomainStats(
	ctx context.Context,
	domainID uuid.UUID,
	filter entities.SMTPStatsFilter,
	page, pageSize int,
) ([]entities.SMTPVerificationStat, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	offset := (page - 1) * pageSize
	filter.DomainID = &domainID

	return uc.repo.GetStatsForDomain(ctx, domainID, filter, pageSize, offset)
}

// GetEmailAddressStats retrieves statistics for a specific email address
func (uc *UseCase) GetEmailAddressStats(
	ctx context.Context,
	emailAddressID uuid.UUID,
	filter entities.SMTPStatsFilter,
	page, pageSize int,
) ([]entities.SMTPVerificationStat, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	offset := (page - 1) * pageSize
	filter.EmailAddressID = &emailAddressID

	return uc.repo.GetStatsForEmailAddress(ctx, emailAddressID, filter, pageSize, offset)
}

// GetOverview retrieves overview statistics
func (uc *UseCase) GetOverview(ctx context.Context, filter entities.SMTPStatsFilter) (*entities.SMTPStatsOverview, error) {
	return uc.repo.GetOverview(ctx, filter)
}

// GetTimeSeriesData retrieves time-series data for visualization
func (uc *UseCase) GetTimeSeriesData(
	ctx context.Context,
	filter entities.SMTPStatsFilter,
	granularity string,
) ([]entities.TimeSeriesPoint, error) {
	// Validate granularity
	validGranularities := map[string]bool{
		"hour": true, "day": true, "week": true, "month": true,
	}

	if !validGranularities[granularity] {
		granularity = "day"
	}

	return uc.repo.GetTimeSeriesData(ctx, filter, granularity)
}

// GetDistributions retrieves all distribution data for dashboard
func (uc *UseCase) GetDistributions(ctx context.Context, filter entities.SMTPStatsFilter) (map[string]interface{}, error) {
	distributions := make(map[string]interface{})

	// Get action distribution
	actionDist, err := uc.repo.GetActionDistribution(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("getting action distribution: %w", err)
	}
	distributions["actions"] = actionDist

	// Get reputation distribution
	repDist, err := uc.repo.GetReputationDistribution(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("getting reputation distribution: %w", err)
	}
	distributions["reputation"] = repDist

	// Get content distribution
	contentDist, err := uc.repo.GetContentDistribution(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("getting content distribution: %w", err)
	}
	distributions["content"] = contentDist

	// Get SPF distribution
	spfDist, err := uc.repo.GetSPFDistribution(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("getting SPF distribution: %w", err)
	}
	distributions["spf"] = spfDist

	// Get DKIM distribution
	dkimDist, err := uc.repo.GetDKIMDistribution(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("getting DKIM distribution: %w", err)
	}
	distributions["dkim"] = dkimDist

	// Get DMARC distribution
	dmarcDist, err := uc.repo.GetDMARCDistribution(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("getting DMARC distribution: %w", err)
	}
	distributions["dmarc"] = dmarcDist

	return distributions, nil
}

// GetTopSenders retrieves top sender domains and IPs
func (uc *UseCase) GetTopSenders(ctx context.Context, filter entities.SMTPStatsFilter, limit int) (map[string]interface{}, error) {
	if limit < 1 || limit > 100 {
		limit = 10
	}

	senders := make(map[string]interface{})

	// Get top sender domains
	topDomains, err := uc.repo.GetTopSenderDomains(ctx, filter, limit)
	if err != nil {
		return nil, fmt.Errorf("getting top sender domains: %w", err)
	}
	senders["domains"] = topDomains

	// Get top sender IPs
	topIPs, err := uc.repo.GetTopSenderIPs(ctx, filter, limit)
	if err != nil {
		return nil, fmt.Errorf("getting top sender IPs: %w", err)
	}
	senders["ips"] = topIPs

	return senders, nil
}

// CleanupOldStats removes statistics older than the specified duration
func (uc *UseCase) CleanupOldStats(ctx context.Context, retentionPeriod time.Duration) (int64, error) {
	if retentionPeriod < 24*time.Hour {
		return 0, fmt.Errorf("retention period must be at least 24 hours")
	}

	return uc.repo.DeleteOldStats(ctx, retentionPeriod)
}

// Helper functions

// extractDomainFromEmail extracts the domain part from an email address
func extractDomainFromEmail(email string) string {
	domain, err := utils.ExtractDomain(email)
	if err != nil {
		return ""
	}
	return domain
}

// determineContentVerdict determines verdict based on spam score
func determineContentVerdict(spamScore float64) string {
	if spamScore >= 0.8 {
		return "spam"
	} else if spamScore >= 0.3 {
		return "suspicious"
	}
	return "clean"
}

// extractDKIMDomain extracts the first valid DKIM domain from results
func extractDKIMDomain(dkimResult verification.DKIMResult) string {
	if len(dkimResult.Results) > 0 {
		return dkimResult.Results[0].Domain
	}
	return ""
}

// extractDKIMSelector extracts the first valid DKIM selector from results
func extractDKIMSelector(dkimResult verification.DKIMResult) string {
	if len(dkimResult.Results) > 0 {
		return dkimResult.Results[0].Selector
	}
	return ""
}

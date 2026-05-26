package smtp_stats

import (
	"context"
	"time"

	"github.com/mailvault/mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
)

// Repository defines the interface for SMTP statistics storage operations
type Repository interface {
	// CreateStat stores a new SMTP verification statistic
	CreateStat(ctx context.Context, stat *entities.SMTPVerificationStat) error

	// GetStatsForDomain retrieves statistics for a specific domain
	GetStatsForDomain(ctx context.Context, domainID uuid.UUID, filter entities.SMTPStatsFilter, limit, offset int) ([]entities.SMTPVerificationStat, int64, error)

	// GetStatsForEmailAddress retrieves statistics for a specific email address
	GetStatsForEmailAddress(ctx context.Context, emailAddressID uuid.UUID, filter entities.SMTPStatsFilter, limit, offset int) ([]entities.SMTPVerificationStat, int64, error)

	// GetOverview retrieves overview statistics with optional filtering
	GetOverview(ctx context.Context, filter entities.SMTPStatsFilter) (*entities.SMTPStatsOverview, error)

	// GetTimeSeriesData retrieves time-series data for charts
	GetTimeSeriesData(ctx context.Context, filter entities.SMTPStatsFilter, granularity string) ([]entities.TimeSeriesPoint, error)

	// GetActionDistribution retrieves distribution of actions taken
	GetActionDistribution(ctx context.Context, filter entities.SMTPStatsFilter) ([]entities.ActionDistribution, error)

	// GetReputationDistribution retrieves distribution of reputation scores
	GetReputationDistribution(ctx context.Context, filter entities.SMTPStatsFilter) ([]entities.ReputationDistribution, error)

	// GetContentDistribution retrieves distribution of content analysis results
	GetContentDistribution(ctx context.Context, filter entities.SMTPStatsFilter) ([]entities.ContentDistribution, error)

	// GetSPFDistribution retrieves distribution of SPF results
	GetSPFDistribution(ctx context.Context, filter entities.SMTPStatsFilter) ([]entities.SPFDistribution, error)

	// GetDKIMDistribution retrieves distribution of DKIM validation results
	GetDKIMDistribution(ctx context.Context, filter entities.SMTPStatsFilter) ([]entities.DKIMDistribution, error)

	// GetDMARCDistribution retrieves distribution of DMARC results
	GetDMARCDistribution(ctx context.Context, filter entities.SMTPStatsFilter) ([]entities.DMARCDistribution, error)

	// GetTopSenderDomains retrieves most active sender domains
	GetTopSenderDomains(ctx context.Context, filter entities.SMTPStatsFilter, limit int) ([]struct {
		Domain string `json:"domain"`
		Count  int64  `json:"count"`
	}, error)

	// GetTopSenderIPs retrieves most active sender IPs
	GetTopSenderIPs(ctx context.Context, filter entities.SMTPStatsFilter, limit int) ([]struct {
		IP    string `json:"ip"`
		Count int64  `json:"count"`
	}, error)

	// DeleteOldStats removes statistics older than the specified duration
	DeleteOldStats(ctx context.Context, olderThan time.Duration) (int64, error)
}

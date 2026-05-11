package entities

import (
	"net"
	"time"

	"github.com/gofrs/uuid/v5"
)

// SMTPVerificationStat represents a single SMTP verification statistic record
type SMTPVerificationStat struct {
	ID             uuid.UUID `json:"id"`
	DomainID       uuid.UUID `json:"domain_id"`
	EmailAddressID uuid.UUID `json:"email_address_id"`
	VerifiedAt     time.Time `json:"verified_at"`

	// Sender information
	SenderIP     net.IP `json:"sender_ip"`
	SenderDomain string `json:"sender_domain"`
	FromAddress  string `json:"from_address"`

	// SPF results
	SPFResult    string `json:"spf_result"`
	SPFMechanism string `json:"spf_mechanism"`

	// DKIM results
	DKIMValid    bool   `json:"dkim_valid"`
	DKIMDomain   string `json:"dkim_domain"`
	DKIMSelector string `json:"dkim_selector"`

	// DMARC results
	DMARCResult        string `json:"dmarc_result"`
	DMARCPolicy        string `json:"dmarc_policy"`
	DMARCAlignmentSPF  bool   `json:"dmarc_alignment_spf"`
	DMARCAlignmentDKIM bool   `json:"dmarc_alignment_dkim"`

	// Content analysis
	SpamScore      float64 `json:"spam_score"`
	ContentVerdict string  `json:"content_verdict"`

	// Reputation
	ReputationScore float64 `json:"reputation_score"`
	IsBlacklisted   bool    `json:"is_blacklisted"`

	// Final action
	FinalAction   string `json:"final_action"`
	IsQuarantined bool   `json:"is_quarantined"`
}

// SMTPStatsOverview provides overview statistics for SMTP verification
type SMTPStatsOverview struct {
	TotalProcessed   int64             `json:"total_processed"`
	AcceptedCount    int64             `json:"accepted_count"`
	RejectedCount    int64             `json:"rejected_count"`
	QuarantinedCount int64             `json:"quarantined_count"`
	TempFailCount    int64             `json:"temp_fail_count"`
	AverageSpamScore float64           `json:"average_spam_score"`
	ActionBreakdown  map[string]int64  `json:"action_breakdown"`
	TimeSeriesData   []TimeSeriesPoint `json:"time_series_data"`
}

// TimeSeriesPoint represents a point in time for statistics
type TimeSeriesPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	Accepted    int64     `json:"accepted"`
	Rejected    int64     `json:"rejected"`
	Quarantined int64     `json:"quarantined"`
	TempFail    int64     `json:"temp_fail"`
}

// SMTPStatsFilter provides filtering options for statistics queries
type SMTPStatsFilter struct {
	DomainID       *uuid.UUID `json:"domain_id,omitempty"`
	EmailAddressID *uuid.UUID `json:"email_address_id,omitempty"`
	StartDate      *time.Time `json:"start_date,omitempty"`
	EndDate        *time.Time `json:"end_date,omitempty"`
	FinalAction    string     `json:"final_action,omitempty"`
	SenderDomain   string     `json:"sender_domain,omitempty"`
	MinSpamScore   *float64   `json:"min_spam_score,omitempty"`
	MaxSpamScore   *float64   `json:"max_spam_score,omitempty"`
}

// ActionDistribution provides breakdown of actions taken
type ActionDistribution struct {
	Action     string  `json:"action"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

// ReputationDistribution provides breakdown of reputation scores
type ReputationDistribution struct {
	ScoreRange string  `json:"score_range"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

// ContentDistribution provides breakdown of content analysis results
type ContentDistribution struct {
	Verdict    string  `json:"verdict"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

// SPFDistribution provides breakdown of SPF results
type SPFDistribution struct {
	Result     string  `json:"result"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

// DKIMDistribution provides breakdown of DKIM validation results
type DKIMDistribution struct {
	Valid      bool    `json:"valid"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

// DMARCDistribution provides breakdown of DMARC results
type DMARCDistribution struct {
	Result     string  `json:"result"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

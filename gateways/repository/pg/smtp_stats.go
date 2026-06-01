package pg

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/mailvault/mailvault/domain/entities"
	"github.com/mailvault/mailvault/domain/smtp_stats"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SMTPStatsRepository implements the SMTP statistics repository using PostgreSQL
type SMTPStatsRepository struct {
	db *pgxpool.Pool
}

// NewSMTPStatsRepository creates a new SMTP statistics repository
func NewSMTPStatsRepository(db *pgxpool.Pool) smtp_stats.Repository {
	return &SMTPStatsRepository{db: db}
}

// CreateStat stores a new SMTP verification statistic
func (r *SMTPStatsRepository) CreateStat(ctx context.Context, stat *entities.SMTPVerificationStat) error {
	query := `
		INSERT INTO smtp_verification_stats (
			id, domain_id, email_address_id, verified_at,
			sender_ip, sender_domain, from_address,
			spf_result, spf_mechanism,
			dkim_valid, dkim_domain, dkim_selector,
			dmarc_result, dmarc_policy, dmarc_alignment_spf, dmarc_alignment_dkim,
			spam_score, content_verdict,
			reputation_score, is_blacklisted,
			final_action, is_quarantined
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			$8, $9,
			$10, $11, $12,
			$13, $14, $15, $16,
			$17, $18,
			$19, $20,
			$21, $22
		)`

	_, err := r.db.Exec(ctx, query,
		stat.ID, stat.DomainID, stat.EmailAddressID, stat.VerifiedAt,
		stat.SenderIP, stat.SenderDomain, stat.FromAddress,
		stat.SPFResult, stat.SPFMechanism,
		stat.DKIMValid, stat.DKIMDomain, stat.DKIMSelector,
		stat.DMARCResult, stat.DMARCPolicy, stat.DMARCAlignmentSPF, stat.DMARCAlignmentDKIM,
		stat.SpamScore, stat.ContentVerdict,
		stat.ReputationScore, stat.IsBlacklisted,
		stat.FinalAction, stat.IsQuarantined,
	)

	if err != nil {
		return fmt.Errorf("creating SMTP stat: %w", err)
	}

	return nil
}

// GetStatsForDomain retrieves statistics for a specific domain
func (r *SMTPStatsRepository) GetStatsForDomain(
	ctx context.Context,
	domainID uuid.UUID,
	filter entities.SMTPStatsFilter,
	limit, offset int,
) ([]entities.SMTPVerificationStat, int64, error) {
	whereClause, args := r.buildWhereClause(filter, []interface{}{domainID})

	// Get total count
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) 
		FROM smtp_verification_stats 
		WHERE domain_id = $1 %s`, whereClause)

	var total int64
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting stats: %w", err)
	}

	// Get paginated results
	args = append(args, limit, offset)
	dataQuery := fmt.Sprintf(`
		SELECT id, domain_id, email_address_id, verified_at,
			   sender_ip, sender_domain, from_address,
			   spf_result, spf_mechanism,
			   dkim_valid, dkim_domain, dkim_selector,
			   dmarc_result, dmarc_policy, dmarc_alignment_spf, dmarc_alignment_dkim,
			   spam_score, content_verdict,
			   reputation_score, is_blacklisted,
			   final_action, is_quarantined
		FROM smtp_verification_stats 
		WHERE domain_id = $1 %s
		ORDER BY verified_at DESC
		LIMIT $%d OFFSET $%d`, whereClause, len(args)-1, len(args))

	rows, err := r.db.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying stats: %w", err)
	}
	defer rows.Close()

	var stats []entities.SMTPVerificationStat
	for rows.Next() {
		var stat entities.SMTPVerificationStat
		var senderIP string

		err := rows.Scan(
			&stat.ID, &stat.DomainID, &stat.EmailAddressID, &stat.VerifiedAt,
			&senderIP, &stat.SenderDomain, &stat.FromAddress,
			&stat.SPFResult, &stat.SPFMechanism,
			&stat.DKIMValid, &stat.DKIMDomain, &stat.DKIMSelector,
			&stat.DMARCResult, &stat.DMARCPolicy, &stat.DMARCAlignmentSPF, &stat.DMARCAlignmentDKIM,
			&stat.SpamScore, &stat.ContentVerdict,
			&stat.ReputationScore, &stat.IsBlacklisted,
			&stat.FinalAction, &stat.IsQuarantined,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scanning stat: %w", err)
		}

		if senderIP != "" {
			stat.SenderIP = net.ParseIP(senderIP)
		}

		stats = append(stats, stat)
	}

	return stats, total, nil
}

// GetStatsForEmailAddress retrieves statistics for a specific email address
func (r *SMTPStatsRepository) GetStatsForEmailAddress(
	ctx context.Context,
	emailAddressID uuid.UUID,
	filter entities.SMTPStatsFilter,
	limit, offset int,
) ([]entities.SMTPVerificationStat, int64, error) {
	filter.EmailAddressID = &emailAddressID
	return r.GetStatsForDomain(ctx, uuid.Nil, filter, limit, offset)
}

// GetOverview retrieves overview statistics
func (r *SMTPStatsRepository) GetOverview(ctx context.Context, filter entities.SMTPStatsFilter) (*entities.SMTPStatsOverview, error) {
	whereClause, args := r.buildWhereClause(filter, []interface{}{})

	query := fmt.Sprintf(`
		SELECT 
			COUNT(*) as total_processed,
			COUNT(*) FILTER (WHERE final_action = 'accept') as accepted_count,
			COUNT(*) FILTER (WHERE final_action = 'reject') as rejected_count,
			COUNT(*) FILTER (WHERE final_action = 'quarantine') as quarantined_count,
			COUNT(*) FILTER (WHERE final_action = 'tempfail') as temp_fail_count,
			COALESCE(AVG(spam_score), 0) as average_spam_score
		FROM smtp_verification_stats
		%s`, r.formatWhereClause(whereClause))

	overview := &entities.SMTPStatsOverview{}

	err := r.db.QueryRow(ctx, query, args...).Scan(
		&overview.TotalProcessed,
		&overview.AcceptedCount,
		&overview.RejectedCount,
		&overview.QuarantinedCount,
		&overview.TempFailCount,
		&overview.AverageSpamScore,
	)
	if err != nil {
		return nil, fmt.Errorf("getting overview: %w", err)
	}

	// Get action breakdown
	actionQuery := fmt.Sprintf(`
		SELECT final_action, COUNT(*) 
		FROM smtp_verification_stats 
		%s
		GROUP BY final_action`, r.formatWhereClause(whereClause))

	actionRows, err := r.db.Query(ctx, actionQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("getting action breakdown: %w", err)
	}
	defer actionRows.Close()

	overview.ActionBreakdown = make(map[string]int64)
	for actionRows.Next() {
		var action string
		var count int64
		if err := actionRows.Scan(&action, &count); err != nil {
			return nil, fmt.Errorf("scanning action breakdown: %w", err)
		}
		overview.ActionBreakdown[action] = count
	}

	return overview, nil
}

// GetTimeSeriesData retrieves time-series data for charts
func (r *SMTPStatsRepository) GetTimeSeriesData(
	ctx context.Context,
	filter entities.SMTPStatsFilter,
	granularity string,
) ([]entities.TimeSeriesPoint, error) {
	whereClause, args := r.buildWhereClause(filter, []interface{}{})

	var truncateFunc string
	switch granularity {
	case "hour":
		truncateFunc = "date_trunc('hour', verified_at)"
	case "day":
		truncateFunc = "date_trunc('day', verified_at)"
	case "week":
		truncateFunc = "date_trunc('week', verified_at)"
	case "month":
		truncateFunc = "date_trunc('month', verified_at)"
	default:
		truncateFunc = "date_trunc('day', verified_at)"
	}

	query := fmt.Sprintf(`
		SELECT 
			%s as timestamp,
			COUNT(*) FILTER (WHERE final_action = 'accept') as accepted,
			COUNT(*) FILTER (WHERE final_action = 'reject') as rejected,
			COUNT(*) FILTER (WHERE final_action = 'quarantine') as quarantined,
			COUNT(*) FILTER (WHERE final_action = 'tempfail') as temp_fail
		FROM smtp_verification_stats
		%s
		GROUP BY timestamp
		ORDER BY timestamp`, truncateFunc, r.formatWhereClause(whereClause))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("getting time series data: %w", err)
	}
	defer rows.Close()

	var points []entities.TimeSeriesPoint
	for rows.Next() {
		var point entities.TimeSeriesPoint
		err := rows.Scan(
			&point.Timestamp,
			&point.Accepted,
			&point.Rejected,
			&point.Quarantined,
			&point.TempFail,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning time series point: %w", err)
		}
		points = append(points, point)
	}

	return points, nil
}

// GetActionDistribution retrieves distribution of actions taken
func (r *SMTPStatsRepository) GetActionDistribution(ctx context.Context, filter entities.SMTPStatsFilter) ([]entities.ActionDistribution, error) {
	whereClause, args := r.buildWhereClause(filter, []interface{}{})

	query := fmt.Sprintf(`
		SELECT 
			final_action,
			COUNT(*) as count,
			COUNT(*) * 100.0 / SUM(COUNT(*)) OVER() as percentage
		FROM smtp_verification_stats
		%s
		GROUP BY final_action
		ORDER BY count DESC`, r.formatWhereClause(whereClause))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("getting action distribution: %w", err)
	}
	defer rows.Close()

	var distributions []entities.ActionDistribution
	for rows.Next() {
		var dist entities.ActionDistribution
		err := rows.Scan(&dist.Action, &dist.Count, &dist.Percentage)
		if err != nil {
			return nil, fmt.Errorf("scanning action distribution: %w", err)
		}
		distributions = append(distributions, dist)
	}

	return distributions, nil
}

// GetReputationDistribution retrieves distribution of reputation scores
func (r *SMTPStatsRepository) GetReputationDistribution(ctx context.Context, filter entities.SMTPStatsFilter) ([]entities.ReputationDistribution, error) {
	whereClause, args := r.buildWhereClause(filter, []interface{}{})

	query := fmt.Sprintf(`
		SELECT 
			CASE 
				WHEN reputation_score >= 0.8 THEN '0.8-1.0'
				WHEN reputation_score >= 0.6 THEN '0.6-0.8'
				WHEN reputation_score >= 0.4 THEN '0.4-0.6'
				WHEN reputation_score >= 0.2 THEN '0.2-0.4'
				ELSE '0.0-0.2'
			END as score_range,
			COUNT(*) as count,
			COUNT(*) * 100.0 / SUM(COUNT(*)) OVER() as percentage
		FROM smtp_verification_stats
		%s
		GROUP BY score_range
		ORDER BY score_range`, r.formatWhereClause(whereClause))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("getting reputation distribution: %w", err)
	}
	defer rows.Close()

	var distributions []entities.ReputationDistribution
	for rows.Next() {
		var dist entities.ReputationDistribution
		err := rows.Scan(&dist.ScoreRange, &dist.Count, &dist.Percentage)
		if err != nil {
			return nil, fmt.Errorf("scanning reputation distribution: %w", err)
		}
		distributions = append(distributions, dist)
	}

	return distributions, nil
}

// GetContentDistribution retrieves distribution of content analysis results
func (r *SMTPStatsRepository) GetContentDistribution(ctx context.Context, filter entities.SMTPStatsFilter) ([]entities.ContentDistribution, error) {
	whereClause, args := r.buildWhereClause(filter, []interface{}{})

	query := fmt.Sprintf(`
		SELECT 
			content_verdict,
			COUNT(*) as count,
			COUNT(*) * 100.0 / SUM(COUNT(*)) OVER() as percentage
		FROM smtp_verification_stats
		%s
		GROUP BY content_verdict
		ORDER BY count DESC`, r.formatWhereClause(whereClause))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("getting content distribution: %w", err)
	}
	defer rows.Close()

	var distributions []entities.ContentDistribution
	for rows.Next() {
		var dist entities.ContentDistribution
		err := rows.Scan(&dist.Verdict, &dist.Count, &dist.Percentage)
		if err != nil {
			return nil, fmt.Errorf("scanning content distribution: %w", err)
		}
		distributions = append(distributions, dist)
	}

	return distributions, nil
}

// GetSPFDistribution retrieves distribution of SPF results
func (r *SMTPStatsRepository) GetSPFDistribution(ctx context.Context, filter entities.SMTPStatsFilter) ([]entities.SPFDistribution, error) {
	whereClause, args := r.buildWhereClause(filter, []interface{}{})

	query := fmt.Sprintf(`
		SELECT 
			spf_result,
			COUNT(*) as count,
			COUNT(*) * 100.0 / SUM(COUNT(*)) OVER() as percentage
		FROM smtp_verification_stats
		%s
		GROUP BY spf_result
		ORDER BY count DESC`, r.formatWhereClause(whereClause))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("getting SPF distribution: %w", err)
	}
	defer rows.Close()

	var distributions []entities.SPFDistribution
	for rows.Next() {
		var dist entities.SPFDistribution
		err := rows.Scan(&dist.Result, &dist.Count, &dist.Percentage)
		if err != nil {
			return nil, fmt.Errorf("scanning SPF distribution: %w", err)
		}
		distributions = append(distributions, dist)
	}

	return distributions, nil
}

// GetDKIMDistribution retrieves distribution of DKIM validation results
func (r *SMTPStatsRepository) GetDKIMDistribution(ctx context.Context, filter entities.SMTPStatsFilter) ([]entities.DKIMDistribution, error) {
	whereClause, args := r.buildWhereClause(filter, []interface{}{})

	query := fmt.Sprintf(`
		SELECT 
			dkim_valid,
			COUNT(*) as count,
			COUNT(*) * 100.0 / SUM(COUNT(*)) OVER() as percentage
		FROM smtp_verification_stats
		%s
		GROUP BY dkim_valid
		ORDER BY count DESC`, r.formatWhereClause(whereClause))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("getting DKIM distribution: %w", err)
	}
	defer rows.Close()

	var distributions []entities.DKIMDistribution
	for rows.Next() {
		var dist entities.DKIMDistribution
		err := rows.Scan(&dist.Valid, &dist.Count, &dist.Percentage)
		if err != nil {
			return nil, fmt.Errorf("scanning DKIM distribution: %w", err)
		}
		distributions = append(distributions, dist)
	}

	return distributions, nil
}

// GetDMARCDistribution retrieves distribution of DMARC results
func (r *SMTPStatsRepository) GetDMARCDistribution(ctx context.Context, filter entities.SMTPStatsFilter) ([]entities.DMARCDistribution, error) {
	whereClause, args := r.buildWhereClause(filter, []interface{}{})

	query := fmt.Sprintf(`
		SELECT 
			dmarc_result,
			COUNT(*) as count,
			COUNT(*) * 100.0 / SUM(COUNT(*)) OVER() as percentage
		FROM smtp_verification_stats
		%s
		GROUP BY dmarc_result
		ORDER BY count DESC`, r.formatWhereClause(whereClause))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("getting DMARC distribution: %w", err)
	}
	defer rows.Close()

	var distributions []entities.DMARCDistribution
	for rows.Next() {
		var dist entities.DMARCDistribution
		err := rows.Scan(&dist.Result, &dist.Count, &dist.Percentage)
		if err != nil {
			return nil, fmt.Errorf("scanning DMARC distribution: %w", err)
		}
		distributions = append(distributions, dist)
	}

	return distributions, nil
}

// GetTopSenderDomains retrieves most active sender domains
func (r *SMTPStatsRepository) GetTopSenderDomains(
	ctx context.Context,
	filter entities.SMTPStatsFilter,
	limit int,
) ([]struct {
	Domain string `json:"domain"`
	Count  int64  `json:"count"`
}, error) {
	whereClause, args := r.buildWhereClause(filter, []interface{}{})
	args = append(args, limit)

	query := fmt.Sprintf(`
		SELECT sender_domain, COUNT(*) as count
		FROM smtp_verification_stats
		%s
		AND sender_domain != ''
		GROUP BY sender_domain
		ORDER BY count DESC
		LIMIT $%d`, r.formatWhereClause(whereClause), len(args))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("getting top sender domains: %w", err)
	}
	defer rows.Close()

	var results []struct {
		Domain string `json:"domain"`
		Count  int64  `json:"count"`
	}

	for rows.Next() {
		var result struct {
			Domain string `json:"domain"`
			Count  int64  `json:"count"`
		}
		err := rows.Scan(&result.Domain, &result.Count)
		if err != nil {
			return nil, fmt.Errorf("scanning top sender domain: %w", err)
		}
		results = append(results, result)
	}

	return results, nil
}

// GetTopSenderIPs retrieves most active sender IPs
func (r *SMTPStatsRepository) GetTopSenderIPs(
	ctx context.Context,
	filter entities.SMTPStatsFilter,
	limit int,
) ([]struct {
	IP    string `json:"ip"`
	Count int64  `json:"count"`
}, error) {
	whereClause, args := r.buildWhereClause(filter, []interface{}{})
	args = append(args, limit)

	query := fmt.Sprintf(`
		SELECT sender_ip::text, COUNT(*) as count
		FROM smtp_verification_stats
		%s
		AND sender_ip IS NOT NULL
		GROUP BY sender_ip
		ORDER BY count DESC
		LIMIT $%d`, r.formatWhereClause(whereClause), len(args))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("getting top sender IPs: %w", err)
	}
	defer rows.Close()

	var results []struct {
		IP    string `json:"ip"`
		Count int64  `json:"count"`
	}

	for rows.Next() {
		var result struct {
			IP    string `json:"ip"`
			Count int64  `json:"count"`
		}
		err := rows.Scan(&result.IP, &result.Count)
		if err != nil {
			return nil, fmt.Errorf("scanning top sender IP: %w", err)
		}
		results = append(results, result)
	}

	return results, nil
}

// DeleteOldStats removes statistics older than the specified duration
func (r *SMTPStatsRepository) DeleteOldStats(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoffTime := time.Now().Add(-olderThan)

	query := `DELETE FROM smtp_verification_stats WHERE verified_at < $1`

	result, err := r.db.Exec(ctx, query, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("deleting old stats: %w", err)
	}

	return result.RowsAffected(), nil
}

// Helper methods

// buildWhereClause builds WHERE clause conditions from filter
func (r *SMTPStatsRepository) buildWhereClause(filter entities.SMTPStatsFilter, baseArgs []interface{}) (string, []interface{}) {
	var conditions []string
	args := baseArgs

	argIndex := len(baseArgs) + 1

	if filter.DomainID != nil {
		conditions = append(conditions, fmt.Sprintf("domain_id = $%d", argIndex))
		args = append(args, *filter.DomainID)
		argIndex++
	}

	if filter.EmailAddressID != nil {
		conditions = append(conditions, fmt.Sprintf("email_address_id = $%d", argIndex))
		args = append(args, *filter.EmailAddressID)
		argIndex++
	}

	if filter.StartDate != nil {
		conditions = append(conditions, fmt.Sprintf("verified_at >= $%d", argIndex))
		args = append(args, *filter.StartDate)
		argIndex++
	}

	if filter.EndDate != nil {
		conditions = append(conditions, fmt.Sprintf("verified_at <= $%d", argIndex))
		args = append(args, *filter.EndDate)
		argIndex++
	}

	if filter.FinalAction != "" {
		conditions = append(conditions, fmt.Sprintf("final_action = $%d", argIndex))
		args = append(args, filter.FinalAction)
		argIndex++
	}

	if filter.SenderDomain != "" {
		conditions = append(conditions, fmt.Sprintf("sender_domain ILIKE $%d", argIndex))
		args = append(args, "%"+filter.SenderDomain+"%")
		argIndex++
	}

	if filter.MinSpamScore != nil {
		conditions = append(conditions, fmt.Sprintf("spam_score >= $%d", argIndex))
		args = append(args, *filter.MinSpamScore)
		argIndex++
	}

	if filter.MaxSpamScore != nil {
		conditions = append(conditions, fmt.Sprintf("spam_score <= $%d", argIndex))
		args = append(args, *filter.MaxSpamScore)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "AND " + strings.Join(conditions, " AND ")
	}

	return whereClause, args
}

// formatWhereClause formats the WHERE clause for inclusion in SQL
func (r *SMTPStatsRepository) formatWhereClause(whereClause string) string {
	if whereClause == "" {
		return ""
	}
	return "WHERE " + strings.TrimPrefix(whereClause, "AND ")
}

-- Drop indexes
DROP INDEX IF EXISTS idx_smtp_stats_is_quarantined;
DROP INDEX IF EXISTS idx_smtp_stats_dmarc_result;
DROP INDEX IF EXISTS idx_smtp_stats_dkim_valid;
DROP INDEX IF EXISTS idx_smtp_stats_spf_result;
DROP INDEX IF EXISTS idx_smtp_stats_final_action;
DROP INDEX IF EXISTS idx_smtp_stats_sender_ip;
DROP INDEX IF EXISTS idx_smtp_stats_verified_at;
DROP INDEX IF EXISTS idx_smtp_stats_email_address_id;
DROP INDEX IF EXISTS idx_smtp_stats_domain_id;

-- Drop table
DROP TABLE IF EXISTS smtp_verification_stats;
-- Add performance indexes for common query patterns

-- Index for email lookup by full address (used in SMTP processing)
-- This supports queries joining email_addresses and domains for email routing
CREATE INDEX IF NOT EXISTS idx_email_addresses_local_part_domain
ON email_addresses(local_part, domain_id);

-- Index for faster domain lookups by name (used frequently in SMTP)
CREATE INDEX IF NOT EXISTS idx_domains_domain_lookup
ON domains(domain) WHERE verified = true;

-- Index for received emails from_address (used in spam analysis and filtering)
CREATE INDEX IF NOT EXISTS idx_received_emails_from_address
ON received_emails(from_address);

-- Index for received emails by domain (for user email listing)
CREATE INDEX IF NOT EXISTS idx_received_emails_domain_user
ON received_emails(received_at DESC)
INCLUDE (email_address_id, from_address, subject);

-- Index for email address foreign key performance
CREATE INDEX IF NOT EXISTS idx_received_emails_email_address_received_at
ON received_emails(email_address_id, received_at DESC);

-- Composite index for user domain statistics queries
CREATE INDEX IF NOT EXISTS idx_domains_user_verified
ON domains(user_id, verified)
INCLUDE (domain, created_at);

-- Index for SMTP verification stats queries
CREATE INDEX IF NOT EXISTS idx_smtp_stats_domain_verified_at
ON smtp_verification_stats(domain_id, verified_at DESC);

-- Index for user search and admin queries
CREATE INDEX IF NOT EXISTS idx_users_created_account_type
ON users(created_at DESC, account_type);

-- Partial index for active domains only (most common queries)
CREATE INDEX IF NOT EXISTS idx_domains_active_only
ON domains(id, user_id, domain)
WHERE verified = true;

-- Comments for maintenance
COMMENT ON INDEX idx_email_addresses_local_part_domain IS 'Supports email routing queries in SMTP processing';
COMMENT ON INDEX idx_domains_domain_lookup IS 'Optimizes domain verification in SMTP sessions';
COMMENT ON INDEX idx_received_emails_from_address IS 'Supports spam analysis and sender reputation queries';
COMMENT ON INDEX idx_received_emails_domain_user IS 'Optimizes user email listing with covering index';
COMMENT ON INDEX idx_smtp_stats_domain_verified_at IS 'Supports SMTP verification statistics queries';
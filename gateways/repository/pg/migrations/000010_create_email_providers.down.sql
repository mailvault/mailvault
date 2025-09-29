-- Rollback email provider integration system

-- Drop trigger and function
DROP TRIGGER IF EXISTS trigger_email_providers_updated_at ON email_providers;
DROP FUNCTION IF EXISTS update_email_provider_updated_at();

-- Drop indexes
DROP INDEX IF EXISTS idx_email_provider_logs_recent;
DROP INDEX IF EXISTS idx_email_providers_healthy;
DROP INDEX IF EXISTS idx_email_providers_domain_default_unique;
DROP INDEX IF EXISTS idx_sent_emails_provider_name;
DROP INDEX IF EXISTS idx_sent_emails_provider_id;
DROP INDEX IF EXISTS idx_email_provider_logs_performance;
DROP INDEX IF EXISTS idx_email_provider_logs_created_at;
DROP INDEX IF EXISTS idx_email_provider_logs_operation;
DROP INDEX IF EXISTS idx_email_provider_logs_sent_email_id;
DROP INDEX IF EXISTS idx_email_provider_logs_provider_id;
DROP INDEX IF EXISTS idx_email_providers_retry;
DROP INDEX IF EXISTS idx_email_providers_health;
DROP INDEX IF EXISTS idx_email_providers_status;
DROP INDEX IF EXISTS idx_email_providers_type;
DROP INDEX IF EXISTS idx_email_providers_domain_default;
DROP INDEX IF EXISTS idx_email_providers_domain_priority;
DROP INDEX IF EXISTS idx_email_providers_domain_id;

-- Remove columns from sent_emails
ALTER TABLE sent_emails DROP COLUMN IF EXISTS last_provider_error;
ALTER TABLE sent_emails DROP COLUMN IF EXISTS provider_attempt_count;
ALTER TABLE sent_emails DROP COLUMN IF EXISTS provider_name;
ALTER TABLE sent_emails DROP COLUMN IF EXISTS provider_id;

-- Drop tables
DROP TABLE IF EXISTS email_provider_logs;
DROP TABLE IF EXISTS email_providers;

-- Drop types
DROP TYPE IF EXISTS email_provider_status;
DROP TYPE IF EXISTS email_provider_type;
-- Remove sent emails tracking table and related objects

-- Drop indexes
DROP INDEX IF EXISTS idx_sent_emails_pending;
DROP INDEX IF EXISTS idx_sent_emails_webhook_pending;
DROP INDEX IF EXISTS idx_sent_emails_retry_queue;
DROP INDEX IF EXISTS idx_sent_emails_from_address;
DROP INDEX IF EXISTS idx_sent_emails_created_at;
DROP INDEX IF EXISTS idx_sent_emails_status;
DROP INDEX IF EXISTS idx_sent_emails_message_id;
DROP INDEX IF EXISTS idx_sent_emails_domain_id;

-- Drop table
DROP TABLE IF EXISTS sent_emails;

-- Drop enum type
DROP TYPE IF EXISTS email_send_status;
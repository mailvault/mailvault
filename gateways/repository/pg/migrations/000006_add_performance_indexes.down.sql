-- Remove performance indexes

DROP INDEX IF EXISTS idx_email_addresses_local_part_domain;
DROP INDEX IF EXISTS idx_domains_domain_lookup;
DROP INDEX IF EXISTS idx_received_emails_from_address;
DROP INDEX IF EXISTS idx_received_emails_domain_user;
DROP INDEX IF EXISTS idx_received_emails_email_address_received_at;
DROP INDEX IF EXISTS idx_domains_user_verified;
DROP INDEX IF EXISTS idx_smtp_stats_domain_verified_at;
DROP INDEX IF EXISTS idx_users_created_account_type;
DROP INDEX IF EXISTS idx_domains_active_only;
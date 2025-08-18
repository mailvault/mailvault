-- Drop trigger and function
DROP TRIGGER IF EXISTS trigger_set_received_email_sequence ON received_emails;
DROP FUNCTION IF EXISTS set_received_email_sequence;

-- Drop indexes
DROP INDEX IF EXISTS unique_catch_all_per_domain;
DROP INDEX IF EXISTS idx_email_sequence;
DROP INDEX IF EXISTS idx_received_emails_received_at;
DROP INDEX IF EXISTS idx_received_emails_email_address_id;
DROP INDEX IF EXISTS idx_email_addresses_local_part;
DROP INDEX IF EXISTS idx_email_addresses_domain_id;
DROP INDEX IF EXISTS idx_domains_domain;
DROP INDEX IF EXISTS idx_domains_api_key;
DROP INDEX IF EXISTS idx_domains_user_id;
DROP INDEX IF EXISTS idx_users_auth_provider;
DROP INDEX IF EXISTS idx_users_email;

-- Drop tables in reverse order of creation
DROP TABLE IF EXISTS received_emails;
DROP TABLE IF EXISTS email_addresses;  
DROP TABLE IF EXISTS domains;
DROP TABLE IF EXISTS users;
DROP TRIGGER IF EXISTS trigger_set_received_email_sequence ON received_emails;
DROP FUNCTION IF EXISTS set_received_email_sequence();

DROP TABLE IF EXISTS received_emails;
DROP TABLE IF EXISTS email_addresses;
DROP TABLE IF EXISTS domains;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS domain_verification_status;
DROP TYPE IF EXISTS user_plan;
DROP TYPE IF EXISTS account_type;
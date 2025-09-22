-- Drop triggers and functions
DROP TRIGGER IF EXISTS trigger_set_verification_token ON domains;
DROP FUNCTION IF EXISTS set_verification_token();
DROP FUNCTION IF EXISTS generate_verification_token();

-- Drop table
DROP TABLE IF EXISTS domain_validation_records;

-- Remove check constraints
ALTER TABLE domains DROP CONSTRAINT IF EXISTS check_verification_status;

-- Drop indexes
DROP INDEX IF EXISTS idx_domains_verification_status;
DROP INDEX IF EXISTS idx_domains_next_verification_attempt;

-- Remove added columns from domains table
ALTER TABLE domains DROP COLUMN IF EXISTS verification_status;
ALTER TABLE domains DROP COLUMN IF EXISTS verification_token;
ALTER TABLE domains DROP COLUMN IF EXISTS last_verification_attempt;
ALTER TABLE domains DROP COLUMN IF EXISTS verification_error;
ALTER TABLE domains DROP COLUMN IF EXISTS verification_attempts;
ALTER TABLE domains DROP COLUMN IF EXISTS next_verification_attempt;
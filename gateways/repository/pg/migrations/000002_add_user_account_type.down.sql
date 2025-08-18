-- Remove account_type field from users table

-- Remove index
DROP INDEX IF EXISTS idx_users_account_type;

-- Remove column
ALTER TABLE users DROP COLUMN IF EXISTS account_type;

-- Drop enum type
DROP TYPE IF EXISTS account_type;
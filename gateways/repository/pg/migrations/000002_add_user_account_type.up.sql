-- Add account_type field to users table
-- Account types: freemium (default), basic, pro, pay-as-go

-- Create account type enum
CREATE TYPE account_type AS ENUM ('freemium', 'basic', 'pro', 'pay-as-go');

-- Add account_type column to users table with default 'freemium'
ALTER TABLE users ADD COLUMN account_type account_type DEFAULT 'freemium';

-- Add index for account type queries
CREATE INDEX idx_users_account_type ON users(account_type);

-- Update comments
COMMENT ON COLUMN users.account_type IS 'User account type: freemium (1 domain limit), basic, pro, or pay-as-go';
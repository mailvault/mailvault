-- Refactor user account types and add user plan field
-- New account types: user, owner, admin (for role management)
-- New user plan: free, pro, premium (for billing/feature management)

-- First, create the new enums
CREATE TYPE account_type_new AS ENUM ('user', 'owner', 'admin');
CREATE TYPE user_plan AS ENUM ('free', 'pro', 'premium');

-- Add the new user_plan column with default 'free'
ALTER TABLE users ADD COLUMN user_plan user_plan DEFAULT 'free';

-- Migrate existing account_type data to the new structure
-- Map existing types to new account types and plans
UPDATE users SET 
    user_plan = CASE 
        WHEN account_type = 'freemium' THEN 'free'::user_plan
        WHEN account_type = 'basic' THEN 'pro'::user_plan  
        WHEN account_type = 'pro' THEN 'pro'::user_plan
        WHEN account_type = 'pay-as-go' THEN 'premium'::user_plan
        ELSE 'free'::user_plan
    END;

-- Add a temporary column for the new account type
ALTER TABLE users ADD COLUMN account_type_new account_type_new;

-- Set all users to 'user' account type by default (since we're refactoring)
-- Admin accounts will need to be manually set after migration
UPDATE users SET 
    account_type_new = 'user'::account_type_new;

-- Drop the old account_type column and rename the new one
ALTER TABLE users DROP COLUMN account_type;
ALTER TABLE users RENAME COLUMN account_type_new TO account_type;

-- Drop the old enum type
DROP TYPE account_type;

-- Rename the new enum type to the original name
ALTER TYPE account_type_new RENAME TO account_type;

-- Make user_plan NOT NULL
ALTER TABLE users ALTER COLUMN user_plan SET NOT NULL;

-- Add indexes for the new columns
CREATE INDEX idx_users_user_plan ON users(user_plan);

-- Update the existing account_type index (should be recreated automatically)
-- The index idx_users_account_type already exists and will be updated

-- Update comments
COMMENT ON COLUMN users.account_type IS 'User account type for role management: user, owner, admin';
COMMENT ON COLUMN users.user_plan IS 'User plan for billing/features: free (1 domain), pro (10 domains), premium (unlimited)';
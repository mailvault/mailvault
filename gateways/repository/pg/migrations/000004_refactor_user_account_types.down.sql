-- Rollback refactor of user account types and user plan field
-- Restore original account types: freemium, basic, pro, pay-as-go, admin, super_admin

-- Create the old account_type enum
CREATE TYPE account_type_old AS ENUM ('freemium', 'basic', 'pro', 'pay-as-go', 'admin', 'super_admin');

-- Add temporary column for old account type
ALTER TABLE users ADD COLUMN account_type_old account_type_old;

-- Map new structure back to old account types
-- This is a best-effort mapping and some information may be lost
UPDATE users SET 
    account_type_old = CASE 
        WHEN account_type = 'admin' THEN 'admin'::account_type_old
        WHEN account_type = 'user' AND user_plan = 'free' THEN 'freemium'::account_type_old
        WHEN account_type = 'user' AND user_plan = 'pro' THEN 'pro'::account_type_old
        WHEN account_type = 'user' AND user_plan = 'premium' THEN 'pay-as-go'::account_type_old
        ELSE 'freemium'::account_type_old
    END;

-- Drop the new columns and indexes
DROP INDEX idx_users_user_plan;
ALTER TABLE users DROP COLUMN user_plan;
ALTER TABLE users DROP COLUMN account_type;

-- Rename the old column back
ALTER TABLE users RENAME COLUMN account_type_old TO account_type;

-- Drop the new enum types
DROP TYPE account_type;
DROP TYPE user_plan;

-- Rename the old enum type back
ALTER TYPE account_type_old RENAME TO account_type;

-- Restore the original comment
COMMENT ON COLUMN users.account_type IS 'User account type: freemium (1 domain limit), basic, pro, or pay-as-go';
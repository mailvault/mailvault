-- Remove is_catch_all field and related constraints
-- This migration removes the catch-all functionality as it's no longer needed

-- Drop the unique constraint for catch-all per domain
DROP INDEX IF EXISTS unique_catch_all_per_domain;

-- Remove the is_catch_all column from email_addresses table
ALTER TABLE email_addresses DROP COLUMN IF EXISTS is_catch_all;

-- Update table comment
COMMENT ON TABLE email_addresses IS 'Email addresses within domains (catch-all functionality removed)';
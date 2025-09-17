-- Restore is_catch_all field and related constraints
-- This rollback migration restores the catch-all functionality

-- Add back the is_catch_all column
ALTER TABLE email_addresses ADD COLUMN is_catch_all BOOLEAN DEFAULT false;

-- Recreate the unique constraint for catch-all per domain
CREATE UNIQUE INDEX unique_catch_all_per_domain ON email_addresses (domain_id)
WHERE is_catch_all = true;

-- Restore table comment
COMMENT ON TABLE email_addresses IS 'Email addresses within domains';
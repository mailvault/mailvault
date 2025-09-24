-- Rollback migration: restore the verified column
-- This restores the legacy boolean verified field for rollback purposes

-- Add back the verified column
ALTER TABLE domains ADD COLUMN verified BOOLEAN DEFAULT false NOT NULL;

-- Populate the verified column based on verification_status
-- Set verified = true where verification_status = 'verified'
UPDATE domains SET verified = (verification_status = 'verified');

-- Remove the updated comment
COMMENT ON COLUMN domains.verification_status IS 'Current verification status: pending, validating, verified, failed, expired';
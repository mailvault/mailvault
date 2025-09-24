-- Drop the legacy verified column as it has been replaced by verification_status
-- This migration completes the transition from boolean verification to enum-based verification status

-- Remove the verified column from domains table
ALTER TABLE domains DROP COLUMN verified;

-- Add comment for documentation
COMMENT ON COLUMN domains.verification_status IS 'Domain verification status using enum values: pending, validating, verified, failed, expired (replaced legacy verified boolean)';
-- Remove verification tracking columns from domains table
ALTER TABLE domains
    DROP COLUMN IF EXISTS next_verification_attempt,
    DROP COLUMN IF EXISTS verification_attempts,
    DROP COLUMN IF EXISTS verification_error,
    DROP COLUMN IF EXISTS last_verification_attempt;

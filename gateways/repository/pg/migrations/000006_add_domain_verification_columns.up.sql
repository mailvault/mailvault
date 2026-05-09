-- Add verification tracking columns to domains table
ALTER TABLE domains
    ADD COLUMN last_verification_attempt TIMESTAMPTZ,
    ADD COLUMN verification_error TEXT,
    ADD COLUMN verification_attempts INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN next_verification_attempt TIMESTAMPTZ;

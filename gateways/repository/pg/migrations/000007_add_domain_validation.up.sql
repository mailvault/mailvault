-- Add domain validation fields to domains table
ALTER TABLE domains ADD COLUMN verification_status VARCHAR(20) DEFAULT 'pending' NOT NULL;
ALTER TABLE domains ADD COLUMN verification_token VARCHAR(64);
ALTER TABLE domains ADD COLUMN last_verification_attempt TIMESTAMPTZ;
ALTER TABLE domains ADD COLUMN verification_error TEXT;
ALTER TABLE domains ADD COLUMN verification_attempts INTEGER DEFAULT 0 NOT NULL;
ALTER TABLE domains ADD COLUMN next_verification_attempt TIMESTAMPTZ;

-- Create domain_validation_records table for tracking validation history
CREATE TABLE domain_validation_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id UUID REFERENCES domains(id) ON DELETE CASCADE,
    validation_type VARCHAR(20) NOT NULL, -- 'mx_record', 'txt_record', 'ownership'
    status VARCHAR(20) NOT NULL, -- 'pending', 'success', 'failed', 'timeout'
    details JSONB, -- Store validation details like DNS records found, errors, etc.
    started_at TIMESTAMPTZ DEFAULT now(),
    completed_at TIMESTAMPTZ,
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Add indexes for performance
CREATE INDEX idx_domains_verification_status ON domains(verification_status);
CREATE INDEX idx_domains_next_verification_attempt ON domains(next_verification_attempt);
CREATE INDEX idx_domain_validation_records_domain_id ON domain_validation_records(domain_id);
CREATE INDEX idx_domain_validation_records_status ON domain_validation_records(status);
CREATE INDEX idx_domain_validation_records_created_at ON domain_validation_records(created_at);

-- Add check constraints for valid statuses
ALTER TABLE domains ADD CONSTRAINT check_verification_status
    CHECK (verification_status IN ('pending', 'validating', 'verified', 'failed', 'expired'));

ALTER TABLE domain_validation_records ADD CONSTRAINT check_validation_type
    CHECK (validation_type IN ('mx_record', 'txt_record', 'ownership', 'full_validation'));

ALTER TABLE domain_validation_records ADD CONSTRAINT check_validation_status
    CHECK (status IN ('pending', 'running', 'success', 'failed', 'timeout', 'error'));

-- Function to generate verification token
CREATE OR REPLACE FUNCTION generate_verification_token()
RETURNS TEXT AS $$
BEGIN
    -- Generate a random 32-character hex string for verification
    RETURN encode(gen_random_bytes(16), 'hex');
END;
$$ LANGUAGE plpgsql;

-- Function to set verification token when status is pending
CREATE OR REPLACE FUNCTION set_verification_token()
RETURNS TRIGGER AS $$
BEGIN
    -- Only set token if status is pending and token is null
    IF NEW.verification_status = 'pending' AND NEW.verification_token IS NULL THEN
        NEW.verification_token = generate_verification_token();
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically set verification token
CREATE TRIGGER trigger_set_verification_token
    BEFORE INSERT OR UPDATE ON domains
    FOR EACH ROW
    EXECUTE FUNCTION set_verification_token();

-- Set verification tokens for existing unverified domains
UPDATE domains
SET verification_token = generate_verification_token()
WHERE verified = false AND verification_token IS NULL;

-- Comments for documentation
COMMENT ON COLUMN domains.verification_status IS 'Current verification status: pending, validating, verified, failed, expired';
COMMENT ON COLUMN domains.verification_token IS 'Unique token for domain ownership verification via TXT record';
COMMENT ON COLUMN domains.last_verification_attempt IS 'Timestamp of the last verification attempt';
COMMENT ON COLUMN domains.verification_error IS 'Last verification error message';
COMMENT ON COLUMN domains.verification_attempts IS 'Number of verification attempts made';
COMMENT ON COLUMN domains.next_verification_attempt IS 'Scheduled time for next verification attempt';
COMMENT ON TABLE domain_validation_records IS 'History of domain validation attempts with detailed results';
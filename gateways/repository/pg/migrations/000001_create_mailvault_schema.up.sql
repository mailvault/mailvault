-- MailVault Complete Database Schema

-- Ensure required extension for gen_random_uuid
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    auth_provider VARCHAR(50) NOT NULL, -- 'supabase', 'firebase', 'basic'
    auth_provider_id VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- Domains table with webhook config and storage option
CREATE TABLE domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    domain VARCHAR(255) UNIQUE NOT NULL,
    public_key TEXT NOT NULL, -- For email encryption
    encrypted_private_key TEXT, -- Optional encrypted private key for server-side decryption (if enabled)
    api_key VARCHAR(255) UNIQUE NOT NULL, -- For API access
    verified BOOLEAN DEFAULT false,
    webhook_config JSONB, -- JSON configuration for webhook (URL, secret, headers, enabled)
    storage_enabled BOOLEAN DEFAULT true, -- Whether to store emails in database
    auto_create_address BOOLEAN DEFAULT false, -- Auto-create addresses on first receive
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- Email addresses table (simplified, no webhook URL)
CREATE TABLE email_addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id UUID REFERENCES domains(id) ON DELETE CASCADE,
    local_part VARCHAR(255) NOT NULL, -- part before @
    is_catch_all BOOLEAN DEFAULT false,
    forward_addresses TEXT[], -- Array of forward emails
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(domain_id, local_part)
);

-- Received emails table
CREATE TABLE received_emails (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email_address_id UUID REFERENCES email_addresses(id),
    from_address VARCHAR(255) NOT NULL,
    subject VARCHAR(500),
    encrypted_body TEXT NOT NULL, -- Encrypted with domain public key
    received_at TIMESTAMPTZ DEFAULT now(),
    sequence_number INTEGER -- per-address sequence number
);

-- Function and trigger to set sequence_number per email_address
CREATE OR REPLACE FUNCTION set_received_email_sequence()
RETURNS TRIGGER AS $$
BEGIN
    SELECT COALESCE(MAX(sequence_number), 0) + 1
    INTO NEW.sequence_number
    FROM received_emails
    WHERE email_address_id = NEW.email_address_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_set_received_email_sequence
    BEFORE INSERT ON received_emails
    FOR EACH ROW
    EXECUTE FUNCTION set_received_email_sequence();

-- Make sequence_number not null (after function exists)
ALTER TABLE received_emails ALTER COLUMN sequence_number SET NOT NULL;

-- Indexes for performance
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_auth_provider ON users(auth_provider, auth_provider_id);
CREATE INDEX idx_domains_user_id ON domains(user_id);
CREATE INDEX idx_domains_api_key ON domains(api_key);
CREATE INDEX idx_domains_domain ON domains(domain);
CREATE INDEX idx_email_addresses_domain_id ON email_addresses(domain_id);
CREATE INDEX idx_email_addresses_local_part ON email_addresses(local_part);
CREATE INDEX idx_received_emails_email_address_id ON received_emails(email_address_id);
CREATE INDEX idx_received_emails_received_at ON received_emails(received_at);
CREATE UNIQUE INDEX idx_email_sequence ON received_emails(email_address_id, sequence_number);

-- Constraint to ensure only one catch-all per domain
CREATE UNIQUE INDEX unique_catch_all_per_domain ON email_addresses (domain_id) 
WHERE is_catch_all = true;

-- Comments for documentation
COMMENT ON TABLE users IS 'User accounts with authentication provider information';
COMMENT ON TABLE domains IS 'User-owned domains for email routing';
COMMENT ON COLUMN domains.webhook_config IS 'JSON configuration for webhook including URL, secret, headers, and enabled status';
COMMENT ON COLUMN domains.storage_enabled IS 'Whether to store received emails in our database or just forward/webhook them';
COMMENT ON COLUMN domains.public_key IS 'Public key for encrypting emails received for this domain';
COMMENT ON COLUMN domains.encrypted_private_key IS 'Encrypted private key for domain (if stored)';
COMMENT ON COLUMN domains.auto_create_address IS 'Whether to automatically create email addresses when receiving emails to non-existent addresses';
COMMENT ON TABLE email_addresses IS 'Email addresses within domains';
COMMENT ON TABLE received_emails IS 'Stored received emails (encrypted)';
COMMENT ON COLUMN received_emails.encrypted_body IS 'Email body encrypted with domain public key';
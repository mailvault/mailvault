CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE account_type AS ENUM ('user', 'owner', 'admin');
CREATE TYPE user_plan AS ENUM ('free', 'pro', 'premium');
CREATE TYPE domain_verification_status AS ENUM ('pending', 'validating', 'verified', 'failed', 'expired');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    auth_provider VARCHAR(50) NOT NULL,
    auth_provider_id VARCHAR(255),
    account_type account_type NOT NULL DEFAULT 'user',
    user_plan user_plan NOT NULL DEFAULT 'free',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    domain VARCHAR(255) UNIQUE NOT NULL,
    public_key TEXT NOT NULL,
    api_key VARCHAR(255) UNIQUE NOT NULL,
    verification_status domain_verification_status NOT NULL DEFAULT 'pending',
    verification_token VARCHAR(255),
    verification_method VARCHAR(50),
    verification_data JSONB,
    webhook_config JSONB,
    storage_enabled BOOLEAN DEFAULT true,
    auto_create_address BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE email_addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id UUID REFERENCES domains(id) ON DELETE CASCADE,
    local_part VARCHAR(255) NOT NULL,
    forward_addresses TEXT[],
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(domain_id, local_part)
);

CREATE TABLE received_emails (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email_address_id UUID REFERENCES email_addresses(id),
    from_address VARCHAR(255) NOT NULL,
    subject VARCHAR(500),
    encrypted_body TEXT NOT NULL,
    received_at TIMESTAMPTZ DEFAULT now(),
    sequence_number INTEGER NOT NULL
);

CREATE OR REPLACE FUNCTION set_received_email_sequence()
RETURNS TRIGGER AS $$
BEGIN
    SELECT COALESCE(MAX(sequence_number), 0) + 1
    INTO NEW.sequence_number
    FROM received_emails
    WHERE email_address_id = NEW.email_address_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql VOLATILE;

CREATE TRIGGER trigger_set_received_email_sequence
    BEFORE INSERT ON received_emails
    FOR EACH ROW
    EXECUTE FUNCTION set_received_email_sequence();

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_auth_provider ON users(auth_provider, auth_provider_id);
CREATE INDEX idx_users_account_type ON users(account_type);
CREATE INDEX idx_users_user_plan ON users(user_plan);

CREATE INDEX idx_domains_user_id ON domains(user_id);
CREATE INDEX idx_domains_api_key ON domains(api_key);
CREATE INDEX idx_domains_domain ON domains(domain);
CREATE INDEX idx_domains_verification_status ON domains(verification_status);

CREATE INDEX idx_email_addresses_domain_id ON email_addresses(domain_id);
CREATE INDEX idx_email_addresses_local_part ON email_addresses(local_part);

CREATE INDEX idx_received_emails_email_address_id ON received_emails(email_address_id);
CREATE INDEX idx_received_emails_received_at ON received_emails(received_at);
CREATE UNIQUE INDEX idx_email_sequence ON received_emails(email_address_id, sequence_number);
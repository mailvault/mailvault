-- Create email provider integration system
-- This allows domains to configure multiple email providers (Resend, SendGrid, AWS SES, etc.)
-- with automatic failover and smart routing for outbound emails

-- Email provider type enum
CREATE TYPE email_provider_type AS ENUM (
    'resend',
    'sendgrid',
    'aws_ses',
    'postmark',
    'mailgun'
);

-- Email provider status enum
CREATE TYPE email_provider_status AS ENUM (
    'active',
    'inactive',
    'error',
    'suspended'
);

-- Email providers table
CREATE TABLE email_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id UUID REFERENCES domains(id) ON DELETE CASCADE,

    -- Provider configuration
    type email_provider_type NOT NULL,
    name VARCHAR(255) NOT NULL, -- User-defined name
    status email_provider_status NOT NULL DEFAULT 'active',
    priority INTEGER NOT NULL DEFAULT 0, -- Lower number = higher priority
    is_default BOOLEAN NOT NULL DEFAULT false,
    is_enabled BOOLEAN NOT NULL DEFAULT true,

    -- Provider-specific configuration (encrypted in application layer)
    config JSONB NOT NULL DEFAULT '{}',

    -- Rate limiting and retry settings
    rate_limit INTEGER NOT NULL DEFAULT 60, -- Emails per minute
    max_retries INTEGER NOT NULL DEFAULT 3,
    retry_delay INTEGER NOT NULL DEFAULT 300, -- Seconds between retries
    failover_delay INTEGER NOT NULL DEFAULT 600, -- Seconds before trying again after max failures

    -- Health monitoring
    health_status email_provider_status NOT NULL DEFAULT 'active',
    last_health_check TIMESTAMPTZ,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    last_failure TIMESTAMPTZ,
    last_success TIMESTAMPTZ,

    -- Error tracking
    last_error TEXT,
    error_count INTEGER NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ,

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),

    -- Constraints
    CONSTRAINT email_providers_priority_positive CHECK (priority >= 0),
    CONSTRAINT email_providers_rate_limit_positive CHECK (rate_limit > 0),
    CONSTRAINT email_providers_retries_non_negative CHECK (max_retries >= 0),
    CONSTRAINT email_providers_delays_non_negative CHECK (retry_delay >= 0 AND failover_delay >= 0),
    CONSTRAINT email_providers_domain_priority_unique UNIQUE (domain_id, priority),
    CONSTRAINT email_providers_domain_name_unique UNIQUE (domain_id, name)
);

-- Email provider logs table for tracking operations and performance
CREATE TABLE email_provider_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID REFERENCES email_providers(id) ON DELETE CASCADE,
    sent_email_id UUID REFERENCES sent_emails(id) ON DELETE CASCADE,

    -- Operation details
    operation VARCHAR(50) NOT NULL, -- 'send', 'health_check', 'webhook'
    status VARCHAR(20) NOT NULL, -- 'success', 'failure', 'retry'
    provider_response TEXT,
    error_message TEXT,

    -- Performance metrics
    request_duration INTEGER NOT NULL DEFAULT 0, -- Duration in milliseconds
    attempt_number INTEGER NOT NULL DEFAULT 1,

    -- Provider-specific data
    provider_data JSONB,

    -- Timestamp
    created_at TIMESTAMPTZ DEFAULT now(),

    -- Constraints
    CONSTRAINT email_provider_logs_duration_non_negative CHECK (request_duration >= 0),
    CONSTRAINT email_provider_logs_attempt_positive CHECK (attempt_number > 0)
);

-- Update sent_emails table to track provider usage
ALTER TABLE sent_emails ADD COLUMN provider_id UUID REFERENCES email_providers(id);
ALTER TABLE sent_emails ADD COLUMN provider_name VARCHAR(255);
ALTER TABLE sent_emails ADD COLUMN provider_attempt_count INTEGER DEFAULT 1;
ALTER TABLE sent_emails ADD COLUMN last_provider_error TEXT;

-- Indexes for email_providers
CREATE INDEX idx_email_providers_domain_id ON email_providers(domain_id);
CREATE INDEX idx_email_providers_domain_priority ON email_providers(domain_id, priority);
CREATE INDEX idx_email_providers_domain_default ON email_providers(domain_id, is_default) WHERE is_default = true;
CREATE INDEX idx_email_providers_type ON email_providers(type);
CREATE INDEX idx_email_providers_status ON email_providers(status);
CREATE INDEX idx_email_providers_health ON email_providers(health_status, is_enabled) WHERE is_enabled = true;
CREATE INDEX idx_email_providers_retry ON email_providers(next_retry_at, consecutive_failures)
    WHERE next_retry_at IS NOT NULL;

-- Indexes for email_provider_logs
CREATE INDEX idx_email_provider_logs_provider_id ON email_provider_logs(provider_id);
CREATE INDEX idx_email_provider_logs_sent_email_id ON email_provider_logs(sent_email_id);
CREATE INDEX idx_email_provider_logs_operation ON email_provider_logs(operation, status);
CREATE INDEX idx_email_provider_logs_created_at ON email_provider_logs(created_at);
CREATE INDEX idx_email_provider_logs_performance ON email_provider_logs(provider_id, created_at, request_duration);

-- Indexes for updated sent_emails
CREATE INDEX idx_sent_emails_provider_id ON sent_emails(provider_id);
CREATE INDEX idx_sent_emails_provider_name ON sent_emails(provider_name);

-- Unique constraint: only one default provider per domain
CREATE UNIQUE INDEX idx_email_providers_domain_default_unique
    ON email_providers(domain_id)
    WHERE is_default = true;

-- Partial indexes for performance optimization
CREATE INDEX idx_email_providers_healthy ON email_providers(domain_id, priority, is_enabled)
    WHERE status = 'active' AND health_status = 'active' AND is_enabled = true;

CREATE INDEX idx_email_provider_logs_recent ON email_provider_logs(provider_id, created_at DESC)
    WHERE created_at > now() - interval '7 days';

-- Function to automatically update updated_at timestamp
CREATE OR REPLACE FUNCTION update_email_provider_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically update updated_at
CREATE TRIGGER trigger_email_providers_updated_at
    BEFORE UPDATE ON email_providers
    FOR EACH ROW
    EXECUTE FUNCTION update_email_provider_updated_at();

-- Comments for documentation
COMMENT ON TABLE email_providers IS 'Email provider configurations for domains (Resend, SendGrid, AWS SES, etc.)';
COMMENT ON COLUMN email_providers.type IS 'Type of email provider (resend, sendgrid, aws_ses, postmark, mailgun)';
COMMENT ON COLUMN email_providers.priority IS 'Lower number = higher priority for provider selection';
COMMENT ON COLUMN email_providers.config IS 'Provider-specific configuration (API keys, etc.) - encrypted in application';
COMMENT ON COLUMN email_providers.rate_limit IS 'Maximum emails per minute for this provider';
COMMENT ON COLUMN email_providers.consecutive_failures IS 'Number of consecutive failures before marking as unhealthy';
COMMENT ON COLUMN email_providers.next_retry_at IS 'When to retry using this provider after failures';

COMMENT ON TABLE email_provider_logs IS 'Audit log of all email provider operations for monitoring and debugging';
COMMENT ON COLUMN email_provider_logs.operation IS 'Type of operation: send, health_check, webhook';
COMMENT ON COLUMN email_provider_logs.request_duration IS 'Duration of provider API call in milliseconds';
COMMENT ON COLUMN email_provider_logs.provider_data IS 'Provider-specific response data for debugging';

COMMENT ON COLUMN sent_emails.provider_id IS 'ID of email provider used to send this email';
COMMENT ON COLUMN sent_emails.provider_name IS 'Name of email provider for quick reference';
COMMENT ON COLUMN sent_emails.provider_attempt_count IS 'Number of different providers tried for this email';
COMMENT ON COLUMN sent_emails.last_provider_error IS 'Last error from any provider attempt';
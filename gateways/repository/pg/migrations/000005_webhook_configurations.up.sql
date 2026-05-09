-- Authentication types for webhook configurations
CREATE TYPE webhook_auth_type AS ENUM (
    'none',
    'basic',
    'bearer',
    'hmac_sha256'
);

-- Webhook health status
CREATE TYPE webhook_health_status AS ENUM (
    'unknown',
    'healthy',
    'degraded',
    'unhealthy'
);

-- Main webhook configurations table
CREATE TABLE webhook_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id UUID NOT NULL REFERENCES domains(id) ON DELETE CASCADE,

    -- Basic configuration
    name VARCHAR(255) NOT NULL,
    description TEXT,
    url VARCHAR(2048) NOT NULL,
    method VARCHAR(10) NOT NULL DEFAULT 'POST',
    enabled BOOLEAN NOT NULL DEFAULT true,
    verified BOOLEAN NOT NULL DEFAULT false,

    -- Authentication configuration
    auth_type webhook_auth_type NOT NULL DEFAULT 'none',
    auth_secret TEXT,
    auth_username VARCHAR(255),
    custom_headers JSONB,

    -- Event filtering
    event_types TEXT[] NOT NULL DEFAULT ARRAY['email.received']::TEXT[],

    -- Timeout and retry configuration
    timeout_seconds INTEGER NOT NULL DEFAULT 30,
    max_retries INTEGER NOT NULL DEFAULT 3,
    retry_backoff_multiplier DECIMAL(3,2) NOT NULL DEFAULT 2.0,
    initial_retry_delay_seconds INTEGER NOT NULL DEFAULT 60,

    -- Rate limiting
    rate_limit_per_minute INTEGER NOT NULL DEFAULT 60,
    rate_limit_per_hour INTEGER NOT NULL DEFAULT 1000,

    -- Circuit breaker configuration
    circuit_breaker_enabled BOOLEAN NOT NULL DEFAULT true,
    circuit_breaker_threshold INTEGER NOT NULL DEFAULT 5,
    circuit_breaker_timeout_seconds INTEGER NOT NULL DEFAULT 300,
    circuit_breaker_state VARCHAR(20) NOT NULL DEFAULT 'closed',
    circuit_breaker_opened_at TIMESTAMPTZ,

    -- Health and monitoring
    health_status webhook_health_status NOT NULL DEFAULT 'unknown',
    last_health_check_at TIMESTAMPTZ,
    last_success_at TIMESTAMPTZ,
    last_failure_at TIMESTAMPTZ,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    total_success_count BIGINT NOT NULL DEFAULT 0,
    total_failure_count BIGINT NOT NULL DEFAULT 0,
    average_response_time_ms INTEGER,

    -- Metadata
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Constraints
    CONSTRAINT webhook_configurations_valid_method CHECK (method IN ('POST', 'PUT', 'PATCH')),
    CONSTRAINT webhook_configurations_valid_url CHECK (url ~ '^https?://.*'),
    CONSTRAINT webhook_configurations_timeout_positive CHECK (timeout_seconds > 0 AND timeout_seconds <= 300),
    CONSTRAINT webhook_configurations_max_retries_valid CHECK (max_retries >= 0 AND max_retries <= 10),
    CONSTRAINT webhook_configurations_backoff_valid CHECK (retry_backoff_multiplier >= 1.0 AND retry_backoff_multiplier <= 5.0),
    CONSTRAINT webhook_configurations_rate_limit_valid CHECK (rate_limit_per_minute > 0 AND rate_limit_per_hour > 0),
    CONSTRAINT webhook_configurations_circuit_breaker_valid CHECK (circuit_breaker_threshold > 0),
    CONSTRAINT webhook_configurations_circuit_state_valid CHECK (circuit_breaker_state IN ('closed', 'open', 'half_open')),
    CONSTRAINT webhook_configurations_name_not_empty CHECK (length(trim(name)) > 0),
    UNIQUE(domain_id, name)
);

-- Webhook configuration audit log
CREATE TABLE webhook_configuration_audit (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_config_id UUID NOT NULL REFERENCES webhook_configurations(id) ON DELETE CASCADE,
    changed_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(50) NOT NULL,
    old_values JSONB,
    new_values JSONB,
    change_reason TEXT,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT webhook_config_audit_action_valid CHECK (action IN ('created', 'updated', 'deleted', 'enabled', 'disabled', 'tested'))
);

-- Webhook health check history
CREATE TABLE webhook_health_checks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_config_id UUID NOT NULL REFERENCES webhook_configurations(id) ON DELETE CASCADE,
    check_type VARCHAR(50) NOT NULL,
    status webhook_health_status NOT NULL,
    response_time_ms INTEGER,
    response_status_code INTEGER,
    error_message TEXT,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT webhook_health_checks_type_valid CHECK (check_type IN ('automatic', 'manual', 'on_failure', 'on_recovery'))
);

-- Webhook configuration templates
CREATE TABLE webhook_configuration_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT NOT NULL,
    provider_name VARCHAR(100) NOT NULL,
    default_method VARCHAR(10) NOT NULL DEFAULT 'POST',
    default_auth_type webhook_auth_type NOT NULL DEFAULT 'none',
    default_headers JSONB,
    default_timeout_seconds INTEGER NOT NULL DEFAULT 30,
    documentation_url VARCHAR(2048),
    is_active BOOLEAN NOT NULL DEFAULT true,
    usage_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_webhook_configuration_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql VOLATILE;

CREATE TRIGGER trigger_webhook_configurations_updated_at
    BEFORE UPDATE ON webhook_configurations
    FOR EACH ROW
    EXECUTE FUNCTION update_webhook_configuration_updated_at();

CREATE TRIGGER trigger_webhook_configuration_templates_updated_at
    BEFORE UPDATE ON webhook_configuration_templates
    FOR EACH ROW
    EXECUTE FUNCTION update_webhook_configuration_updated_at();

-- Indexes for webhook_configurations
CREATE INDEX idx_webhook_configurations_domain_id ON webhook_configurations(domain_id);
CREATE INDEX idx_webhook_configurations_enabled ON webhook_configurations(enabled) WHERE enabled = true;
CREATE INDEX idx_webhook_configurations_health ON webhook_configurations(health_status, last_health_check_at);
CREATE INDEX idx_webhook_configurations_circuit_breaker ON webhook_configurations(circuit_breaker_state, circuit_breaker_opened_at)
    WHERE circuit_breaker_enabled = true;
CREATE INDEX idx_webhook_configurations_domain_enabled ON webhook_configurations(domain_id, enabled) WHERE enabled = true;

-- Indexes for webhook_configuration_audit
CREATE INDEX idx_webhook_config_audit_config_id ON webhook_configuration_audit(webhook_config_id, created_at DESC);
CREATE INDEX idx_webhook_config_audit_user_id ON webhook_configuration_audit(changed_by_user_id, created_at DESC);
CREATE INDEX idx_webhook_config_audit_action ON webhook_configuration_audit(action, created_at DESC);

-- Indexes for webhook_health_checks
CREATE INDEX idx_webhook_health_checks_config_id ON webhook_health_checks(webhook_config_id, checked_at DESC);
CREATE INDEX idx_webhook_health_checks_status ON webhook_health_checks(status, checked_at DESC);

-- Insert common webhook templates
INSERT INTO webhook_configuration_templates (name, description, provider_name, default_method, default_auth_type, default_headers, documentation_url) VALUES
    ('Generic Webhook', 'Standard webhook configuration for any HTTP endpoint', 'Generic', 'POST', 'none', '{"Content-Type": "application/json"}', 'https://docs.mailvault.sh/webhooks/generic'),
    ('Slack Incoming Webhook', 'Webhook configuration for Slack incoming webhooks', 'Slack', 'POST', 'none', '{"Content-Type": "application/json"}', 'https://api.slack.com/messaging/webhooks'),
    ('Discord Webhook', 'Webhook configuration for Discord webhooks', 'Discord', 'POST', 'none', '{"Content-Type": "application/json"}', 'https://discord.com/developers/docs/resources/webhook'),
    ('Microsoft Teams', 'Webhook configuration for Microsoft Teams incoming webhooks', 'Microsoft Teams', 'POST', 'none', '{"Content-Type": "application/json"}', 'https://docs.microsoft.com/en-us/microsoftteams/platform/webhooks-and-connectors/'),
    ('Zapier', 'Webhook configuration for Zapier catch hooks', 'Zapier', 'POST', 'none', '{"Content-Type": "application/json"}', 'https://zapier.com/help/create/basics/trigger-zaps-from-webhooks'),
    ('Custom HMAC', 'Webhook with HMAC-SHA256 authentication', 'Custom', 'POST', 'hmac_sha256', '{"Content-Type": "application/json"}', 'https://docs.mailvault.sh/webhooks/hmac');

-- Migrate existing webhook_config data to webhook_configurations
INSERT INTO webhook_configurations (
    domain_id,
    name,
    description,
    url,
    method,
    enabled,
    verified,
    auth_type,
    auth_secret,
    custom_headers,
    event_types,
    created_at,
    updated_at
)
SELECT
    d.id as domain_id,
    'Default Webhook' as name,
    'Migrated from legacy webhook_config field' as description,
    d.webhook_config->>'url' as url,
    'POST' as method,
    COALESCE((d.webhook_config->>'enabled')::boolean, true) as enabled,
    false as verified,
    CASE
        WHEN d.webhook_config->>'secret' IS NOT NULL AND d.webhook_config->>'secret' != ''
        THEN 'hmac_sha256'::webhook_auth_type
        ELSE 'none'::webhook_auth_type
    END as auth_type,
    d.webhook_config->>'secret' as auth_secret,
    d.webhook_config->'headers' as custom_headers,
    ARRAY['email.received']::TEXT[] as event_types,
    d.created_at,
    d.updated_at
FROM domains d
WHERE d.webhook_config IS NOT NULL
  AND d.webhook_config != 'null'::jsonb
  AND d.webhook_config->>'url' IS NOT NULL
  AND d.webhook_config->>'url' != '';

-- Remove the old webhook_config column from domains table
ALTER TABLE domains DROP COLUMN IF EXISTS webhook_config;

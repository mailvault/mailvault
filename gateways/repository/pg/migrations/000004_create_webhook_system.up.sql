CREATE TYPE webhook_event_status AS ENUM (
    'pending',
    'sending',
    'success',
    'failure',
    'retry',
    'cancelled'
);

CREATE TYPE webhook_event_type AS ENUM (
    'email.received',
    'email.sent',
    'email.delivered',
    'email.bounced',
    'email.rejected',
    'email.unsubscribed',
    'email.complained',
    'email.opened',
    'email.clicked'
);

CREATE TABLE webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id UUID REFERENCES domains(id) ON DELETE CASCADE,
    sent_email_id UUID REFERENCES sent_emails(id) ON DELETE CASCADE,
    received_email_id UUID REFERENCES received_emails(id) ON DELETE CASCADE,
    event_type webhook_event_type NOT NULL,
    status webhook_event_status NOT NULL DEFAULT 'pending',
    recipient VARCHAR(255) NOT NULL,
    webhook_url VARCHAR(2048) NOT NULL,
    webhook_method VARCHAR(10) NOT NULL DEFAULT 'POST',
    webhook_headers JSONB,
    webhook_payload JSONB NOT NULL,
    attempt_number INTEGER NOT NULL DEFAULT 1,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    webhook_timestamp TIMESTAMPTZ NOT NULL,
    response_status_code INTEGER,
    response_headers JSONB,
    response_body TEXT,
    response_time_ms INTEGER,
    error_message TEXT,
    reason TEXT,
    description TEXT,
    provider_id UUID REFERENCES email_providers(id),
    provider_data JSONB,
    scheduled_at TIMESTAMPTZ,
    next_retry_at TIMESTAMPTZ,
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    CONSTRAINT webhook_events_attempt_positive CHECK (attempt_number > 0),
    CONSTRAINT webhook_events_max_attempts_positive CHECK (max_attempts > 0),
    CONSTRAINT webhook_events_valid_method CHECK (webhook_method IN ('POST', 'PUT', 'PATCH')),
    CONSTRAINT webhook_events_valid_url CHECK (webhook_url ~ '^https?://.*'),
    CONSTRAINT webhook_events_response_time_non_negative CHECK (response_time_ms >= 0)
);

CREATE OR REPLACE FUNCTION update_webhook_event_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql VOLATILE;

CREATE TRIGGER trigger_webhook_events_updated_at
    BEFORE UPDATE ON webhook_events
    FOR EACH ROW
    EXECUTE FUNCTION update_webhook_event_updated_at();

CREATE INDEX idx_webhook_events_domain_id ON webhook_events(domain_id);
CREATE INDEX idx_webhook_events_sent_email_id ON webhook_events(sent_email_id);
CREATE INDEX idx_webhook_events_received_email_id ON webhook_events(received_email_id);
CREATE INDEX idx_webhook_events_status ON webhook_events(status);
CREATE INDEX idx_webhook_events_event_type ON webhook_events(event_type);
CREATE INDEX idx_webhook_events_created_at ON webhook_events(created_at DESC);
CREATE INDEX idx_webhook_events_webhook_timestamp ON webhook_events(webhook_timestamp DESC);
CREATE INDEX idx_webhook_events_processed_at ON webhook_events(processed_at DESC);

CREATE INDEX idx_webhook_events_domain_status ON webhook_events(domain_id, status, created_at DESC);
CREATE INDEX idx_webhook_events_domain_type ON webhook_events(domain_id, event_type, created_at DESC);
CREATE INDEX idx_webhook_events_monitoring ON webhook_events(domain_id, status, event_type, created_at DESC);

CREATE INDEX idx_webhook_events_retry_queue ON webhook_events(status, next_retry_at)
    WHERE status = 'retry' AND next_retry_at IS NOT NULL;

CREATE INDEX idx_webhook_events_pending ON webhook_events(status, scheduled_at)
    WHERE status IN ('pending', 'retry');

CREATE INDEX idx_webhook_events_failed ON webhook_events(domain_id, created_at DESC, attempt_number)
    WHERE status = 'failure';
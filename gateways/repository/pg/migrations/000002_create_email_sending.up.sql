CREATE TYPE email_send_status AS ENUM (
    'pending',
    'queued',
    'sending',
    'sent',
    'delivered',
    'bounced',
    'failed',
    'cancelled'
);

CREATE TABLE sent_emails (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id UUID REFERENCES domains(id) ON DELETE CASCADE,
    from_address VARCHAR(255) NOT NULL,
    to_addresses TEXT[] NOT NULL,
    cc_addresses TEXT[],
    bcc_addresses TEXT[],
    subject VARCHAR(500) NOT NULL,
    text_body TEXT,
    html_body TEXT,
    message_id VARCHAR(255) UNIQUE NOT NULL,
    status email_send_status NOT NULL DEFAULT 'pending',
    error_message TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,
    created_at TIMESTAMPTZ DEFAULT now(),
    queued_at TIMESTAMPTZ,
    sent_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    next_retry_at TIMESTAMPTZ,
    smtp_response TEXT,
    smtp_message_id VARCHAR(255),
    webhook_data JSONB,
    webhook_delivered BOOLEAN DEFAULT false,
    webhook_attempts INTEGER DEFAULT 0,
    CONSTRAINT sent_emails_valid_body CHECK (text_body IS NOT NULL OR html_body IS NOT NULL),
    CONSTRAINT sent_emails_valid_recipients CHECK (array_length(to_addresses, 1) > 0)
);

CREATE INDEX idx_sent_emails_domain_id ON sent_emails(domain_id);
CREATE INDEX idx_sent_emails_message_id ON sent_emails(message_id);
CREATE INDEX idx_sent_emails_status ON sent_emails(status);
CREATE INDEX idx_sent_emails_created_at ON sent_emails(created_at);
CREATE INDEX idx_sent_emails_from_address ON sent_emails(from_address);
CREATE INDEX idx_sent_emails_retry_queue ON sent_emails(status, next_retry_at)
    WHERE status = 'failed' AND next_retry_at IS NOT NULL;
CREATE INDEX idx_sent_emails_webhook_pending ON sent_emails(webhook_delivered, webhook_attempts, created_at)
    WHERE webhook_delivered = false;
CREATE INDEX idx_sent_emails_pending ON sent_emails(created_at, status)
    WHERE status IN ('pending', 'queued', 'sending');
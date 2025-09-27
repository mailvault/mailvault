-- Add sent emails tracking table
-- This table stores all emails sent via the API, similar to how Resend tracks sent emails

-- Email sending status enum
CREATE TYPE email_send_status AS ENUM (
    'pending',      -- Email queued but not yet processed
    'queued',       -- Email in worker queue for sending
    'sending',      -- Email currently being sent
    'sent',         -- Email successfully sent to SMTP server
    'delivered',    -- Email successfully delivered (if tracking available)
    'bounced',      -- Email bounced back
    'failed',       -- Email sending failed
    'cancelled'     -- Email sending cancelled
);

-- Sent emails table
CREATE TABLE sent_emails (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id UUID REFERENCES domains(id) ON DELETE CASCADE,

    -- Email addressing
    from_address VARCHAR(255) NOT NULL,
    to_addresses TEXT[] NOT NULL, -- Array of recipient emails
    cc_addresses TEXT[], -- Array of CC emails
    bcc_addresses TEXT[], -- Array of BCC emails

    -- Email content
    subject VARCHAR(500) NOT NULL,
    text_body TEXT,
    html_body TEXT,

    -- Tracking and metadata
    message_id VARCHAR(255) UNIQUE NOT NULL, -- External message ID (mv_timestamp_random format)
    status email_send_status NOT NULL DEFAULT 'pending',

    -- Error handling and retries
    error_message TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT now(),
    queued_at TIMESTAMPTZ,
    sent_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    next_retry_at TIMESTAMPTZ,

    -- SMTP delivery details
    smtp_response TEXT, -- Response from SMTP server
    smtp_message_id VARCHAR(255), -- Message ID returned by SMTP server

    -- Webhook and event data
    webhook_data JSONB, -- Data to send in webhook events
    webhook_delivered BOOLEAN DEFAULT false,
    webhook_attempts INTEGER DEFAULT 0,

    -- Constraints
    CONSTRAINT sent_emails_valid_body CHECK (text_body IS NOT NULL OR html_body IS NOT NULL),
    CONSTRAINT sent_emails_valid_recipients CHECK (array_length(to_addresses, 1) > 0)
);

-- Indexes for performance
CREATE INDEX idx_sent_emails_domain_id ON sent_emails(domain_id);
CREATE INDEX idx_sent_emails_message_id ON sent_emails(message_id);
CREATE INDEX idx_sent_emails_status ON sent_emails(status);
CREATE INDEX idx_sent_emails_created_at ON sent_emails(created_at);
CREATE INDEX idx_sent_emails_from_address ON sent_emails(from_address);
CREATE INDEX idx_sent_emails_retry_queue ON sent_emails(status, next_retry_at)
    WHERE status = 'failed' AND next_retry_at IS NOT NULL;

-- Index for webhook processing
CREATE INDEX idx_sent_emails_webhook_pending ON sent_emails(webhook_delivered, webhook_attempts, created_at)
    WHERE webhook_delivered = false;

-- Partial index for active/pending emails that need processing
CREATE INDEX idx_sent_emails_pending ON sent_emails(created_at, status)
    WHERE status IN ('pending', 'queued', 'sending');

-- Comments for documentation
COMMENT ON TABLE sent_emails IS 'Outbound emails sent via API with full tracking and delivery status';
COMMENT ON COLUMN sent_emails.message_id IS 'External message ID returned to API clients (mv_timestamp_random format)';
COMMENT ON COLUMN sent_emails.status IS 'Current status of email delivery pipeline';
COMMENT ON COLUMN sent_emails.webhook_data IS 'JSON data to include in webhook events for this email';
COMMENT ON COLUMN sent_emails.smtp_message_id IS 'Message ID returned by SMTP server after successful send';
COMMENT ON COLUMN sent_emails.retry_count IS 'Number of delivery attempts made for this email';
COMMENT ON COLUMN sent_emails.next_retry_at IS 'When to retry sending this email if it failed';
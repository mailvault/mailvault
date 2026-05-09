-- smtp_verification_stats records SPF/DKIM/DMARC + content + reputation
-- verification results captured by the SMTP daemon. Used for the inbox
-- security panel and admin reporting.
CREATE TABLE IF NOT EXISTS smtp_verification_stats (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id             UUID NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    email_address_id      UUID NOT NULL REFERENCES email_addresses(id) ON DELETE CASCADE,
    verified_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    sender_ip             INET,
    sender_domain         TEXT NOT NULL DEFAULT '',
    from_address          TEXT NOT NULL DEFAULT '',

    spf_result            TEXT NOT NULL DEFAULT '',
    spf_mechanism         TEXT NOT NULL DEFAULT '',

    dkim_valid            BOOLEAN NOT NULL DEFAULT false,
    dkim_domain           TEXT NOT NULL DEFAULT '',
    dkim_selector         TEXT NOT NULL DEFAULT '',

    dmarc_result          TEXT NOT NULL DEFAULT '',
    dmarc_policy          TEXT NOT NULL DEFAULT '',
    dmarc_alignment_spf   BOOLEAN NOT NULL DEFAULT false,
    dmarc_alignment_dkim  BOOLEAN NOT NULL DEFAULT false,

    spam_score            DOUBLE PRECISION NOT NULL DEFAULT 0,
    content_verdict       TEXT NOT NULL DEFAULT '',

    reputation_score      DOUBLE PRECISION NOT NULL DEFAULT 0,
    is_blacklisted        BOOLEAN NOT NULL DEFAULT false,

    final_action          TEXT NOT NULL DEFAULT '',
    is_quarantined        BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_smtp_verification_stats_domain_id      ON smtp_verification_stats(domain_id);
CREATE INDEX IF NOT EXISTS idx_smtp_verification_stats_email_addr     ON smtp_verification_stats(email_address_id);
CREATE INDEX IF NOT EXISTS idx_smtp_verification_stats_from_address   ON smtp_verification_stats(from_address);
CREATE INDEX IF NOT EXISTS idx_smtp_verification_stats_verified_at    ON smtp_verification_stats(verified_at DESC);
CREATE INDEX IF NOT EXISTS idx_smtp_verification_stats_is_quarantined ON smtp_verification_stats(is_quarantined) WHERE is_quarantined = true;

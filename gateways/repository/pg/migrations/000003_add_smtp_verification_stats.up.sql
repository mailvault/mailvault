-- SMTP verification statistics table
CREATE TABLE smtp_verification_stats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id UUID REFERENCES domains(id) ON DELETE CASCADE,
    email_address_id UUID REFERENCES email_addresses(id) ON DELETE CASCADE,
    verified_at TIMESTAMPTZ DEFAULT now(),
    
    -- Sender information
    sender_ip INET,
    sender_domain VARCHAR(255),
    from_address VARCHAR(255),
    
    -- SPF results
    spf_result VARCHAR(20), -- pass, fail, softfail, neutral, none, temperror, permerror
    spf_mechanism VARCHAR(100),
    
    -- DKIM results
    dkim_valid BOOLEAN DEFAULT false,
    dkim_domain VARCHAR(255),
    dkim_selector VARCHAR(100),
    
    -- DMARC results
    dmarc_result VARCHAR(20), -- pass, fail, none
    dmarc_policy VARCHAR(20), -- none, quarantine, reject
    dmarc_alignment_spf BOOLEAN DEFAULT false,
    dmarc_alignment_dkim BOOLEAN DEFAULT false,
    
    -- Content analysis
    spam_score DECIMAL(4,2),
    content_verdict VARCHAR(20), -- clean, suspicious, spam
    
    -- Reputation
    reputation_score DECIMAL(4,2),
    is_blacklisted BOOLEAN DEFAULT false,
    
    -- Final action
    final_action VARCHAR(20), -- accept, quarantine, reject, tempfail
    is_quarantined BOOLEAN DEFAULT false
);

-- Indexes for performance
CREATE INDEX idx_smtp_stats_domain_id ON smtp_verification_stats(domain_id);
CREATE INDEX idx_smtp_stats_email_address_id ON smtp_verification_stats(email_address_id);
CREATE INDEX idx_smtp_stats_verified_at ON smtp_verification_stats(verified_at);
CREATE INDEX idx_smtp_stats_sender_ip ON smtp_verification_stats(sender_ip);
CREATE INDEX idx_smtp_stats_final_action ON smtp_verification_stats(final_action);
CREATE INDEX idx_smtp_stats_spf_result ON smtp_verification_stats(spf_result);
CREATE INDEX idx_smtp_stats_dkim_valid ON smtp_verification_stats(dkim_valid);
CREATE INDEX idx_smtp_stats_dmarc_result ON smtp_verification_stats(dmarc_result);

-- Comments for documentation
COMMENT ON TABLE smtp_verification_stats IS 'Statistics for SMTP email verification results';
COMMENT ON COLUMN smtp_verification_stats.sender_ip IS 'IP address of the sender';
COMMENT ON COLUMN smtp_verification_stats.spf_result IS 'SPF verification result';
COMMENT ON COLUMN smtp_verification_stats.spf_mechanism IS 'SPF mechanism that matched';
COMMENT ON COLUMN smtp_verification_stats.dkim_valid IS 'Whether DKIM signature was valid';
COMMENT ON COLUMN smtp_verification_stats.dmarc_result IS 'DMARC policy evaluation result';
COMMENT ON COLUMN smtp_verification_stats.dmarc_policy IS 'DMARC policy from DNS record';
COMMENT ON COLUMN smtp_verification_stats.spam_score IS 'Content analysis spam score (0-1)';
COMMENT ON COLUMN smtp_verification_stats.content_verdict IS 'Content analysis verdict';
COMMENT ON COLUMN smtp_verification_stats.reputation_score IS 'Sender reputation score (0-1)';
COMMENT ON COLUMN smtp_verification_stats.final_action IS 'Final action taken on the email';
COMMENT ON COLUMN smtp_verification_stats.is_quarantined IS 'Whether the email was quarantined';
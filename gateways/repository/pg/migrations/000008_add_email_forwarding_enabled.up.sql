ALTER TABLE email_addresses ADD COLUMN IF NOT EXISTS forwarding_enabled BOOLEAN NOT NULL DEFAULT false;

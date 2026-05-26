-- local_credentials stores password hashes for users authenticated by the
-- built-in local auth provider. Each row mirrors exactly one users.id; rows
-- only exist for users whose users.auth_provider = 'local'.
CREATE TABLE IF NOT EXISTS local_credentials (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    password_hash TEXT NOT NULL,
    email_confirmed BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

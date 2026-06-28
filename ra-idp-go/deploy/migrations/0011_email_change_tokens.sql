CREATE TABLE IF NOT EXISTS email_change_tokens (
    token_hash TEXT PRIMARY KEY,
    sub TEXT NOT NULL REFERENCES users(sub) ON DELETE CASCADE,
    new_email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS email_change_tokens_sub_idx
    ON email_change_tokens (sub);

CREATE INDEX IF NOT EXISTS email_change_tokens_expires_at_idx
    ON email_change_tokens (expires_at);

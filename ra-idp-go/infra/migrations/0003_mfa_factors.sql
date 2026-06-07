CREATE TABLE IF NOT EXISTS mfa_factors (
    sub TEXT NOT NULL REFERENCES users(sub) ON DELETE CASCADE,
    type TEXT NOT NULL,
    secret TEXT,
    label TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ,
    PRIMARY KEY (sub, type)
);

-- =================================================================
-- 0005 — password_reset_tokens (ADR-030 forgot-password)
-- =================================================================
--
-- spec/scl.yaml model PasswordResetTokenRecord の永続化境界。
-- token は base64url の 32 バイト乱数、保存時は SHA-256 hash のみ。
-- 単発消費は consume() の DELETE ... RETURNING で原子的に行う。
-- 同 sub の未消費 token は新規 INSERT 時に削除し、最後のリクエストだけを生かす。

BEGIN;

CREATE TABLE password_reset_tokens (
    token_hash   TEXT PRIMARY KEY,
    sub          TEXT NOT NULL REFERENCES users(sub) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL
);

CREATE INDEX password_reset_tokens_sub_idx
    ON password_reset_tokens (sub);
CREATE INDEX password_reset_tokens_expires_at_idx
    ON password_reset_tokens (expires_at);

COMMENT ON TABLE password_reset_tokens IS
    'ADR-030 forgot-password — single-use reset tokens (stored as SHA-256 hashes)';

COMMIT;

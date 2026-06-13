-- =================================================================
-- 0006 — user administration fields (ADR-031)
-- =================================================================

BEGIN;

ALTER TABLE users
    ADD COLUMN roles JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE users
    ADD COLUMN disabled_at TIMESTAMPTZ;

COMMENT ON COLUMN users.roles IS
    'ADR-031 RBAC role names. admin grants access to /admin/*.';
COMMENT ON COLUMN users.disabled_at IS
    'ADR-031 reversible account disablement timestamp.';

COMMIT;

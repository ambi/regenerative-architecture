-- wi-19 / ADR-039: User をコア + lifecycle(JSONB) + attributes(JSONB) に再構成する。
-- 旧 disabled_at / deleted_at は lifecycle.status に統合する ("いつ" は監査イベントが持つ)。

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS lifecycle JSONB NOT NULL DEFAULT jsonb_build_object('status', 'active');
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS attributes JSONB NOT NULL DEFAULT '{}'::jsonb;

-- 既存行の状態を status へ移送 (deleted を disabled より優先)。
UPDATE users
SET lifecycle = jsonb_build_object(
        'status', CASE
            WHEN deleted_at IS NOT NULL THEN 'deleted'
            WHEN disabled_at IS NOT NULL THEN 'disabled'
            ELSE 'active'
        END)
        || CASE
            WHEN deleted_at IS NOT NULL THEN jsonb_build_object('status_changed_at', deleted_at)
            WHEN disabled_at IS NOT NULL THEN jsonb_build_object('status_changed_at', disabled_at)
            ELSE '{}'::jsonb
        END
WHERE disabled_at IS NOT NULL OR deleted_at IS NOT NULL;

-- preferred_username の一意性は tombstone(status=deleted) を除外して維持する。
DROP INDEX IF EXISTS users_preferred_username_active_idx;
CREATE UNIQUE INDEX IF NOT EXISTS users_preferred_username_active_idx
    ON users (tenant_id, preferred_username)
    WHERE lifecycle->>'status' <> 'deleted';

ALTER TABLE users DROP COLUMN IF EXISTS disabled_at;
ALTER TABLE users DROP COLUMN IF EXISTS deleted_at;

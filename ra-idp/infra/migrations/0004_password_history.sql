-- =================================================================
-- 0004 — password_history (ADR-027 reuse prevention)
-- =================================================================
--
-- spec/scl.yaml invariant PasswordHistoryNoReuse の永続化境界。
-- 直近 history_depth 件のパスワードと一致した new password を拒否する
-- ために、過去の PHC encoded を時系列で保持する。
--
-- 列は password_hash と同じ PHC 文字列 (Argon2id) を持つため、追加の
-- 暗号化は行わない (現行 users.password_hash と同等の攻撃耐性)。
-- ユーザー削除時 (ON DELETE CASCADE) に履歴も消去する: 残存させると
-- ハッシュとはいえ無効な永続化となり GDPR の最小化原則と衝突する。

BEGIN;

CREATE TABLE password_history (
    id           BIGSERIAL PRIMARY KEY,
    sub          TEXT NOT NULL REFERENCES users(sub) ON DELETE CASCADE,
    encoded      TEXT NOT NULL,
        -- x-sensitive: true (argon2id PHC string)
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 直近 N 件取得 (sub + created_at DESC) を効率化
CREATE INDEX password_history_sub_created_at_idx
    ON password_history (sub, created_at DESC);

COMMENT ON TABLE password_history IS
    'ADR-027 password reuse prevention — keeps last N hashes per sub';

COMMIT;

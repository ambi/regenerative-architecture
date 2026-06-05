-- Migration 0001 — Initial schema
-- Layer 1: Specification Core (data model authority)
--
-- このファイルが Postgres における durable state のテーブル定義の唯一の権威。
-- JSON Schema (../client.schema.json 等) のフィールドと 1 対 1 対応する。
-- CI (infra/scripts/check-spec-coherence.ts) で機械検証する。
--
-- PII 注釈は -- x-pii / -- x-retention-days / -- x-purge-on-deletion で表現する。
-- これは ../user.schema.json 等の x-* 注釈と一致させる。
--
-- 採用判断は ADR-016 (永続化アダプタ選定) を参照。

BEGIN;

-- =================================================================
-- マイグレーション管理テーブル
-- =================================================================
CREATE TABLE IF NOT EXISTS schema_migrations (
    version     TEXT PRIMARY KEY,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    checksum    TEXT NOT NULL
);

-- =================================================================
-- clients — OAuth2 / OIDC クライアント (RFC 7591)
-- =================================================================
-- spec: client.schema.json
-- retention: 2555 days (x-retention-days, audit required)
CREATE TABLE clients (
    client_id                              TEXT PRIMARY KEY,
    client_secret_hash                     TEXT,
        -- x-sensitive: true (argon2id hash; never plaintext)
    client_name                            TEXT,
    client_type                            TEXT NOT NULL CHECK (client_type IN ('public', 'confidential')),
    redirect_uris                          JSONB NOT NULL,
    grant_types                            JSONB NOT NULL,
    response_types                         JSONB NOT NULL DEFAULT '[]'::jsonb,
    token_endpoint_auth_method             TEXT NOT NULL CHECK (token_endpoint_auth_method IN (
        'client_secret_basic',
        'client_secret_post',
        'private_key_jwt',
        'tls_client_auth',
        'none'
    )),
    scope                                  TEXT NOT NULL,
    jwks_uri                               TEXT,
    jwks                                   JSONB,
    tls_client_auth_subject_dn             TEXT,
    id_token_signed_response_alg           TEXT NOT NULL DEFAULT 'PS256' CHECK (id_token_signed_response_alg IN ('PS256', 'ES256')),
    require_pushed_authorization_requests  BOOLEAN NOT NULL DEFAULT FALSE,
    dpop_bound_access_tokens               BOOLEAN NOT NULL DEFAULT FALSE,
    fapi_profile                           TEXT NOT NULL DEFAULT 'none' CHECK (fapi_profile IN ('none', 'fapi_2_security_profile')),
    created_at                             TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE clients IS 'spec/client.schema.json — retention 7y, audit-required';

-- =================================================================
-- users — リソースオーナー (OIDC Core §5.1)
-- =================================================================
-- spec: user.schema.json
-- retention: 2555 days, but PII columns are purged 30 days after deleted_at
CREATE TABLE users (
    sub                  TEXT PRIMARY KEY,
        -- x-pii: false (仮名化された識別子、削除後も監査ログに残る)
    preferred_username   TEXT NOT NULL,
        -- x-pii: true
        -- x-retention-days: 2555
    password_hash        TEXT NOT NULL,
        -- x-sensitive: true (argon2id)
        -- x-purge-on-deletion: true
    name                 TEXT,
        -- x-pii: true
        -- x-purge-on-deletion: true
        -- x-purge-deadline-days: 30
    given_name           TEXT,
        -- x-pii: true
        -- x-purge-on-deletion: true
        -- x-purge-deadline-days: 30
    family_name          TEXT,
        -- x-pii: true
        -- x-purge-on-deletion: true
        -- x-purge-deadline-days: 30
    email                TEXT,
        -- x-pii: true
        -- x-purge-on-deletion: true
        -- x-purge-deadline-days: 30
    email_verified       BOOLEAN NOT NULL DEFAULT FALSE,
    mfa_enrolled         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at           TIMESTAMPTZ
        -- ソフトデリート時刻。deleted_at + 30 日で PII を物理消去する。
);
CREATE UNIQUE INDEX users_preferred_username_active_idx
    ON users (preferred_username)
    WHERE deleted_at IS NULL;
CREATE INDEX users_deleted_at_idx ON users (deleted_at)
    WHERE deleted_at IS NOT NULL;
COMMENT ON TABLE users IS 'spec/user.schema.json — PII purge 30d after deleted_at';

-- =================================================================
-- consents — RFC 6749 §1.3 ユーザー同意
-- =================================================================
-- spec: consent.schema.json
-- retention: 2555 days (slo.yaml consent_records_days)
CREATE TABLE consents (
    sub          TEXT NOT NULL REFERENCES users(sub) ON DELETE RESTRICT,
    client_id    TEXT NOT NULL REFERENCES clients(client_id) ON DELETE RESTRICT,
    scopes       JSONB NOT NULL,
    granted_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ,
    PRIMARY KEY (sub, client_id)
);
CREATE INDEX consents_sub_idx       ON consents (sub);
CREATE INDEX consents_client_id_idx ON consents (client_id);
COMMENT ON TABLE consents IS 'spec/consent.schema.json — retention 7y';

-- =================================================================
-- refresh_tokens — ADR-004 family-based rotation & revocation
-- =================================================================
-- spec: tokens/refresh-token.schema.json
CREATE TABLE refresh_tokens (
    id                    UUID PRIMARY KEY,
    hash                  TEXT NOT NULL UNIQUE,
        -- SHA-256 ハッシュのみ保存。プレーンテキスト不可。
    family_id             UUID NOT NULL,
        -- 認可コード由来のチェーン全体で共通。リプレイ検出時にこの単位で失効。
    parent_id             UUID REFERENCES refresh_tokens(id) ON DELETE NO ACTION,
    client_id             TEXT NOT NULL REFERENCES clients(client_id) ON DELETE RESTRICT,
    sub                   TEXT NOT NULL,
        -- users(sub) への FK は意図的に張らない:
        -- ユーザー削除後も監査トレース可能性のため (sub は仮名化されている)
    scopes                JSONB NOT NULL,
    issued_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at            TIMESTAMPTZ NOT NULL,
        -- slo.yaml refresh_token_ttl_seconds = 14 days
    absolute_expires_at   TIMESTAMPTZ NOT NULL,
        -- slo.yaml refresh_token_absolute_ttl_seconds = 30 days
        -- ローテーション越境不可
    revoked               BOOLEAN NOT NULL DEFAULT FALSE,
    rotated               BOOLEAN NOT NULL DEFAULT FALSE,
    sender_constraint     JSONB
        -- null | { type: 'dpop' | 'mtls', jkt?: ..., 'x5t#S256'?: ... }
);
CREATE INDEX refresh_tokens_family_id_idx     ON refresh_tokens (family_id);
CREATE INDEX refresh_tokens_client_id_idx     ON refresh_tokens (client_id);
CREATE INDEX refresh_tokens_sub_idx           ON refresh_tokens (sub);
CREATE INDEX refresh_tokens_expires_at_idx    ON refresh_tokens (expires_at)
    WHERE revoked = FALSE AND rotated = FALSE;
COMMENT ON TABLE refresh_tokens IS 'ADR-004 family-based rotation';

-- =================================================================
-- signing_keys — ADR-009 90-day rotation, 7-day overlap
-- =================================================================
-- spec: tokens/access-token.schema.json (kid header reference)
-- retention: 2555 days (slo.yaml signing_key_archive_days)
CREATE TABLE signing_keys (
    kid              TEXT PRIMARY KEY,
    alg              TEXT NOT NULL CHECK (alg IN ('PS256', 'ES256')),
    public_jwk       JSONB NOT NULL,
    private_jwk      JSONB NOT NULL,
        -- x-sensitive: true (KMS / HSM 採用時はここを参照 ID に置換)
    active           BOOLEAN NOT NULL DEFAULT TRUE,
        -- 同時にひとつだけ active = TRUE
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    rotated_at       TIMESTAMPTZ,
        -- 新鍵に切り替えた時刻 (active → inactive のタイミング)
    archived_at      TIMESTAMPTZ
        -- JWKS から外した時刻。これ以降は監査検証用途のみ。
);
CREATE INDEX signing_keys_active_idx ON signing_keys (active) WHERE active = TRUE;
COMMENT ON TABLE signing_keys IS 'ADR-009 — archive 7y for audit token verification';

-- =================================================================
-- audit_log — 不変 append-only ログ
-- =================================================================
-- spec: events.schema.json (oneOf)
-- retention: 2555 days (slo.yaml audit_log_days)
-- このテーブルは partition の追加・削除のみ許可。UPDATE / DELETE は禁止。
CREATE TABLE audit_log (
    id            BIGSERIAL PRIMARY KEY,
    event_type    TEXT NOT NULL,
        -- events.schema.json の oneOf の type 値
    occurred_at   TIMESTAMPTZ NOT NULL,
    actor_ip      INET,
        -- x-pii: true (アクセスログとして取り扱う)
    actor_ua      TEXT,
    payload       JSONB NOT NULL
        -- events.schema.json の各イベント型を JSON Schema で検証
);
CREATE INDEX audit_log_event_type_idx ON audit_log (event_type, occurred_at);
CREATE INDEX audit_log_occurred_at_idx ON audit_log (occurred_at);
COMMENT ON TABLE audit_log IS 'spec/events.schema.json — append-only, 7y retention, SIEM source';

-- 監査ログへの UPDATE / DELETE を物理的に禁止 (compliance hard guard)
CREATE OR REPLACE FUNCTION audit_log_immutable() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is append-only (ADR-016, slo.yaml audit_log_days)';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_log_no_update
    BEFORE UPDATE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_immutable();

CREATE TRIGGER audit_log_no_delete
    BEFORE DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_immutable();

-- =================================================================
-- outbox — domain events → Kafka two-stage delivery (ADR-016)
-- =================================================================
-- transactional outbox pattern: usecase が DB トランザクション内で INSERT し、
-- 別プロセス (event-relay) が tail して Kafka に publish する。
-- ペイロードは spec/scl.yaml models.DomainEvent (派生: gen/DomainEvent.json) で固定される契約スキーマ。
CREATE TABLE outbox (
    id              BIGSERIAL PRIMARY KEY,
    event_type      TEXT NOT NULL,
    topic           TEXT NOT NULL,
        -- infra/event-routing.yaml の event_to_topic から派生
    payload         JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at    TIMESTAMPTZ,
        -- relay が Kafka publish 成功後に書き込む
    published_to    TEXT,
        -- 'kafka' | 'console' | 'kafka,siem' etc
    attempts        INT NOT NULL DEFAULT 0,
    last_error      TEXT
);
CREATE INDEX outbox_unpublished_idx ON outbox (id)
    WHERE published_at IS NULL;
COMMENT ON TABLE outbox IS 'ADR-016 transactional outbox — relay tails and publishes';

COMMIT;

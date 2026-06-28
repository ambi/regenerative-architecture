-- ADR-018 / ADR-041 / wi-44: 監査イベント読み出しモデルと認証イベント bucket 集約。
--
-- audit_events は admin の時系列調査 (ListAdminAuditEvents / 認証イベント検索) 用の
-- 読み出しモデル。SIEM streaming 用の outbox とは別系統に保つ (ADR-018: 監査 /
-- アプリログ分離)。tenant_id は payload に tenantId を持たないイベントでは '' を許す
-- ため tenants への FK は張らない。
CREATE TABLE IF NOT EXISTS audit_events (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL DEFAULT '',
    type        TEXT NOT NULL,
    sub         TEXT,
    occurred_at TIMESTAMPTZ NOT NULL,
    payload     JSONB NOT NULL DEFAULT '{}'::jsonb
);

-- admin 検索は limit (既定 100 / 上限 1000) と任意の type/category/sub/期間
-- フィルタで bounded に読み出す (ADR-045)。(tenant_id, occurred_at desc) の index で
-- 時系列降順スキャンを当てる。
CREATE INDEX IF NOT EXISTS audit_events_tenant_occurred_idx
    ON audit_events (tenant_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS audit_events_type_idx ON audit_events (type);
CREATE INDEX IF NOT EXISTS audit_events_sub_idx ON audit_events (sub) WHERE sub IS NOT NULL;

-- ADR-041: 攻撃 (クレデンシャル試行洪水) 時に個別行を出さず、(tenant_id, kind,
-- key_hash, 5 分窓) 単位の 1 行へ畳み込む計数。key_hash は tenant salt 付き SHA-256 で
-- 平文を監査に流さない (ADR-046)。
CREATE TABLE IF NOT EXISTS authentication_event_buckets (
    tenant_id    TEXT NOT NULL,
    kind         TEXT NOT NULL,
    key_hash     TEXT NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    count        BIGINT NOT NULL DEFAULT 0,
    first_seen   TIMESTAMPTZ NOT NULL,
    last_seen    TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, kind, key_hash, window_start)
);

CREATE INDEX IF NOT EXISTS authentication_event_buckets_window_idx
    ON authentication_event_buckets (tenant_id, window_start DESC);

-- ADR-050: tenant-registered Rich Authorization Requests (RFC 9396) の type 定義。
-- 受理する authorization_details の type ごとに、検証スキーマ (フィールド半順序) と
-- 同意 UI の表示テンプレートを保持する。Enabled な type だけが新規要求で受理される。

CREATE TABLE IF NOT EXISTS authorization_detail_types (
    tenant_id        TEXT NOT NULL,
    type             TEXT NOT NULL,
    description      TEXT,
    schema           JSONB NOT NULL DEFAULT '{"rules":[]}'::jsonb,
    display_template TEXT NOT NULL,
    state            TEXT NOT NULL DEFAULT 'Enabled'
        CHECK (state IN ('Enabled', 'Disabled')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, type),
    CONSTRAINT authorization_detail_types_tenant_id_fkey
        FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT
);

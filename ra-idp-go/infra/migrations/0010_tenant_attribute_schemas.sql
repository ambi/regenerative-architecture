-- wi-19 / ADR-040: tenant ごとの custom 属性定義 (独立 aggregate)。
-- 組み込み属性 (BuiltinAttributeDefs) はコードが持ち、本テーブルは tenant 固有分のみ。
CREATE TABLE IF NOT EXISTS tenant_attribute_schemas (
    tenant_id TEXT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    attributes JSONB NOT NULL DEFAULT '[]'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

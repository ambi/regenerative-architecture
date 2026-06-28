-- wi-70 / ADR-069: 利用者ポータルのアプリ分類。
-- ApplicationCategory は管理者が tenant 単位で定義するセクションで、Application に 0..N 個
-- 付与する。付与は applications.category_ids (UUID 文字列の配列) で持ち、カテゴリ削除時は
-- アプリ側の配列からも除く。すべてテナント境界に閉じる。

CREATE TABLE IF NOT EXISTS application_categories (
    tenant_id   TEXT        NOT NULL DEFAULT 'default',
    category_id UUID        NOT NULL,
    name        TEXT        NOT NULL,
    position    INTEGER     NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, category_id)
);

ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS category_ids TEXT[] NOT NULL DEFAULT '{}';

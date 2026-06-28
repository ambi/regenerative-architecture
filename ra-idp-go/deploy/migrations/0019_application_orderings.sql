-- wi-70 / ADR-069: 利用者ポータルの手動並び順。
-- tenant_id + user_sub をキーに、Application の表示順を application_id の順序列で持つ。
-- 表示設定であり ON DELETE CASCADE は持たない (未割当 application は一覧解決時に除外する)。
-- すべてテナント境界に閉じる。

CREATE TABLE IF NOT EXISTS application_orderings (
    tenant_id       TEXT        NOT NULL DEFAULT 'default',
    user_sub        TEXT        NOT NULL,
    application_ids  TEXT[]      NOT NULL DEFAULT '{}',
    updated_at      TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, user_sub)
);

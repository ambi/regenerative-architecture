-- wi-69 / ADR-064: ApplicationCatalog の上位 aggregate と割当。
-- Application は OIDC client / SAML SP / WS-Fed RP を protocol binding として束ね、
-- 割当はポータル可視性とフェデレーション利用可否を fail-closed で制御する。
-- すべてテナント境界に閉じる。割当は application への ON DELETE CASCADE FK を持つ。

CREATE TABLE IF NOT EXISTS applications (
    tenant_id      TEXT        NOT NULL DEFAULT 'default',
    application_id UUID        NOT NULL,
    name           TEXT        NOT NULL,
    kind           TEXT        NOT NULL,
    status         TEXT        NOT NULL,
    icon_url       TEXT        NOT NULL DEFAULT '',
    launch_url     TEXT        NOT NULL DEFAULT '',
    bindings       JSONB       NOT NULL DEFAULT '[]'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, application_id)
);

CREATE TABLE IF NOT EXISTS application_assignments (
    tenant_id      TEXT        NOT NULL DEFAULT 'default',
    application_id UUID        NOT NULL,
    subject_type   TEXT        NOT NULL,
    subject_id     TEXT        NOT NULL,
    visibility     TEXT        NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, application_id, subject_type, subject_id),
    FOREIGN KEY (tenant_id, application_id)
        REFERENCES applications (tenant_id, application_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS application_assignments_subject_idx
    ON application_assignments (tenant_id, subject_type, subject_id);

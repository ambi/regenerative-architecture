-- wi-83: SAML SP / WS-Fed RP trust registration の PostgreSQL 永続化。
-- Federation trust は URI を識別子に持つため TEXT primary key とし、claim policy /
-- protocol-specific profile は JSONB に閉じ込める。すべて tenant scope に閉じる。

CREATE TABLE IF NOT EXISTS saml_service_providers (
    tenant_id      TEXT        NOT NULL DEFAULT 'default',
    entity_id      TEXT        NOT NULL,
    display_name   TEXT        NOT NULL DEFAULT '',
    acs_urls       JSONB       NOT NULL DEFAULT '[]'::jsonb,
    slo_url        TEXT        NOT NULL DEFAULT '',
    audience       TEXT        NOT NULL DEFAULT '',
    claim_policy   JSONB       NOT NULL,
    sign_assertion BOOLEAN     NOT NULL DEFAULT TRUE,
    sign_response  BOOLEAN     NOT NULL DEFAULT FALSE,
    want_authn_requests_signed BOOLEAN NOT NULL DEFAULT FALSE,
    authn_request_signing_certificate_pem TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ,
    PRIMARY KEY (tenant_id, entity_id),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS wsfed_relying_parties (
    tenant_id      TEXT        NOT NULL DEFAULT 'default',
    wtrealm        TEXT        NOT NULL,
    display_name   TEXT        NOT NULL DEFAULT '',
    reply_urls     JSONB       NOT NULL DEFAULT '[]'::jsonb,
    audience       TEXT        NOT NULL DEFAULT '',
    token_type     TEXT        NOT NULL DEFAULT '',
    claim_policy   JSONB       NOT NULL,
    entra_profile  JSONB,
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ,
    PRIMARY KEY (tenant_id, wtrealm),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT
);

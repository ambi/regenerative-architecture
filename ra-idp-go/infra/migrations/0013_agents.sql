-- ADR-048: tenant-scoped Agent (非人間 identity principal) と OAuth2Client 束縛。
-- Agent は自身の資格情報を持たず、agent_credential_bindings で既存 client に束縛して
-- トークンを得る。status は active / disabled / killed の三状態で killed は一方向終端。

CREATE TABLE IF NOT EXISTS agents (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT,
    kind        TEXT NOT NULL DEFAULT 'supervised',
    owner_sub   TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled', 'killed')),
    roles       JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ,
    disabled_at TIMESTAMPTZ,
    killed_at   TIMESTAMPTZ,
    CONSTRAINT agents_tenant_id_fkey
        FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT
);

-- Agent 名はテナント内で一意。
CREATE UNIQUE INDEX IF NOT EXISTS agents_tenant_name_idx
    ON agents (tenant_id, name);

CREATE TABLE IF NOT EXISTS agent_credential_bindings (
    agent_id   TEXT NOT NULL,
    client_id  TEXT NOT NULL,
    tenant_id  TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_id, client_id),
    CONSTRAINT agent_credential_bindings_agent_id_fkey
        FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);

-- client から束縛された Agent を引く (トークン発行経路の status gate)。
CREATE INDEX IF NOT EXISTS agent_credential_bindings_tenant_client_idx
    ON agent_credential_bindings (tenant_id, client_id);

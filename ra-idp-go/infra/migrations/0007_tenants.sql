CREATE TABLE IF NOT EXISTS tenants (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('active', 'disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    disabled_at TIMESTAMPTZ,
    CONSTRAINT tenants_id_format CHECK (
        id <> 'admin' AND id ~ '^[a-z0-9][a-z0-9-]{0,62}$'
    )
);

INSERT INTO tenants (id, display_name, status)
VALUES ('default', 'Default', 'active')
ON CONFLICT (id) DO NOTHING;

ALTER TABLE clients ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE users ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE consents ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE refresh_tokens ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';

ALTER TABLE consents DROP CONSTRAINT IF EXISTS consents_client_id_fkey;
ALTER TABLE consents DROP CONSTRAINT IF EXISTS consents_pkey;
ALTER TABLE refresh_tokens DROP CONSTRAINT IF EXISTS refresh_tokens_client_id_fkey;
ALTER TABLE clients DROP CONSTRAINT IF EXISTS clients_pkey;

ALTER TABLE clients ADD CONSTRAINT clients_pkey PRIMARY KEY (tenant_id, client_id);
ALTER TABLE clients ADD CONSTRAINT clients_tenant_id_fkey
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT;

DROP INDEX IF EXISTS users_preferred_username_active_idx;
CREATE UNIQUE INDEX users_tenant_username_active_idx
    ON users (tenant_id, preferred_username) WHERE deleted_at IS NULL;
ALTER TABLE users ADD CONSTRAINT users_tenant_id_fkey
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT;

ALTER TABLE consents ADD CONSTRAINT consents_pkey PRIMARY KEY (tenant_id, sub, client_id);
ALTER TABLE consents ADD CONSTRAINT consents_tenant_id_fkey
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT;
ALTER TABLE consents ADD CONSTRAINT consents_tenant_client_fkey
    FOREIGN KEY (tenant_id, client_id)
    REFERENCES clients(tenant_id, client_id) ON DELETE RESTRICT;

ALTER TABLE refresh_tokens ADD CONSTRAINT refresh_tokens_tenant_id_fkey
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT;
ALTER TABLE refresh_tokens ADD CONSTRAINT refresh_tokens_tenant_client_fkey
    FOREIGN KEY (tenant_id, client_id)
    REFERENCES clients(tenant_id, client_id) ON DELETE RESTRICT;
CREATE INDEX IF NOT EXISTS refresh_tokens_tenant_sub_idx
    ON refresh_tokens (tenant_id, sub);
CREATE INDEX IF NOT EXISTS refresh_tokens_tenant_client_idx
    ON refresh_tokens (tenant_id, client_id);

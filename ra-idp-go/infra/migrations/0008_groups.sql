-- ADR-038: tenant-scoped Group aggregate と user-group membership。
-- effective_roles = union(user.roles, group.roles)。階層・deny・自動所属なし。

CREATE TABLE IF NOT EXISTS groups (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT,
    roles       JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ,
    CONSTRAINT groups_tenant_id_fkey
        FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT
);

-- グループ名はテナント内で一意 (Keycloak / Okta と同等の制約)。
CREATE UNIQUE INDEX IF NOT EXISTS groups_tenant_name_idx
    ON groups (tenant_id, name);

CREATE TABLE IF NOT EXISTS group_members (
    group_id TEXT NOT NULL,
    user_sub TEXT NOT NULL,
    added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, user_sub),
    CONSTRAINT group_members_group_id_fkey
        FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE,
    CONSTRAINT group_members_user_sub_fkey
        FOREIGN KEY (user_sub) REFERENCES users(sub) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS group_members_user_sub_idx
    ON group_members (user_sub);

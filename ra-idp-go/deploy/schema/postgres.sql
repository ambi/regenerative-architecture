CREATE TABLE tenants (
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

CREATE TABLE clients (
    tenant_id TEXT NOT NULL DEFAULT 'default',
    client_id TEXT NOT NULL,
    client_secret_hash TEXT,
    client_name TEXT,
    client_type TEXT NOT NULL CHECK (client_type IN ('public', 'confidential')),
    redirect_uris JSONB NOT NULL,
    grant_types JSONB NOT NULL,
    response_types JSONB NOT NULL DEFAULT '[]'::jsonb,
    token_endpoint_auth_method TEXT NOT NULL,
    scope TEXT NOT NULL,
    jwks_uri TEXT,
    jwks JSONB,
    tls_client_auth_subject_dn TEXT,
    id_token_signed_response_alg TEXT NOT NULL DEFAULT 'PS256',
    require_pushed_authorization_requests BOOLEAN NOT NULL DEFAULT FALSE,
    dpop_bound_access_tokens BOOLEAN NOT NULL DEFAULT FALSE,
    fapi_profile TEXT NOT NULL DEFAULT 'none',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    first_party BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (tenant_id, client_id),
    CONSTRAINT clients_tenant_id_fkey
        FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT
);

CREATE TABLE users (
    sub TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT 'default',
    preferred_username TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    name TEXT,
    given_name TEXT,
    family_name TEXT,
    email TEXT,
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    mfa_enrolled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    roles JSONB NOT NULL DEFAULT '[]'::jsonb,
    lifecycle JSONB NOT NULL DEFAULT jsonb_build_object('status', 'active'),
    attributes JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT users_tenant_id_fkey
        FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX users_preferred_username_active_idx
    ON users (tenant_id, preferred_username)
    WHERE lifecycle->>'status' <> 'deleted';

CREATE TABLE mfa_factors (
    sub TEXT NOT NULL REFERENCES users(sub) ON DELETE CASCADE,
    type TEXT NOT NULL,
    secret TEXT,
    label TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ,
    PRIMARY KEY (sub, type)
);

CREATE TABLE consents (
    tenant_id TEXT NOT NULL DEFAULT 'default',
    sub TEXT NOT NULL REFERENCES users(sub) ON DELETE RESTRICT,
    client_id TEXT NOT NULL,
    scopes JSONB NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    PRIMARY KEY (tenant_id, sub, client_id),
    CONSTRAINT consents_tenant_id_fkey
        FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT,
    CONSTRAINT consents_tenant_client_fkey
        FOREIGN KEY (tenant_id, client_id)
        REFERENCES clients(tenant_id, client_id) ON DELETE RESTRICT
);

CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY,
    hash TEXT NOT NULL UNIQUE,
    family_id UUID NOT NULL,
    parent_id UUID REFERENCES refresh_tokens(id) ON DELETE NO ACTION,
    tenant_id TEXT NOT NULL DEFAULT 'default',
    client_id TEXT NOT NULL,
    sub TEXT NOT NULL,
    scopes JSONB NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    absolute_expires_at TIMESTAMPTZ NOT NULL,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    rotated BOOLEAN NOT NULL DEFAULT FALSE,
    sender_constraint JSONB,
    CONSTRAINT refresh_tokens_tenant_id_fkey
        FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT,
    CONSTRAINT refresh_tokens_tenant_client_fkey
        FOREIGN KEY (tenant_id, client_id)
        REFERENCES clients(tenant_id, client_id) ON DELETE RESTRICT
);

CREATE INDEX refresh_tokens_family_id_idx ON refresh_tokens (family_id);
CREATE INDEX refresh_tokens_tenant_sub_idx ON refresh_tokens (tenant_id, sub);
CREATE INDEX refresh_tokens_tenant_client_idx ON refresh_tokens (tenant_id, client_id);

CREATE TABLE signing_keys (
    kid TEXT PRIMARY KEY,
    alg TEXT NOT NULL,
    public_jwk JSONB NOT NULL,
    private_jwk JSONB NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    rotated_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX signing_keys_single_active_idx
    ON signing_keys (active)
    WHERE active;

CREATE TABLE outbox (
    id BIGSERIAL PRIMARY KEY,
    event_type TEXT NOT NULL,
    topic TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    published_to TEXT,
    attempts INT NOT NULL DEFAULT 0,
    last_error TEXT
);

CREATE INDEX outbox_unpublished_idx ON outbox (id) WHERE published_at IS NULL;

CREATE TABLE password_history (
    id BIGSERIAL PRIMARY KEY,
    sub TEXT NOT NULL REFERENCES users(sub) ON DELETE CASCADE,
    encoded TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX password_history_sub_created_at_idx
    ON password_history (sub, created_at DESC, id DESC);

CREATE TABLE password_reset_tokens (
    token_hash TEXT PRIMARY KEY,
    sub TEXT NOT NULL REFERENCES users(sub) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX password_reset_tokens_sub_idx ON password_reset_tokens (sub);
CREATE INDEX password_reset_tokens_expires_at_idx ON password_reset_tokens (expires_at);

CREATE TABLE groups (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    roles JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    CONSTRAINT groups_tenant_id_fkey
        FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX groups_tenant_name_idx ON groups (tenant_id, name);

CREATE TABLE group_members (
    group_id TEXT NOT NULL,
    user_sub TEXT NOT NULL,
    added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, user_sub),
    CONSTRAINT group_members_group_id_fkey
        FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE,
    CONSTRAINT group_members_user_sub_fkey
        FOREIGN KEY (user_sub) REFERENCES users(sub) ON DELETE CASCADE
);

CREATE INDEX group_members_user_sub_idx ON group_members (user_sub);

CREATE TABLE tenant_user_attribute_schemas (
    tenant_id TEXT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    attributes JSONB NOT NULL DEFAULT '[]'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE email_change_tokens (
    token_hash TEXT PRIMARY KEY,
    sub TEXT NOT NULL REFERENCES users(sub) ON DELETE CASCADE,
    new_email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX email_change_tokens_sub_idx ON email_change_tokens (sub);
CREATE INDEX email_change_tokens_expires_at_idx ON email_change_tokens (expires_at);

CREATE TABLE audit_events (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL,
    sub TEXT,
    occurred_at TIMESTAMPTZ NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX audit_events_tenant_occurred_idx
    ON audit_events (tenant_id, occurred_at DESC);
CREATE INDEX audit_events_type_idx ON audit_events (type);
CREATE INDEX audit_events_sub_idx ON audit_events (sub) WHERE sub IS NOT NULL;

CREATE TABLE authentication_event_buckets (
    tenant_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    key_hash TEXT NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    count BIGINT NOT NULL DEFAULT 0,
    first_seen TIMESTAMPTZ NOT NULL,
    last_seen TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, kind, key_hash, window_start)
);

CREATE INDEX authentication_event_buckets_window_idx
    ON authentication_event_buckets (tenant_id, window_start DESC);

CREATE TABLE agents (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    kind TEXT NOT NULL DEFAULT 'supervised',
    owner_sub TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled', 'killed')),
    roles JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    disabled_at TIMESTAMPTZ,
    killed_at TIMESTAMPTZ,
    CONSTRAINT agents_tenant_id_fkey
        FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX agents_tenant_name_idx ON agents (tenant_id, name);

CREATE TABLE agent_credential_bindings (
    agent_id TEXT NOT NULL,
    client_id TEXT NOT NULL,
    tenant_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_id, client_id),
    CONSTRAINT agent_credential_bindings_agent_id_fkey
        FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);

CREATE INDEX agent_credential_bindings_tenant_client_idx
    ON agent_credential_bindings (tenant_id, client_id);

CREATE UNIQUE INDEX agent_credential_bindings_tenant_client_unique_idx
    ON agent_credential_bindings (tenant_id, client_id);

CREATE TABLE authorization_detail_types (
    tenant_id TEXT NOT NULL,
    type TEXT NOT NULL,
    description TEXT,
    schema JSONB NOT NULL DEFAULT jsonb_build_object('rules', jsonb_build_array()),
    display_template TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'Enabled'
        CHECK (state IN ('Enabled', 'Disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, type),
    CONSTRAINT authorization_detail_types_tenant_id_fkey
        FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT
);

CREATE TABLE applications (
    tenant_id TEXT NOT NULL DEFAULT 'default',
    application_id UUID NOT NULL,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    status TEXT NOT NULL,
    icon_url TEXT NOT NULL DEFAULT '',
    icon_object_key TEXT NOT NULL DEFAULT '',
    launch_url TEXT NOT NULL DEFAULT '',
    bindings JSONB NOT NULL DEFAULT '[]'::jsonb,
    category_ids TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, application_id)
);

CREATE TABLE application_icons (
    tenant_id TEXT NOT NULL DEFAULT 'default',
    application_id UUID NOT NULL,
    object_key TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    data BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, application_id, object_key),
    FOREIGN KEY (tenant_id, application_id)
        REFERENCES applications (tenant_id, application_id) ON DELETE CASCADE
);

CREATE TABLE application_assignments (
    tenant_id TEXT NOT NULL DEFAULT 'default',
    application_id UUID NOT NULL,
    subject_type TEXT NOT NULL,
    subject_id TEXT NOT NULL,
    visibility TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, application_id, subject_type, subject_id),
    FOREIGN KEY (tenant_id, application_id)
        REFERENCES applications (tenant_id, application_id) ON DELETE CASCADE
);

CREATE INDEX application_assignments_subject_idx
    ON application_assignments (tenant_id, subject_type, subject_id);

CREATE TABLE saml_service_providers (
    tenant_id TEXT NOT NULL DEFAULT 'default',
    entity_id TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    acs_urls JSONB NOT NULL DEFAULT '[]'::jsonb,
    slo_url TEXT NOT NULL DEFAULT '',
    audience TEXT NOT NULL DEFAULT '',
    claim_policy JSONB NOT NULL,
    sign_assertion BOOLEAN NOT NULL DEFAULT TRUE,
    sign_response BOOLEAN NOT NULL DEFAULT FALSE,
    want_authn_requests_signed BOOLEAN NOT NULL DEFAULT FALSE,
    authn_request_signing_certificate_pem TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ,
    PRIMARY KEY (tenant_id, entity_id),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT
);

CREATE TABLE wsfed_relying_parties (
    tenant_id TEXT NOT NULL DEFAULT 'default',
    wtrealm TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    reply_urls JSONB NOT NULL DEFAULT '[]'::jsonb,
    audience TEXT NOT NULL DEFAULT '',
    token_type TEXT NOT NULL DEFAULT '',
    claim_policy JSONB NOT NULL,
    entra_profile JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ,
    PRIMARY KEY (tenant_id, wtrealm),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT
);

CREATE TABLE application_orderings (
    tenant_id TEXT NOT NULL DEFAULT 'default',
    user_sub TEXT NOT NULL,
    application_ids TEXT[] NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, user_sub)
);

CREATE TABLE application_categories (
    tenant_id TEXT NOT NULL DEFAULT 'default',
    category_id UUID NOT NULL,
    name TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, category_id)
);

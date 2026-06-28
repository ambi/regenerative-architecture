-- wi-60: make Agent credential binding deterministic for token issuance.
-- A client_id can be bound to at most one Agent per tenant; otherwise
-- FindByClientID would make the status gate and agent_id claim ambiguous.

CREATE UNIQUE INDEX IF NOT EXISTS agent_credential_bindings_tenant_client_unique_idx
    ON agent_credential_bindings (tenant_id, client_id);

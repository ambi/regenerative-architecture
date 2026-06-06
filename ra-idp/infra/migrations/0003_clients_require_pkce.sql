-- =================================================================
-- 0003 — clients.require_pkce (ADR-002 改訂: PKCE 階段化)
-- =================================================================
--
-- spec/scl.yaml OAuth2Client.require_pkce に対応。NULL の場合は client_type と
-- fapi_profile から動的に決める (public / FAPI → true, confidential → false)。
-- 既存行は NULL のままで、起動時の resolveRequirePkce() がデフォルトを与える。

ALTER TABLE clients
    ADD COLUMN require_pkce BOOLEAN;

COMMENT ON COLUMN clients.require_pkce IS
    'PKCE 必須かの明示。NULL は client_type / fapi_profile から派生 (ADR-002 改訂)。';

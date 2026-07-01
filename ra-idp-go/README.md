# ra-idp-go — Go Implementation of IdP

An Identity Provider (IdP) application developed in Go following the Regenerative Architecture (RA) principles, based on the specification core defined in `spec/scl.yaml`.

For AI agents and new developers, the primary implementation index is located in [`ARCHITECTURE.md`](ARCHITECTURE.md). The canonical specification is in `spec/`, the architectural decisions are in `decisions/`, and change logs/verification records are in `work-items/`.

## Scope

This application acts as an OAuth 2.0 / OpenID Connect authorization server and IdP with the following features. The design decisions for each feature correspond to `spec/scl.yaml`, ADRs in `decisions/`, and work items in `work-items/`.

### Protocol Endpoints

- **Authorization Endpoint (`/authorize`)**: OAuth 2.0 Authorization Code Flow (RFC 6749) + PKCE (RFC 7636), OpenID Connect Core 1.0. Includes login/consent screens and RP-Initiated Logout (`/end_session`, OpenID Connect RP-Initiated Logout 1.0).
- **Token Endpoint (`/token`)**: Support for `authorization_code`, `refresh_token`, and `client_credentials` grant types (OAuth 2.0 RFC 6749), as well as `device_code` (Device Authorization Grant RFC 8628).
- **Pushed Authorization Requests (`/par`)**: Support for PAR (RFC 9126).
- **Device Authorization Grant (`/device_authorization`, `/device`)**: Support for device-based flow (RFC 8628).
- **Token Introspection (`/introspect`)**: Token introspection endpoint (RFC 7662).
- **Token Revocation (`/revoke`)**: Token revocation endpoint (RFC 7009).
- **Token Exchange**: Delegation and impersonation with delegation chains via RFC 8693 (`urn:ietf:params:oauth:grant-type:token-exchange` grant type at `/token`), with constrained use of Resource Indicators RFC 8707 (wi-50).
- **Rich Authorization Requests (`authorization_details`)**: Display on consent screens, disclosure in introspection, scope downscaling during token exchange, and a type registry for management (RFC 9396, wi-51).
- **Userinfo Endpoint (`/userinfo`)**: User information endpoint (OpenID Connect Core 1.0 §5.3).
- **Dynamic Client Registration (`/register`)**: Support for client registration (RFC 7591).
- **OpenID Connect Discovery (`/.well-known/openid-configuration`) & JWK Set (`/jwks`)**: OpenID Connect Discovery 1.0 and JWK Set (RFC 7517).
- **DPoP**: Demonstrating Proof-of-Possession at the Application Layer (RFC 9449) for sender-constrained tokens.
- **`private_key_jwt` Client Authentication**: Client authentication (RFC 7523) using inline JWKS or `jwks_uri`.
- **WS-Federation**: Active IP-STS `/wsfed` supporting WS-Federation passive requestor profile (`wa=wsignin1.0` for browser SSO and `wsignout1.0` / `wsignoutcleanup1.0` for sign-out; signs SAML assertions and automatically POSTs them via RSTR to relying parties, wi-61). Tokens default to SAML 1.1 (compatibility with Entra / AD FS), with SAML 2.0 selectable in relying party settings. Respects `wfresh` (max authentication age in minutes) and `wauth` (requested authentication method; rejects unsupported methods like Integrated Windows Authentication via fail-closed behavior, silent sign-in is covered in wi-65). Relying parties are identified by `wtrealm`, managed under `/api/admin/wsfed/relying-parties` with constraints on allowed `wreply` URLs, token types, and claim issuance policies. Claims use declarative mappings (ADR-059) and XML signatures use `goxmldsig` (ADR-060). Publishes AD FS-compatible federation metadata at `/{realm}/federationmetadata/2007-06/federationmetadata.xml` and WS-Trust MEX at `/{realm}/trust/mex` to advertise issuer, endpoints, and signing certificates (ADR-062, wi-63).
- **Microsoft Entra Domain Federation Preset**: Endpoint `/api/admin/wsfed/entra-federation` (wi-64). Given a verified domain, `IssuerUri`, and `sourceAnchor` attribute, it creates a WS-Fed RP requiring UPN, ImmutableID, and persistent NameID as claims, returning `PassiveLogOnUri`, `ActiveLogOnUri`, and `MetadataExchangeUri` for Microsoft Entra ID registration. The `sourceAnchor` is validated and normalized as a GUID string or base64 ImmutableID. Note: Hybrid Azure AD Join device registration requires `windowstransport` + computer account Kerberos, which is currently unsupported; use managed/PHS or AD FS coexistence as a workaround (ADR-065).
- **WS-Trust 1.3 Active Requestor STS**: Endpoint `/trust/usernamemixed` (Issue binding only, WS-Security UsernameToken authentication, validates Timestamp / MessageID replay / To / Action / RequestType / KeyType / AppliesTo as fail-closed, returning signed SAML assertions inside a SOAP RSTR for registered RPs, ADR-063, wi-62). `windowstransport` / Kerberos is out of scope.
- **SAML 2.0 IdP**: Endpoints `/{realm}/saml/sso`, `/{realm}/saml/slo`, and `/{realm}/saml/metadata` (SAML 2.0 Web Browser SSO Profile, wi-29, ADR-067). SP-initiated SSO accepts `SAMLRequest` (deflate+base64 for HTTP-Redirect, or base64 for HTTP-POST), and IdP-initiated SSO is initiated via the `entityID` query parameter. Both automatically POST signed `<saml:Assertion>` enclosed in `<samlp:Response>` to the ACS. Default signing is assertion-level, with response-level signing (similar to Okta / Entra "Sign Response") configurable. The Issuer must match the registered SP's entityID exactly, the Destination must be the current realm's SSO endpoint, and the `AssertionConsumerServiceURL` is validated against SP-specific allowed URLs to prevent open redirects. Respects `ForceAuthn`, prompting re-login if the authentication is stale, and limits the audience to the SP's entityID (fail-closed). SPs can require AuthnRequest/LogoutRequest signatures, verified with registered X.509 certificates. `/saml/slo` handles LogoutRequests, destroys the local session, and returns a signed LogoutResponse to SingleLogoutService endpoints. The IdP metadata endpoint advertises the `IDPSSODescriptor`, endpoints, signing certificates, and supported NameID formats. SPs are managed in the application catalog under SAML binding (`/api/admin/applications`). SAML ECP, encrypted assertions, and inbound federation as an SP are out of scope.

### Authentication, Accounts, and Administration

- **Browser Authentication API (`/api/auth/*`)**: Session Cookie + CSRF protection.
- **Admin Console & Account Portal**: Authenticated via the IdP's own OIDC RP (`authorization_code` + PKCE, first-party public clients `ra-admin-console` / `ra-account-portal`). Endpoints under `/api/{admin,account}/*` act as resource servers validating RFC 9068 Access Tokens (ADR-061, wi-66). A session-based emergency login endpoint (`POST /api/auth/login`) is retained as a fallback in case of configuration failure.
- **Password Reset via Email**: Single-use, 30-minute TTL tokens (ADR-030).
- **Role-Based Access Control**: Admin user management API (`/api/admin/users`), user disabling (ADR-031), and a 3-step deletion process (soft-delete -> 30-day restoration window -> hard-delete/anonymization cascade, ADR-036 / ADR-072).
- **Groups**: Group aggregation, membership management, and CRUD API/UI (`/admin/groups`) constrained within tenants. Effective roles are evaluated as `user.roles ∪ ⋃ group.roles` (ADR-038).
- **Roles and Permissions**: Administration API/UI `/api/admin/policy/roles` and `/admin/roles` to inspect roles, permissions, and associated HTTP endpoints.
- **Tenant Settings**: Admin settings API/UI `/api/admin/settings` and `/admin/settings` (viewing/updating display names and password policy overrides).
- **Client Management**: Client CRUD UI (`/admin/clients`) isolated per tenant.
- **Consent Management**: API/UI (`/admin/consents`) to view and revoke user consent.
- **AuthZEN**: Policy evaluation (local/remote).
- **Non-Human Principals**: Foundation for treating AI agents as first-class non-human principals (with owners and emergency-stop capabilities, wi-49 / security mitigation in wi-60).

### Tenant, Infrastructure, and Operations

- **Tenant Isolation**: `/realms/{tenant_id}` tenancy structure, management APIs, and database/storage isolation (ADRs 032–034).
- **Refresh Token Rotation (RTR)**: Rotation and family revocation (ADR-004).
- **JWT Signing**: PS256 signature scheme, JWK Set, and memory/PostgreSQL keystores (using RFC 7638 thumbprints for `kid`).
- **Persistence**: PostgreSQL for persistent state, Valkey for volatile state.
- **Outbox Pattern**: PostgreSQL outbox tables processed by the Kafka relay (`ra-idp-relay`).
- **Observability**: Traces and metrics via OpenTelemetry OTLP/HTTP.
- **Domain Events**: Dispatched via Admin console and outbox event sinks.
- **Validation**: Schema, HTTP input, and password strength validation via Zog.

## Getting Started

The UI is built using TypeScript + Vite + React + Tailwind CSS + Radix UI + shadcn/ui + TanStack Router/Table. It is delivered as a separate artifact and runs in a separate process, integrated with the Go API under a single origin via Caddy (or similar gateway). See [`ui/README.md`](ui/README.md) for UI design decisions.

For local development, start the Go API and React UI in separate processes:

```bash
# Terminal 1: Go API
ADDR=:8081 ISSUER=http://localhost:5173 go run ./cmd/ra-idp-go

# Terminal 2: React UI (includes API proxy configuration)
cd ui
bun install
bun run dev
```

Open `http://localhost:5173/` in your browser and select "Start Local Demo Authentication". Do not open `/login` directly, as the login flow requires an active authorization transaction.

### Docker Compose Stack

In Docker Compose, Caddy exposes both the UI and the API under `http://localhost:8080`.

```bash
docker compose -f deploy/docker/docker-compose.dev.yaml up --build
```

In the dev compose stack, the `schema` service applies `deploy/schema/postgres.sql` using `psqldef` once PostgreSQL is ready. The `idp` service starts only after the schema is successfully applied. To re-apply the schema manually:

```bash
docker compose -f deploy/docker/docker-compose.dev.yaml run --rm schema
```

To run a demonstration script covering the primary OAuth 2.0 / OpenID Connect flows:

```bash
BASE=http://localhost:8080 ./demo.sh
```

### Local Email Testing (Mailpit)

By default, `EMAIL_SENDER=console` outputs reset links directly to stdout. However, you can easily test the SMTP adapter (ADR-035) locally using [Mailpit](https://mailpit.axllent.org/) as a catch-all mock inbox.

```bash
# 1) Start Mailpit (example using Homebrew)
brew install mailpit
mailpit --smtp 127.0.0.1:1025 --listen 127.0.0.1:8025

# 2) In another terminal, start ra-idp-go with SMTP settings
export EMAIL_SENDER=smtp
export SMTP_HOST=127.0.0.1
export SMTP_PORT=1025
export SMTP_TLS=none
export SMTP_FROM=noreply@ra-idp.test
./dev.sh
```

Ensure the startup logs show `email sender: smtp host=127.0.0.1 port=1025 tls=none from=...`.

When you trigger "Forgot Password" in the UI for `alice@example.com` (from the demo seed), the reset email containing the link will appear in the Mailpit web UI at `http://127.0.0.1:8025`. Mailpit captures all outbound mails locally and will not send them to the actual recipient.

*Note: `SMTP_TLS=none` is only for local mock servers. Production environments must use `starttls` or `implicit`.*

### Production Adapter Configuration

The PostgreSQL schema uses `deploy/schema/postgres.sql` as its single source of truth. Schema migrations are applied using `psqldef` after verifying changes with a dry-run.

```bash
# Verify schema difference
psqldef -U "$PGUSER" -h "$PGHOST" -p "$PGPORT" "$PGDATABASE" --dry-run < deploy/schema/postgres.sql

# Apply schema difference
psqldef -U "$PGUSER" -h "$PGHOST" -p "$PGPORT" "$PGDATABASE" --apply < deploy/schema/postgres.sql
```

For structure changes, update `deploy/schema/postgres.sql` first. If data migration/backfilling is required, add runbooks in the Work Item or a custom SQL migration script. This application does not use runtime code-based migration runners. For deployment, standard environment variables (`PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD`, etc.) are injected by the pipeline. Refer to `deploy/schema/README.md` for details on installation, diff generation, review, and application.

To run the components with production adapter configs:

```bash
PERSISTENCE=postgres \
DATABASE_URL='postgres://ra_idp:ra_idp@localhost:5432/ra_idp?sslmode=disable' \
VALKEY_URL='valkey://localhost:6379/0' \
EVENT_SINK=outbox \
OBSERVABILITY=otel \
OTEL_EXPORTER_OTLP_ENDPOINT='http://localhost:4318' \
go run ./cmd/ra-idp-go
```

```bash
DATABASE_URL='postgres://ra_idp:ra_idp@localhost:5432/ra_idp?sslmode=disable' \
KAFKA_BROKERS='localhost:9092' \
go run ./cmd/ra-idp-relay
```

### Configuration Environment Variables

| Variable | Values / Default | Description |
| --- | --- | --- |
| `PERSISTENCE` | `memory` / `postgres` (`memory`) | Storage adapter configuration |
| `DATABASE_URL` | Connection string | Required if `PERSISTENCE=postgres` |
| `VALKEY_URL` | Connection string | Required if `PERSISTENCE=postgres` |
| `EVENT_SINK` | `console` / `outbox` (`console`) | Destination for emitted domain events |
| `OBSERVABILITY` | `noop` / `otel` (`noop`) | Enables OpenTelemetry tracing and metrics |
| `AUTHZEN` | `local` / `remote` (`local`) | AuthZEN authorization mode |
| `AUTHZEN_URL` | URL string | Base URL for remote AuthZEN server |
| `KAFKA_BROKERS` | Comma-separated list | Kafka brokers for the event relay |
| `SKIP_DEMO_SEED` | `true` / `false` | If set, skips seeding demo data |
| `LEGACY_BARE_ISSUER`| `true` / `false` (`false`) | If `true`, fallback default issuer is `{base}` |
| `EMAIL_SENDER` | `console` / `smtp` (`console`) | Email sender adapter. `smtp` requires SMTP_* variables |
| `SMTP_HOST` | Host string | Required if `EMAIL_SENDER=smtp` |
| `SMTP_PORT` | Port number | Defaults based on `SMTP_TLS` (starttls: 587, implicit: 465, none: 25) |
| `SMTP_USERNAME` | Username | Plain auth username (omit for no auth) |
| `SMTP_PASSWORD` | Password | Plain auth password (hidden from logs, ADR-035 §10) |
| `SMTP_FROM` | Email address | RFC 5322 From address / SMTP MAIL FROM (bare address) |
| `SMTP_HELO` | Host string | Name used in SMTP EHLO/HELO (`localhost`) |
| `SMTP_TLS` | `starttls` / `implicit` / `none` (`starttls`) | SMTP encryption method. `none` is for dev only |
| `SMTP_TIMEOUT_SECONDS` | Number (`10`) | Connection and command timeout in seconds |
| `BREACHED_PASSWORD_CHECKER` | `noop` / `hibp` (`noop`) | `hibp` queries `api.pwnedpasswords.com`. Fails open (ADR-028) |

- `jwks_uri` requests only permit HTTPS, rejecting loopback, link-local, private IP addresses, userinfo, and fragments. Requests have a 3-second timeout, 1 MiB limit, and a 5-minute cache TTL.
- Model, HTTP inputs, and password strength validations are performed via [Zog](https://zog.dev/). Context-dependent validations (e.g. redirect URI matching, scope validation, state transitions, PKCE verification) reside in the Usecase/Domain layer.

### Configuring Microsoft Entra Domain Federation

Steps to federate a verified domain to Microsoft 365 for sign-in (wi-64, ADR-065):

1. Save the verified domain, `sourceAnchor` attribute (defaults to `object_guid`), and `IssuerUri` in the Admin UI `/admin/federation/entra` (or via `POST /api/admin/wsfed/entra-federation`). The response displays the settings (`IssuerUri`, `PassiveLogOnUri`, `ActiveLogOnUri`, and `MetadataExchangeUri`) and the federation metadata URL for signing certificate retrieval.
2. Register the federation settings via Microsoft Graph PowerShell. Pass the generated values to `Update-MgDomainFederationConfiguration` (or legacy `Set-MsolDomainAuthentication`):

   | UI Display Value | `Update-MgDomainFederationConfiguration` | Legacy `Set-MsolDomainAuthentication` |
   | --- | --- | --- |
   | `IssuerUri` | `-IssuerUri` | `-IssuerUri` |
   | `PassiveLogOnUri` | `-PassiveSignInUri` | `-PassiveLogOnUri` |
   | `ActiveLogOnUri` | `-ActiveSignInUri` | `-ActiveLogOnUri` |
   | `MetadataExchangeUri` | `-MetadataExchangeUri` | `-MetadataExchangeUri` |
   | X.509 from metadata | `-SigningCertificate` | `-SigningCertificate` |

   Apply `-PreferredAuthenticationProtocol wsFed` and `-FederatedIdpMfaBehavior` in accordance with your policies.

Issued tokens must include UPN (`http://schemas.xmlsoap.org/claims/UPN`, by default resolved from `preferred_username`) and ImmutableID (base64-encoded `sourceAnchor`, populated in the persistent NameID and `http://schemas.xmlsoap.org/claims/nameidentifier`). The configuration preset enforces these as fail-closed checks. The `sourceAnchor` must map to an immutable attribute like AD's `objectGUID` (fed by Entra Connect or a similar process on the customer side).

*Silent sign-in from domain-joined PCs requires Kerberos/SPNEGO inbound (wi-65). Hybrid Azure AD Join device registration (which requires `windowstransport` + computer account Kerberos) is out of scope. Use managed/PHS or AD FS coexistence as a workaround.*

## Verification

```bash
go test -race ./...
golangci-lint run
```

Tests cover atomic authorization code redemption, Valkey Lua operations, refresh token family revocation, device flows, client authentication, SSRF mitigation on `jwks_uri`, AuthZEN integration contracts, and domain event formats.

### Demo Seeds

| Resource | Value |
| --- | --- |
| `client_id` | `demo-client` |
| `client_secret` | `demo-client-secret` |
| `redirect_uri` | `http://localhost:3000/callback` |
| Username | `alice` |
| Password | `demo-password-1234` |

## Directory Structure

```text
ra-idp-go/
├── spec/                                    Layer 1: Specification Core (SCL)
├── decisions/                               Layer 2: Conceptions / ADRs
├── ui/                                      React SPA + Caddy reference configuration
│   └── src/features/                       UI feature boundaries
├── cmd/ra-idp-go/main.go                   App entry point
├── internal/shared/                        Technical shared context
│   ├── spec/                               Layer 1 bindings: SCL structs & state machines
│   └── adapters/                           Layer 4: Cross-context adapter implementations
│       ├── crypto/                         Argon2id, PS256, DPoP, private_key_jwt
│       ├── persistence/                    Memory / PostgreSQL / Valkey (per-resource files)
│       ├── http/
│       │   ├── support/                    HTTP shared infrastructure (Deps, middlewares, response helpers)
│       │   └── server/                     Echo v5 router (aggregates adapters/http from all contexts)
│       ├── observability/                  OpenTelemetry
│       ├── policy/                         Local / remote AuthZEN
│       ├── notification/                   Email sending
│       └── eventsink/                      Console / Kafka relay
├── internal/tenancy/                       Layer 3+4: Tenancy (domain, ports, usecases, adapters/http)
├── internal/oauth2/                        Layer 3+4: OAuth2 (domain, ports, usecases, adapters/http)
├── internal/authentication/                Layer 3+4: Authentication (domain, ports, usecases, adapters/http)
├── internal/bootstrap/                     Layer 5: Composition / DI / server wiring
└── deploy/                                 Layer 5: Declarative schema, Docker Compose, OTel Collector
```

> **Structural Note (ADR-047, ADR-070)**: Along with the 5 horizontal layers, the codebase is split vertically into bounded contexts (RA §3.6). SCL Go bindings and shared Layer 4 adapter implementations are located in `internal/shared/` as a technical shared context. Layer 3 logic and context-owned HTTP adapters (`adapters/http`) reside within their respective context packages. To prevent circular dependencies, HTTP routers are split into context-independent support libraries (`adapters/http/support`) and a central route aggregator (`adapters/http/server`). The `internal/` directory enforces Go import visibility. `deploy/` contains declarative infrastructure manifests (Layer 5) isolated from the Go source files.

## Implementation Roadmap

While this implementation showcases RA principles, it lacks some production-ready features. The following table highlights planned but not yet implemented (non-work-item) backlog areas, ordered roughly from infrastructure to frontend.

### Authentication, MFA, and Account Recovery

| Area | Planned Features |
| --- | --- |
| Authentication | Passwordless magic links via email |
| Assurance Levels | Alignment with identity assurance standards (AAL / IAL) |
| Adaptive Authentication | Foundation for risk-based/adaptive auth (re-authentication is implemented in wi-43) |
| Account Recovery | Integrated recovery workflows (recovery code component implemented in wi-26, recovery email in wi-41) |

### Management, Clients, and Delegation

| Area | Planned Features |
| --- | --- |
| DCR Extensions | Support for `registration_access_token`, `software_statement`, and client metadata updates/deletions (secret rotation implemented in wi-25) |
| Delegation / Impersonation | User impersonation and guest access (delegation chains implemented in wi-50) |

### Consent and Privacy

| Area | Planned Features |
| --- | --- |
| Consent Management | Displaying scope purposes, consent grouping by purpose |
| Subject Rights | Async DSAR export, Object Storage integration, hard-delete audit trail |
| Retention | Region-specific retention policies, systematic data minimization |

### Federation and Provisioning

| Area | Planned Features |
| --- | --- |
| Outbound Federation | SAML 2.0 Web Browser SSO/SLO/Metadata (wi-29; encrypted assertion/ECP out of scope), WS-Federation Passive (wi-61), WS-Trust active STS (wi-62), federation metadata & claim mapping (wi-63), Entra domain federation / M365 SSO (wi-64; Hybrid Join out of scope) |
| Inbound Enterprise | Kerberos / SPNEGO inbound for desktop silent SSO (wi-65), LDAP / Active Directory bind integration |

### Protocols and High-Assurance Profiles

| Area | Planned Features |
| --- | --- |
| Request Security | JWT Secured Authorization Request (JAR, RFC 9101) |
| Response Security | JWT Secured Authorization Response Mode (JARM), response signing, encrypted ID Tokens (JWE) |
| Tokens | Generalized Resource Indicators (RFC 8707; currently limited to token exchange), pairwise subject identifiers |
| Auth Flows | Step-up Authentication Challenge Protocol (RFC 9470) |
| FAPI / IDA | OpenID Connect for Identity Assurance (FAPI conformance suite alignment in wi-33) |
| Spec Compliance | Continued alignment with OAuth 2.0 Security BCP / OAuth 2.1 |

### Operations, Availability, and Security

| Area | Planned Features |
| --- | --- |
| Threat Mitigation | WAF, anomaly detection (impossible travel), token revocation playbooks for breaches (SSRF mitigation on `jwks_uri` is implemented) |
| Availability | Multi-region support, zero-downtime migrations, DR exercises, capacity planning |
| Security Ops | Penetration testing, vulnerability disclosure policy, chaos engineering, tamper-evident audit logs |
| Compliance | OIDC/FAPI certification, SOC2/ISO27001 evidence, audit reports, DPA exports |

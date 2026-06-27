# Frontend / Backend Architecture

## Deployment boundary

React and Go are separate build artifacts and separate services.

```text
Browser
  |
  | same origin
  v
Gateway / static server (Caddy, Nginx, CDN + proxy, etc.)
  |-- /login, /consent, /device, /status, /admin/* -> React SPA
  `-- /api/* and OAuth/OIDC endpoints                -> Go
```

Caddy is the reference configuration, not a required runtime. Any gateway that preserves the
same-origin boundary, TLS, headers, and routing contract can replace it.

## Authorization transaction

The Go service keeps the complete OAuth authorization request server-side. Its internal UUID is
stored only in a short-lived `HttpOnly`, `Secure` in HTTPS, `SameSite=Lax` transaction cookie.
It is not included in HTML, URLs, or JavaScript-readable application state.

The SPA calls `GET /api/auth/transaction` to obtain only display data such as the screen kind,
client name, and requested scopes. Login and consent commands resolve the transaction from the
cookie.

## Browser protections

- Session and authorization transaction cookies are `HttpOnly`.
- State-changing UI APIs require a double-submit CSRF cookie and `X-CSRF-Token` header.
- State-changing browser APIs require an `Origin` header matching the configured public issuer.
- Consent verifies that the current login session subject matches the authorization transaction.
- Authorization requests expire after ten minutes and completed requests cannot be reused.
- OAuth redirect URIs, PKCE values, scopes, and client identifiers are read from server-side state.
- UI API responses use `Cache-Control: no-store` and never return credentials or internal request IDs.

## API boundary

Browser-facing authentication APIs live under `/api/auth/*`. OAuth/OIDC protocol endpoints retain
their standard paths. Management APIs live under `/api/admin/*` and self-service APIs under
`/api/account/*`; both use explicit authorization policies independently from the login
transaction APIs.

## Admin console and account portal as OIDC RPs

The admin console (`/admin/*`) and account portal (`/account/*`) authenticate as OIDC relying
parties of the IdP itself, using `authorization_code` + PKCE against the IdP's own `/authorize`
and `/token` (ADR-061). They are registered as first-party public clients (`ra-admin-console`,
`ra-account-portal`) whose consent screen is skipped because the resource owner is the IdP user.

Because they are pure SPA RPs, the access token is held in the browser (`sessionStorage`) and sent
as `Authorization: Bearer` to `/api/{admin,account}/*`, which validate it as RFC 9068 resource
servers. This is a deliberate departure from a strict "no tokens in JavaScript" posture; it is
bounded by short-lived access tokens (600 s), `Cache-Control: no-store`, and keeping tokens out of
URLs, logs, and the DOM. The first-party session login (`POST /api/auth/login`) is retained as an
emergency bootstrap path so a broken OIDC client/key configuration cannot lock administrators out.

## Client-side routing

The SPA uses TanStack Router for client-side navigation with file-based routes under
`src/routes/`. The Vite router plugin generates `src/routeTree.gen.ts` and applies automatic code
splitting, including route loaders and components. Each route file owns its own `loader`, API
requests, page component, and path params (ADR-061, wi-67). Files prefixed with `-` are route-local
helpers and are excluded from route generation. Internal admin/account navigation uses `<Link>`, so
moving between pages does not reload the document or re-fetch every page's data — only the target
route's loader runs.
The OIDC login guard (`ensureLoggedIn`) runs inside the loader, so it applies to both the initial
load and in-app navigation. Auth-flow transitions (login/consent/callback and the OIDC redirects)
remain full-page navigations by nature. The rendered page kind is asserted to the DOM via
`<meta name="ra-idp:page">` for E2E.

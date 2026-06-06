# OAuth2 Component

This component owns OAuth 2.0 and OpenID Connect behavior: authorization
requests, consent, authorization code issuing, token grants, introspection,
revocation, userinfo, device authorization, dynamic client registration, PAR,
and signing-key rotation.

Structure:

- `domain/`: OAuth2/OIDC domain objects and invariants.
- `usecases/`: protocol workflows such as authorization, token exchange, PAR,
  device flow, and key rotation.
- `ports/`: persistence, signing, replay, and token-introspection contracts.
- `protocol/`: OAuth2/OIDC wire-level protocol types and errors.

It consumes authentication through `AuthenticationContext` and
`LoginContinuation`. It must not depend on password verification, browser
session cookie parsing, or login-session storage details.

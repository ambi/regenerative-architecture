# Authentication Component

This component owns browser user authentication: password verification,
login-session creation and revocation, and restoration of
`AuthenticationContext`.

Structure:

- `domain/`: authentication context and login-session model.
- `usecases/`: password verification and session-management behavior.
- `ports/`: persistence and continuation contracts used by authentication.

It does not issue authorization codes, evaluate OAuth2 consent, or inspect
OAuth2 request state directly.

# UI E2E tests

These tests run with Bun's built-in test runner and `Bun.WebView`. They do not
use Playwright, so local and CI runs do not need to download browser binaries.

## Shape

- `fixtures.ts` owns the real Go server, Vite server, callback listener, page
  marker polling, and shared login helpers.
- Specs should prefer user-observable route and DOM state over sleeps.
- Page readiness is detected through `<meta name="ra-idp:page">`, which is set by
  route components after their loaders have completed.
- Tests cover browser integration only. Handler and usecase edge cases stay in
  Go tests unless a browser-specific regression is involved.

## Current coverage

- `authorize-golden-path.spec.ts` covers the OIDC browser authorization golden
  path and the admin shell client-side navigation invariant.
- `ui-scenario-smoke.spec.ts` covers the first wi-75 slice: login assistance
  pages, account portal route reachability, and admin console route reachability.
- `ui-scenario-actions.spec.ts` covers the second wi-75 slice: account profile
  update, account data export trigger, connected application consent revocation,
  TOTP enrollment/removal with step-up, account session revocation, email change
  confirmation through a local SMTP sink, password reset through the same local
  SMTP sink, admin audit filtering and export URL generation, admin tenant user
  attribute schema add/delete, admin application creation/protocol binding/user
  assignment/deletion, admin agent registration and credential binding, admin
  general settings update, and signing key rotation action visibility for tenant
  admins.

## Expansion rules

- Add destructive CRUD E2E only when the browser interaction itself is the risk.
- Reuse `fixtures.ts` for login and route polling instead of open-coded waits.
- Keep TOTP, email confirmation, and reset-link extraction helpers centralized in
  `fixtures.ts` when those scenarios are added.

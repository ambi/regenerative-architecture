# RA Identity UI

The authorization UI of `ra-idp-go` aims to provide a modern, compliant, easy-to-use, and high-visual-quality authentication and identity management experience suitable for enterprise environments.

## Design Guidelines

- **Prioritize Trust**: Establish a secure experience through calm color palettes, clear service identification, and transparent descriptions of current operations and security states, reassuring users during authentication decisions.
- **Present Critical Information First**: Clear information hierarchies display the page title, requesting party, shared information, next action, and cancellation procedures up-front.
- **Simplify Critical Actions**: Limit screens to a single primary action, visually distinguishing it from reject/cancel operations. Avoid modifying OAuth/OIDC form names, submission values, and transition contracts for UI-specific reasons.
- **Accessibility as Standard**: Support keyboard navigation, visible focus indicators, sufficient color contrast, explicit labels, appropriate `aria-*` attributes, and animations respecting reduced-motion preferences.
- **Density for Enterprise Use**: Avoid excessive animations or consumer-focused decorations. Maintain a structured layout using consistent spacing, typography, borders, and state colors.
- **Responsive Without Data Loss**: Display supplementary details on desktops while prioritizing authentication operations on mobile without omitting service identification or safety warnings.
- **Consistency via Shared Components**: Utilize local components conforming to Tailwind CSS, Radix UI, and shadcn/ui. Avoid ad-hoc implementations of colors, border-radii, focus rings, or disabled states.

## Admin Console Policy

The Admin Console's information design is inspired by directory-centric systems like Keycloak, Okta, and Google Cloud IAM. It uses a left-hand navigation sidebar to identify management targets, displays search, status, and major permissions in a high-density table format, and presents detailed views and modification options within the same context. Destructive operations (such as deletion or disabling) are visually separated from standard read-only views.

- **Tables as Command Centers**: Use list views as the primary workspace, allowing users to search, filter, and review status, MFA config, and roles at a glance.
- **Verify Before Modifying**: Enable users to inspect principal IDs, authentication status, and assigned permissions in detail panes before committing changes.
- **Explicit Permission Modification**: Avoid inline editing for sensitive role changes. Use dedicated configuration screens displaying differences (additions/deletions) before confirmation.
- **Visible Danger Actions**: Highlight dangerous actions with clear descriptions and appropriate warning colors to prevent accidental execution.
- **Secure Credentials**: Display client secrets exactly once upon creation. Re-confirm client deletion only after reviewing affected systems.
- **Scalable Architecture**: Structure navigation to accommodate future modules like groups, applications, and audit logs. Unimplemented features must not appear interactive.
- **Consistent Layout**: Maintain a unified structure across the console using `AdminShell` (headers, sidebars, breadcrumbs, content widths, and action placements).
- **Unauthorized Link Fallback**: Redirect unauthenticated direct requests to `/admin/*` to `/login`, returning to the original target destination upon successful login. Allowed redirection targets are constrained to the current realm's `/admin` path.

*References:*
- [Keycloak Server Administration Guide](https://www.keycloak.org/docs/latest/server_admin/)
- [Okta Manage users](https://help.okta.com/en-us/content/topics/users-groups-profiles/usgp-people.htm)
- [Google Cloud IAM access management](https://cloud.google.com/iam/docs/granting-changing-revoking-access)

## UI Library Selection

The UI foundation balances accessibility and design consistency without relying on complex, pre-packaged themes.

| Library | Role | Selection Rationale |
| --- | --- | --- |
| React + TypeScript | UI and type-safe views | Maintains clear component boundaries and state management from simple login screens to the administrative console. |
| Vite | Dev server and production build | Fast, straightforward generation of static bundles that can be served via API gateways or CDNs. |
| Tailwind CSS | Design tokens and styling | Enables consistent styling (states, responsiveness, accessibility) while preserving enterprise branding controls. |
| Radix UI | Accessible headless primitives | Accessible keyboard handling and ARIA compliance decoupled from visual presentation. |
| Local Components (shadcn/ui layout) | Buttons, Inputs, Labels, Cards, Alerts | Maintained within the repository for easy audit and customization, minimizing runtime dependency overhead. |
| TanStack Router | Type-safe routing | Safe translation of page metadata from the Go backend to target UI views. |
| TanStack Table | Administrative data grid | Separates sorting, filtering, and pagination logic from UI presentation. (Reserved for user/client tables; currently unused in the 4 core login screens). |
| Tabler Icons | Vector icons | Consistent line weights and extensive library to serve as visual aids for states and actions rather than mere decoration. |
| Class Variance Authority / Clsx / Tailwind Merge | Class merging | Type-safe styling variants and runtime merging of conflicting Tailwind classes. |
| Biome | Linter and formatter | Rapid automated enforcement of syntax, style, and code quality guidelines. |

Priorities are accessibility, bundle size, maintainability, design ownership, and preserving API contracts. Introduce new libraries only when existing tools cannot satisfy specific requirements.

Refer to [`ARCHITECTURE.md`](ARCHITECTURE.md) for details on deployment boundaries, authorization transactions, cookies, and CSRF protection.

## Implementation Guidelines

When modifying the UI, run the following verification checks:
```bash
bun run lint
bun run typecheck
bun run build
```

When API contracts are modified, run Go HTTP E2E tests to verify that cookies, CSRF protection, OAuth redirects, and JSON schemas remain correct.

While the Vite CLI utilizes `#!/usr/bin/env node`, dev and build scripts execute JS entries directly via `bun`. This unifies running processes under `bun .../vite.js` without requiring a Node.js runtime.

## E2E Smoke Tests

Verify the SPA's golden path (`/authorize → login → consent → callback`) by running:
```bash
bun run test:e2e
```

The runner uses `bun test` and the built-in `Bun.WebView` (using WKWebView on macOS and Chrome via CDP on Linux/Windows), eliminating the need for heavy browser automation frameworks or manual driver downloads.

The test suite (`tests/e2e/`) automatically manages the lifecycle of:
1. **Go API**: Starts in `memory` mode on port `:8081` (`ADDR=:8081 ISSUER=http://localhost:5173`) to match the browser origin and pass CSRF checks.
2. **Vite Dev Server**: Starts on port `:5173`, proxying `/authorize` and `/api` requests to `8081`.
3. **Mock Callback Server**: Starts on port `:3000` to receive the auth code at `demo-client`'s `redirect_uri` (`http://localhost:3000/callback`).

This setup validates client routing (`meta[name="ra-idp:page"]`) and ensures that `code` and `iss` parameters are preserved during cross-origin redirects (RFC 9207). Requires only `go` and `bun` in your `PATH`.

# scl-to-html

A utility to bundle Regenerative Architecture (RA) documentation into a single HTML file. In addition to SCL (`spec/scl.yaml`), it integrates CONCEPTION / ADR (`decisions/*.md`) and Work Items (`work-items/*.yaml`) into a single view.

For specifications, refer to [`spec/scl.yaml`](spec/scl.yaml).

## Usage

```bash
bun scl-to-html \
  --scl       ../ra-idp-go/spec/scl.yaml \
  --title     "ra-idp" \
  --out       out/ra-idp.html
```

- Only `--scl` is required. If `--decisions` or `--work-items` are omitted, their corresponding tabs will be empty.
- If `--out` is omitted, the HTML is written directly to stdout.
- If `--title` is omitted, the `system` field from the SCL is used as the page title.
- For standard reviews, only `--scl` is used. Specify `--decisions` or `--work-items` when generating a full audit-ready bundle.

Running with `--help` prints the CLI usage guide.

## Output

A single HTML file containing all CSS, JS, and text inline (zero external dependencies). Follows system dark/light preferences via `prefers-color-scheme`.

Tabs:

| Tab | Contents |
| --- | --- |
| SCL | Card-based layout of the 12 sections defined in `SPECIFICATION_CORE_LANGUAGE.md` §3 |
| Decisions | CONCEPTION, CONCEPTION_BASELINE, and ADR-NNN-...md (if specified) |
| Work Items | work-items/*.yaml and work-items/done/*.yaml (if specified) |

URL hashes route via `#tab=<name>&sec=<section-id>`. If JavaScript is disabled, the page falls back to a degraded mode where tab contents are listed vertically.

## Relationship with JSON Schema

- SCL Schema: [`tools/yaml-check/schemas/scl.schema.json`](../yaml-check/schemas/scl.schema.json)
- Work-Item Schema: located under `yaml-check/schemas/`
- Run `bun --cwd ../yaml-check yaml-check:all` to validate all three formats concurrently.

Properties not restricted by the schema are ignored and passed through by this tool, ensuring that adding custom/unknown fields to SCL will not break rendering.

## Development

```bash
bun test                  # Run unit tests (Bun test runner)
bun run lint              # Lint code (Biome)
bun run typecheck         # Type check (tsc --noEmit)
bun scl-to-html --help    # Verify CLI help
bun run scl-to-html:ra-idp-go       # Generate ra-idp-go SCL HTML
bun run scl-to-html:ra-idp-go:full  # Generate full HTML bundle containing ADRs / Work Items
```

Source Tree:

```
src/
  main.ts              # CLI entry point (Parse arguments -> load -> render -> write)
  args.ts              # CLI argument parser (Pure functions)
  load.ts              # File loader for SCL, Decisions, and Work Items
  types.ts             # TypeScript definitions for SCL, Decisions, and Work Items
  html.ts              # HTML utilities (escaping, slugification, chip/badge rendering)
  markdown.ts          # markdown-it wrapper (html: false)
  render-scl.ts        # Render SCL sections to HTML
  render-decisions.ts  # Render conceptions and ADRs to HTML
  render-changes.ts    # Render work items to HTML
  page.ts              # Document wrapper containing layout shell, CSS, and JS
spec/scl.yaml          # Specifications for this tool itself
schemas/               # None (SCL schemas are stored in ../yaml-check/schemas/)
```

## Why it was rewritten

The legacy version was a single `render.ts` file (~1800 lines) that could only render SCL. To meet operational requirements for inspecting SCL, ADRs, and Work Items side-by-side, the code was refactored into modules, and the Decisions and Work Items tabs were added. For details, refer to this tool's SCL spec at [`spec/scl.yaml`](spec/scl.yaml) instead of the `CONCEPTION.md` series in `ra-idp-go`.

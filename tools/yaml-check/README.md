# yaml-check

A three-tier YAML validation CLI for the repository:

1. **Parse**: Syntactic validation using Bun's built-in YAML loader. The same engine is used by downstream utilities like `scl-to-html`, ensuring that files passing this step are compatible elsewhere.
2. **Lint**: Structural raw-text checks (checking for tab indentation, trailing spaces, and trailing newlines).
3. **Schema (Optional)**: If `--schema=<name>` is passed, validates content using JSON Schema 2020-12 (via Ajv). Schema-to-file mappings are not auto-guessed from filenames; they must be explicitly declared to prevent applying mismatching schemas.

For specifications, refer to [`spec/scl.yaml`](spec/scl.yaml).

## Usage

```bash
bun yaml-check <file-or-glob>...                          # Parse + Lint
bun yaml-check --schema=<name> <file-or-glob>...          # Parse + Lint + Schema validation
bun yaml-check --list-schemas                             # Print available schemas line-by-line
bun yaml-check --help
```

### Packaged Schemas

| Name | Targets | Origin |
| --- | --- | --- |
| `work-item` | `*/work-items/<id>.yaml` (completed items under `done/`) | `REGENERATIVE_ARCHITECTURE.md` §4.2 |
| `scl` | `*/spec/scl.yaml` | `SPECIFICATION_CORE_LANGUAGE.md` §2–§3 |

If an unknown schema name is supplied, the tool terminates with exit code 2 without running validations.

### NPM Scripts (CI / Automation)

Defined in `tools/package.json`:

```bash
bun run yaml-check:work-items          # Validates */work-items/*.yaml and */work-items/done/*.yaml
bun run yaml-check:scl                 # Validates context SCLs in ra-idp-go and tool SCLs
bun run yaml-check:all                 # Validates both scopes sequentially
```

## Exit Codes

| Code | Meaning |
| --- | --- |
| 0 | All checked files are valid |
| 1 | One or more findings detected (parsing, linting, or schema validation failure) |
| 2 | Incorrect usage (unknown flags or unknown schema names) |

## Finding Format

```
<path>:<line>:<column>: <message>
```

Schema validation errors use heuristics to map Ajv JSON Pointers (e.g. `/scope/ui/pages/0`) to approximate source file lines. Fails back to line 1, column 1 if resolution fails.

## Development

```bash
bun test                  # Run unit tests (Bun test runner)
bun run lint              # Lint code (Biome)
bun run typecheck         # Type check (tsc --noEmit)
bun run yaml-check:all    # Validate all repository files against schemas
```

Core logic resides in [`src/lib.ts`](src/lib.ts), CLI side effects are limited to [`src/main.ts`](src/main.ts). Tests are implemented in `src/lib.test.ts`. JSON Schemas are stored in the [`schemas/`](schemas/) directory.

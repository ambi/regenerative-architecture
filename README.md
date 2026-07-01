# Regenerative Architecture

This repository is a workspace containing the definition of **Regenerative Architecture (RA)**, a software design and development methodology, along with a concrete implementation showcasing its concepts.

The core philosophy of RA is to avoid treating implementations and runtimes as fixed, precious assets. Instead, it prioritizes archiving **declarative specifications, architectural decisions, and verifiable change records**, allowing the actual code and environments to be regenerated when needed. For the detailed philosophy and theory, see [REGENERATIVE_ARCHITECTURE.md](REGENERATIVE_ARCHITECTURE.md).

## Repository Overview

```text
REGENERATIVE_ARCHITECTURE.md   Philosophy and the 5-layer structure of RA
SPECIFICATION_CORE_LANGUAGE.md Definition of Specification Core Language (SCL)
CHANGE_RECORD_FORMAT.md        Canonical formats for Work Items and ADRs
ra-idp-go/                     An Identity Provider (IdP) implementation built using RA
tools/                         Tools for SCL/Work Item verification and code generation
work-items/                    Repository-wide change management records (Work Items)
justfile                       Central command map executed from the repository root
```

## Recommended Reading Order

If you are new to this repository, read these documents in the following order:

1. [REGENERATIVE_ARCHITECTURE.md](REGENERATIVE_ARCHITECTURE.md): The core philosophy of what to archive and what to regenerate.
2. [SPECIFICATION_CORE_LANGUAGE.md](SPECIFICATION_CORE_LANGUAGE.md): How to write the specification core (SCL) and make it verifiable.
3. [ra-idp-go/README.md](ra-idp-go/README.md): Architecture and setup of the reference Identity Provider (IdP) implementation.
4. [tools/README.md](tools/README.md): Tooling used to validate specifications, check change records, and generate downstream artifacts.

> [!IMPORTANT]
> When creating a new Work Item or Architecture Decision Record (ADR), do not copy or guess formats from existing files. Always refer to [CHANGE_RECORD_FORMAT.md](CHANGE_RECORD_FORMAT.md) as the single source of truth for formats.

## Development Entrypoints

To avoid directory confusion in this monorepo, run `just` commands from the repository root. Each recipe is intent-based and navigates to the correct directory automatically.

### Setup and Verification

Show available commands:
```bash
just
```

Install all dependencies (requires Bun and Go):
```bash
just setup
```

Run the complete verification suite (linters, tests, type-checks, YAML validations):
```bash
just verify
```

### Specialized Commands

| Command | Description |
| --- | --- |
| `just verify-go` | Run linter and race-enabled tests for the Go backend |
| `just verify-ui` | Check formatting, lint, type-check, and build the React UI |
| `just verify-tools` | Run type-checks, linters, tests for tools, and validate YAML/SCL files |
| `just yaml-check` | Validate all Work Item and SCL YAML files |
| `just scl-render` | Regenerate HTML views, JSON Schema, and OpenAPI specs from SCL |
| `just dev-api` | Start the Go API development server |
| `just dev-ui` | Start the React UI development server |
| `just dev-compose` | Start the local development stack via Docker Compose |

## Workflow Rules for Changes

- **SCL-First for Feature Work**: When changing features or behaviors, always review and update the Specification Core Language (SCL) in `spec/scl.yaml` first, before modifying the implementation.
- **Record Decisions in ADRs**: Document important design decisions, including their rationale and rejected alternatives, in Architecture Decision Records (ADRs).
- **Trace Changes via Work Items**: Treat every logical change as a "Work Item" and record its context, scope, verification steps, and residual risks in a dedicated work item file under `work-items/`.
- **Regenerate Derived Artifacts**: If a change affects the specification, run `just scl-render` to regenerate downstream artifacts and include the regeneration commands and results in your commit.
- **Verify Local Status**: After any modification, run the appropriate `just verify-*` command depending on the change scope. When in doubt, run `just verify`.

## Context for AI Agents (LLM Instructions)

Before executing tasks in this repository, read [REGENERATIVE_ARCHITECTURE.md](REGENERATIVE_ARCHITECTURE.md) and [SPECIFICATION_CORE_LANGUAGE.md](SPECIFICATION_CORE_LANGUAGE.md). Keep feature modifications SCL-first. When creating new Work Items or ADRs, strictly follow the templates in [CHANGE_RECORD_FORMAT.md](CHANGE_RECORD_FORMAT.md).

Keep the root `README.md` minimal as an entry point. Detailed specifications, design decisions, verification runbooks, and context-specific implementation details must reside in the corresponding SCL files, ADRs, Work Items, or sub-directory READMEs.


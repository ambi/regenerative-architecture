# CLAUDE.md / AGENTS.md

## 1. Regenerative Architecture

**Design and implement things according to Regenerative Architecture.**

- Read `REGENERATIVE_ARCHITECTURE.md` and `SPECIFICATION_CORE_LANGUAGE.md`.
- Follow them.

## 2. Commit Hygiene

**Conventional Commits, English.**

- Write commit messages in English following the Conventional Commits format:
  `type(scope): summary` (e.g. `feat(ra-idp): add Argon2id password hasher`).
- Use `type` from the standard set: `feat`, `fix`, `docs`, `refactor`, `chore`,
  `test`, `perf`, `build`, `ci`, `style`. Append `!` for breaking changes.
- Keep the subject line ≤ 72 chars. Use the body for the *why*, not the *what*.

## 3. Default Tooling

**Prefer the team's default stack; override only when a task or existing code demands it.**

| Prefer | Over |
| --- | --- |
| `rg` | `grep` |
| `fd` | `find` |
| `bat` | `cat` |
| `jq` | ad-hoc JSON parsing |
| TypeScript | Python |
| Bun | Node.js |
| Go | Rust |
| React | Vue, Next.js |
| Postgres | MySQL |
| `golangci-lint` | `go vet` |
| `biome` | `eslint` |
| `tools/yaml-check` | Python/Ruby YAML parser |

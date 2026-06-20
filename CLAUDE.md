# CLAUDE.md / AGENTS.md

## 0. Interaction Language

**User-facing messages must be in Japanese.**

- Reply to the user in Japanese by default.
- Status updates, explanations, questions, summaries, and final responses must be
  Japanese.
- Keep commit messages, branch names, code identifiers, logs, command output, and
  quoted upstream text in their original or required language.
- If a tool, API, or external specification requires English, use English only
  for that artifact and explain it in Japanese.

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
| TypeScript | Python |
| Bun | Node.js |
| Go | Rust |
| React | Vue, Next.js |
| Tailwind CSS + Radix UI + shadcn/ui | Bootstrap, Bulma, MUI, Chakra UI |
| PostgreSQL | MySQL |
| Valkey | Redis |
| `rg` | `grep` |
| `fd` | `find` |
| `bat` | `cat` |
| `jq` | ad-hoc JSON parsing |
| `golangci-lint run` | `go vet` |
| `golangci-lint fmt` | `go fmt`, `gofmt` |
| `biome` | `eslint` |
| `tools/yaml-check` | Python/Ruby YAML parser |

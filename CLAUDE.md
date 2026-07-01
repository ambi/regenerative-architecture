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

- `REGENERATIVE_ARCHITECTURE.md` (the method: 5 layers + derivation rules) and
  `SPECIFICATION_CORE_LANGUAGE.md` (the SCL meta-spec: notation) are
  **section-addressable references, not required reading.** Do not load either in
  full. Locate the section you need with `rg '^#{2,3} ' <file>` and read only that
  line range.
- The operational essentials are already distilled into the Skills
  (`scl-change`, `implement-work-item`, `new-work-item`, `new-adr`, `scl-render`,
  `commit`) and this file. Reach for a Skill first; open a meta-doc section only for a detail
  a Skill does not cover.
- Section map (jump here instead of scanning):
  - RA: §2 derivation rules · §3 the 5 layers (1 Spec Core → 2 Decision Record →
    3 App Logic → 4 Adapter → 5 Runtime) · §4 conception → work-item → dev flow.
  - SCL: §3.1 glossary · 3.2 models · 3.3 interfaces · 3.4 states · 3.5 invariants ·
    3.6 scenarios · 3.7 permissions · 3.8 objectives; §4 types · §5 expressions
    (CEL subset).

## 2. Default Tooling

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

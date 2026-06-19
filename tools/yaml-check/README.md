# yaml-check

リポジトリ内の YAML を 3 段で検査する CLI:

1. **Parse** — Bun 内蔵の YAML ローダで構文解析する。`scl-to-html` 等の他ツールが
   使うのと同じエンジンなので、ここで通れば下流でも通る。
2. **Lint** — 生テキストに対する形式チェック (タブインデント / 行末空白 /
   末尾改行)。意味解釈は行わない。
3. **Schema** (任意) — `--schema=<name>` を渡したときだけ、JSON Schema 2020-12
   ベース (Ajv) で内容を検証する。スキーマはファイル名から推測しない —
   偶然同名のファイルに無関係なスキーマが当たることを避けるため、常に明示する。

仕様は [`spec/scl.yaml`](spec/scl.yaml) を参照。

## 使い方

```bash
bun yaml-check <file-or-glob>...                          # parse + lint
bun yaml-check --schema=<name> <file-or-glob>...          # parse + lint + schema
bun yaml-check --list-schemas                             # スキーマ名を 1 行ずつ
bun yaml-check --help
```

### 同梱スキーマ

| 名前        | 対象                          | 由来                                |
| ----------- | ----------------------------- | ----------------------------------- |
| `work-item` | `*/work-items/<id>.yaml`      | `REGENERATIVE_ARCHITECTURE.md` §4.2 |
| `scl`       | `*/spec/scl.yaml`             | `SPECIFICATION_CORE_LANGUAGE.md` §2–§3 |

未知のスキーマ名を渡すと、ファイル検査を一切行わずに exit code 2 を返す。

### npm scripts (CI 用)

`tools/package.json` から:

```bash
bun run yaml-check:work-items          # */work-items/*.yaml
bun run yaml-check:scl                 # ra-idp-go / scl-to-html / yaml-check の SCL 3 本
bun run yaml-check:all                 # 上 2 つを直列で
```

## Exit code

| code | 意味                                        |
| ---- | ------------------------------------------- |
| 0    | 全ファイルが ok                             |
| 1    | 1 件以上の Finding (parse / lint / schema)  |
| 2    | 使い方の誤り (未知フラグ / 未知スキーマ名)  |

## Finding の形式

```
<path>:<line>:<column>: <message>
```

スキーマ違反は Ajv の JSON Pointer (`/scope/ui/pages/0` など) をソース上の行に
ヒューリスティックで解決して出力する。解決できないときは行 1 / 列 1 になる。

## 開発

```bash
bun test                  # lib.test.ts のユニットテスト (Bun test runner)
bun run lint              # Biome
bun run typecheck         # tsc --noEmit
bun run yaml-check:all    # スキーマ込みの全件検査
```

純粋ロジックは [`src/lib.ts`](src/lib.ts) に集約され、CLI 副作用は
[`src/main.ts`](src/main.ts) のみが持つ。テストは `src/lib.test.ts`。
スキーマは [`schemas/`](schemas/) 配下の JSON。

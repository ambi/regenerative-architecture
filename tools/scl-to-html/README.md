# scl-to-html

Regenerative Architecture (RA) ドキュメント群を 1 枚の HTML に束ねるツール。
SCL (`spec/scl.yaml`) だけでなく、CONCEPTION / ADR (`decisions/*.md`) と
work item (`work-items/*.yaml`) も同じページから辿れる。

仕様は [`spec/scl.yaml`](spec/scl.yaml) を参照。

## 使い方

```bash
bun scl-to-html \
  --scl       ../ra-idp-go/spec/scl.yaml \
  --title     "ra-idp" \
  --out       out/ra-idp.html
```

- `--scl` のみ必須。`--decisions` / `--work-items` 省略時はそのタブが空表示になる。
- `--out` を省略すると HTML が標準出力に書き出される。
- `--title` を省略すると `system` フィールド (SCL) がページタイトルになる。
- 通常のレビュー用 HTML は SCL のみを出す。ADR / work item まで含む監査用の全部入り HTML
  が必要な場合だけ `--decisions` / `--work-items` を指定する。

`--help` で同じ案内が出る。

## 出力

単一の HTML ファイル。CSS / JS / すべてのテキストは inline 同梱 (外部リソース 0)。
ライト / ダーク両対応 (`prefers-color-scheme` に追従)。

タブ:

| タブ      | 内容                                                             |
| --------- | ---------------------------------------------------------------- |
| SCL       | SPECIFICATION_CORE_LANGUAGE.md §3 の 12 セクションをカード表示   |
| Decisions | CONCEPTION + CONCEPTION_BASELINE + ADR-NNN-…md (指定時のみ)      |
| Work Items | work-items/*.yaml + work-items/done/*.yaml (指定時のみ)         |

URL ハッシュは `#tab=<name>&sec=<section-id>` 形式でルーティング。JS 無効でも
生成されたタブの内容が縦に並んで読める (degraded mode)。

## JSON Schema との関係

- SCL は [`tools/yaml-check/schemas/scl.schema.json`](../yaml-check/schemas/scl.schema.json)
- work-item は同 `yaml-check/schemas/`
- `bun --cwd ../yaml-check yaml-check:all` で 3 形式まとめて検証できる

スキーマで弾かれない (= 任意プロパティが許される) 部分は本ツールが受け流すので、
SCL の側で未知フィールドを足しても壊れない。

## 開発

```bash
bun test                  # ユニットテスト (Bun test runner)
bun run lint              # Biome
bun run typecheck         # tsc --noEmit
bun scl-to-html --help    # CLI 確認
bun run scl-to-html:ra-idp-go       # ra-idp-go の SCL HTML
bun run scl-to-html:ra-idp-go:full  # ADR / work item も含む全部入り HTML
```

ソース構成:

```
src/
  main.ts              # CLI shell (引数 → load → render → write)
  args.ts              # CLI 引数パーサ (純関数)
  load.ts              # SCL / decisions / work-items ローダ (file IO のみ)
  types.ts             # SCL + Decision + Change の TS 型
  html.ts              # esc / slug / chip / link / badge / renderValue
  markdown.ts          # markdown-it ラッパ (html: false)
  render-scl.ts        # SCL の 12 セクションを HTML 化
  render-decisions.ts  # CONCEPTION + ADR の HTML 化
  render-changes.ts    # work-item の HTML 化
  page.ts              # トップシェル (タブバー / サイドバー / CSS / JS)
spec/scl.yaml          # 本ツール自身の RA 仕様
schemas/               # (なし — SCL スキーマは ../yaml-check/schemas/)
```

## なぜリライトしたか

旧版は `render.ts` 1 ファイル ~1800 行で、SCL しか描画できなかった。Phase 4
以降「SCL / ADR / ワークアイテム を一緒に読みたい」という運用上の要請が増え、
モジュール分割と Decisions / Work Items タブを追加した。詳細は
[CONCEPTION](../../ra-idp-go/decisions/CONCEPTION.md) 系の文書
ではなく本ツールの SCL (`spec/scl.yaml`) を参照。

---
name: scl-render
description: Regenerate SCL-derived artifacts (HTML views, JSON Schema, OpenAPI) after editing scl.yaml or spec/contexts. Use when scl.yaml changed and downstream artifacts may be stale, or when the user asks to render/regenerate the spec.
---

# SCL 派生物の再生成

scl.yaml が「単一上流」、HTML / JSON Schema / OpenAPI はその「下流」。上流を触ったら
下流を再生成して drift を残さない。

## まず検証

```sh
cd tools && bun run yaml-check:scl
```

## 一括再生成（推奨）

リポジトリルートから:

```sh
just scl-render
```

これは内部で以下を実行する:

- `bun run scl-to-html:ra-idp-go`（spec の HTML ビュー）
- `bun run scl-to-html:self`（scl-to-html 自身の仕様 HTML）
- `bun run scl-to-jsonschema:ra-idp-go`（models の JSON Schema）
- `bun run scl-to-openapi:ra-idp-go`（OpenAPI）

## 個別実行（tools ディレクトリから）

```sh
cd tools
bun run scl-to-html:ra-idp-go          # ../ra-idp-go/spec/ra-idp-go.html
bun run scl-to-html:ra-idp-go:full     # work-items/decisions も束ねた full ビュー
bun run scl-to-jsonschema:ra-idp-go    # ../ra-idp-go/spec/ra-idp-go.models.schema.json
bun run scl-to-openapi:ra-idp-go       # ../ra-idp-go/spec/ra-idp-go.openapi.json
```

## 仕上げ

再生成された成果物の diff を確認し、SCL の変更意図と一致しているかを見る。
commit するときは生成物も同じ commit に含める（spec と生成物の同期を保つ）。

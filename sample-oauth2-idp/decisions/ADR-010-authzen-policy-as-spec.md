# ADR-010: 認可ポリシーを仕様核に置き、AuthZEN スタイルのインターフェースで評価する

## ステータス

採用

## コンテキスト

OAuth2 / OIDC IdP には複数種類の認可判断が散在する:

- 認可エンドポイントでクライアントとリダイレクト URI を検証
- トークンエンドポイントでクライアントが宣言したグラントタイプを保持しているか確認
- リフレッシュトークンが失効していないか、絶対 TTL を超えていないか
- センダー制約 (DPoP/mTLS) と所有証明の整合
- UserInfo エンドポイントの scope チェック
- /introspect 呼び出し元がリソースサーバーとして認証されているか

これらをアダプター層に `if` で散在させると、再生成時に脱落のリスクがある。
Regenerative Architecture が「セキュリティポリシーは仕様核に置く」と主張する所以である。

## 決定

`spec/policy/client-authorization.json` に、全アクションと判定ルールを宣言的に記述する。
評価は AuthZEN（OpenID Foundation Authorization API 仕様）スタイルの
`{ subject, action, resource, context }` インターフェースで行う。

レイヤー構成:

**仕様核 (spec/policy/)**:

- `client-authorization.json` — アクションと判定ルールの宣言（唯一の権威）
- `client-authorization.ts`   — TypeScript 評価アダプター（純粋関数）
- `authorization.test.ts`     — ポリシー単体テスト

**アダプター層 (adapters/policy/)**:

- `local-authzen-adapter.ts`  — `authorize()` 関数を提供
  - 現在: 仕様核の `evaluate()` を直接呼ぶ
  - 将来: 外部の AuthZEN サービス、OPA、Cedar への HTTP 呼び出しに差し替え可能

ユースケース層は `authorize({ subject, action, resource, context })` を呼ぶだけで、
背後の評価エンジンを知らない。

## ルール ID の網羅性をテストで保証

`client-authorization.json` の各アクションには複数の `rules` が宣言される。
それぞれの `rules[].id` は TypeScript 側の `ruleEvaluators` の同名キーで実装される。

未実装ルールが残っていることを `spec/invariants.test.ts` が検知する:

```ts
it('JSON 側で言及されたすべての rule.id が TypeScript 側に実装されている', () => {
  const missing = ALL_RULE_IDS.filter(id => !IMPLEMENTED_RULE_IDS.includes(id))
  expect(missing).toEqual([])
})
```

これにより、JSON に新規ルールを追加したのにコードに反映し忘れることを防ぐ。

## ポリシー言語の選択

本サンプルでは JSON + TypeScript 純粋関数を採用した。
理由はサンプルとしての明快さ・外部ランタイム依存なし・即時テスト可能性。

本番候補:

- **Cedar (AWS)**: 形式検証済み。FAPI 系の規制産業に最適
- **OPA Rego**: 汎用エンジン、Kubernetes 統合
- **AuthZEN リモートサービス**: 認可サービスを共有インフラ化

これらへの移行は `adapters/policy/local-authzen-adapter.ts` のみ変更すればよい。

## 却下した代替案

- **HTTP ルーターに `if` で散在**: 再生成リスク・監査困難
- **OPA Rego を仕様核に直接**: language-agnostic ではあるが可読性が低く、
  ビジネスステークホルダーが読めない
- **Cedar を仕様核に直接**: 良い候補だが、ランタイム依存（AWS）が発生する。
  今回は中立な JSON + 純粋関数を選び、Cedar 移行は ADR を分岐させる前提とする

## 影響

- 認可ルールの変更は `spec/policy/client-authorization.json` のみで完結する
- HTTP / トークン / UserInfo / Introspection の各ルーターは `authorize()` を呼ぶだけ
- ポリシー変更後は `authorization.test.ts` の全テストが通ることを必ず確認する
- `spec/invariants.test.ts` の網羅性テストで、ルールの実装漏れを防ぐ

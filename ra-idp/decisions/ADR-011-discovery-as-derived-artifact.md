# ADR-011: Discovery 文書はルーティングと grant matrix から導出され、独立に編集しない

## ステータス

採用

## コンテキスト

OAuth 2.0 Authorization Server Metadata (RFC 8414) と OIDC Discovery (Discovery 1.0) は、
クライアントが IdP の能力を自動発見するための重要なメタデータである。
これが現実と乖離すると、クライアントの自動構成が壊れる。

実装上の罠:

- Discovery 文書を「文書として」手書きで保守する → grant_types_supported と
  実装が乖離するバグが頻発
- 各エンドポイントの実装を追加・削除しても Discovery に反映され忘れる
- 鍵アルゴリズムを変更しても `id_token_signing_alg_values_supported` が古いまま

## 決定

Discovery 文書を「導出された成果物」として扱う。具体的には:

1. **仕様核に Discovery テンプレートを置く** (`spec/discovery.json`):
   - issuer プレースホルダーを `{{ISSUER}}` で表記
   - 全エンドポイントパス・サポートアルゴリズム・サポートスコープを宣言

2. **テンプレートの内容は他の仕様核ファイルと整合**する:
   - `grant_types_supported` ↔ `grants/grant-types.json`
   - `token_endpoint_auth_methods_supported` ↔ `grants/grant-types.json`
   - `id_token_signing_alg_values_supported` ↔ `tokens/id-token.schema.json` の `x-signature-algorithms`
   - `response_types_supported` ↔ `grants/grant-types.json` の `supported_response_types`
   - `code_challenge_methods_supported` ↔ ADR-002

   整合は `spec/invariants.test.ts` の「Discovery — Grant matrix との整合」スイートで
   機械的に保証する。

3. **アダプター層は Discovery テンプレートを読み、`{{ISSUER}}` を置換して返す** だけ。
   個別フィールドをアダプターで書き換えない。新規エンドポイントを追加する場合は、
   仕様核（テンプレート + ルーティング表）から修正する。

## なぜ「導出されたファイル」を生成しないか

選択肢として「ビルド時に Discovery JSON を生成する」もあるが:

- 生成済みファイルをコミットすると、テンプレートと生成物の二重管理になる
- ビルド忘れによる現実との乖離が再発生する

代わりに、**ランタイムでテンプレートを読んで置換する** ことで、
「仕様核に書かれた内容がそのまま配信される」状態を保つ。

## 整合性検証

仕様核内の複数ファイルが同じ事実を述べる以上、機械的な整合性検証が必須:

```ts
it('discovery.grant_types_supported が grant-types.json と一致する', () => {
  expect([...discoverySpec.grant_types_supported].sort()).toEqual(
    [...SUPPORTED_GRANT_TYPES].sort(),
  )
})
```

これにより、片方を変更してもう片方を忘れることを防ぐ。

## 却下した代替案

- **Discovery を完全自動生成**: 上述のとおり二重管理になる
- **手書きで保守**: 乖離リスクが高い
- **コードジェネレーターで型を生成**: 「JSON が権威」原則に反する。型生成は補助であり、
  権威ではない

## 影響

- 新規エンドポイント追加時は `spec/discovery.json` を必ず更新する
- 新規アルゴリズムサポート時は `spec/discovery.json` と `spec/tokens/*.schema.json` の両方を更新
- `spec/invariants.test.ts` の Discovery 整合性テストを CI で常時実行する

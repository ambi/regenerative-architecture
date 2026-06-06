# ADR-006: PAR を FAPI クライアントに必須とし、一般クライアントにはオプションで提供する

## ステータス

採用

## コンテキスト

`/authorize` への直接アクセスは、認可リクエストパラメータがブラウザのアドレスバーを通る。
これは以下のリスクを生む:

- パラメータの改ざん（オープンリダイレクト・CSRF）
- 長大化したクエリ文字列（JAR / リッチ認可リクエスト等）の URL 上限
- リクエスト整合性の事前検証ができない（クライアント認証なしで /authorize に到達する）

Pushed Authorization Requests (RFC 9126) は、これらに対処するため
**先にバックチャネルでクライアント認証付きで認可パラメータを送る** という設計を導入する。
`/par` が `request_uri` を返し、`/authorize` ではこれを参照するだけになる。

FAPI 2.0 §5.2 は PAR を MUST とする。

## 決定

本アプリ IdP は以下を採用する:

1. すべてのクライアントに対して `/par` エンドポイントを提供する
2. クライアントメタデータの `require_pushed_authorization_requests = true` を
   宣言したクライアント（FAPI プロファイル含む）は、PAR 必須
3. それ以外のクライアントは PAR / 通常 `/authorize` の両方を選択できる
4. `request_uri` の TTL は 600 秒以下、**単一使用**

## 単一使用と短寿命の理由

- 攻撃者が `request_uri` を盗んでも、すでに使用済みなら再利用できない
- 失効した `request_uri` を保持し続けない（メモリ・ストレージのコスト管理）
- 600 秒は「ユーザーが認証で時間を要する」現実的なケースをカバー
- FAPI 2.0 推奨値も同程度

## 認可リクエストのマージ規則

`/authorize?request_uri=...` で `request_uri` を参照したリクエストに、追加の
クエリパラメータが付随した場合、**保存されたパラメータを優先し、追加パラメータは無視** する。
これは RFC 9126 §4 に従う動作。攻撃者が `request_uri` に追加パラメータを連結して
リクエストを改竄することを防ぐ。

## 却下した代替案

- **JAR (RFC 9101) のみ**: JAR は同等の防御を提供するが、認可リクエストを JWT に
  封入するためクライアント実装の負荷が高い。PAR のほうがエコシステム採用が進んでいる
- **PAR を全クライアントに必須**: 既存クライアントの移行コストが高い。FAPI のみ必須化
- **request_uri を再利用可能に**: 攻撃時のウィンドウが広がる

## 影響

- `adapters/persistence/in-memory-par-store.ts` を実装する
- `policy/client-authorization.json` の `authorize:initiate.rules` に
  `par_required_if_fapi` を含める
- `spec/slo.yaml` の `par_request_uri_ttl_seconds` で TTL を制御
- Discovery `pushed_authorization_request_endpoint` と `require_pushed_authorization_requests`
  を反映する

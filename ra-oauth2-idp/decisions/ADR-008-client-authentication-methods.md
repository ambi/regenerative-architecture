# ADR-008: クライアント認証方式を 5 種類サポートし、推奨優先順位を明示する

## ステータス

採用

## コンテキスト

OAuth 2.0 はクライアント認証方式を複数定義しており、それぞれセキュリティ強度と運用コストが
異なる。すべてを実装するか、一部に絞るかの判断が必要だった。

業界調査:

- 大半の SaaS IdP は `client_secret_basic` と `client_secret_post` を実装する
- FAPI は `private_key_jwt` と `tls_client_auth` を強く推奨
- public クライアントは `none` のみ（PKCE 必須）

## 決定

以下の 5 方式をすべてサポートする:

| 方式                   | 用途                             | 推奨度       |
|-----------------------|----------------------------------|--------------|
| `private_key_jwt`     | 一般的な confidential クライアント | ★★★ 推奨    |
| `tls_client_auth`     | FAPI / B2B (mTLS PKI を持つ組織) | ★★★ 推奨    |
| `none`                | public クライアント (SPA/native) | ★★★ 必須    |
| `client_secret_post`  | レガシー confidential 移行用     | ★★ 許容    |
| `client_secret_basic` | レガシー confidential 移行用     | ★ 非推奨   |

## なぜ `client_secret_*` を残すか

- 既存クライアントの段階的移行のため。すべてを `private_key_jwt` に移行するには
  数年単位の運用変更が必要
- ただし新規クライアントには `private_key_jwt` または `tls_client_auth` を強く推奨する

## なぜ `client_secret_jwt` を実装しないか

`client_secret_jwt` は HMAC-SHA256 を共有鍵で行う方式。
非対称署名のほうが鍵漏洩時の被害が小さく、運用上の利点も大きい。
`private_key_jwt` を採用するなら `client_secret_jwt` は冗長なので実装しない。

## 認証失敗時の応答ポリシー

クライアント認証が失敗したときは:

- HTTP `401`
- `error: invalid_client`
- **client_id が登録されているかどうかを開示しない**

これは `requirements.md §8` で EARS として記述する。
タイミング攻撃を防ぐため、未登録 client_id でも認証検証のラウンドトリップを
等価な時間かけて実行する（実装メモ）。

## 却下した代替案

- **すべてを `private_key_jwt` に強制**: レガシー移行が非現実的
- **`client_secret_basic` のみ**: FAPI 要件を満たさない
- **新たな独自方式**: 標準に従わない設計は将来の再生成で破綻する

## 影響

- `client.schema.json` の `token_endpoint_auth_method` enum がこの 5 種類
- Discovery `token_endpoint_auth_methods_supported` がこの 5 種類
- `adapters/http/client-authentication.ts` が認証ロジックを実装する
  （本アプリでは `client_secret_basic` / `client_secret_post` / `none` /
  `private_key_jwt` を実装。`private_key_jwt` の検証規則は ADR-023 を参照。
  `tls_client_auth` はランタイムの mTLS 終端に依存するため枠組みのみ）

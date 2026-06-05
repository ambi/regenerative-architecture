# ADR-003: JWT 署名アルゴリズムを PS256 / ES256 に限定する

## ステータス

採用

## コンテキスト

JWT には複数の署名アルゴリズムが定義されているが、それぞれに既知のリスクがある:

- `none`           — 署名なし。**絶対に許可してはならない**。アルゴリズム混乱攻撃の代表例
- `HS256` (HMAC)   — 対称鍵。JWKS 経由で配布できないため、リソースサーバーが多い OIDC では不適
- `RS256` (RSA-PKCS1v1.5) — 広く使われるが、padding oracle の理論的リスク
- `PS256` (RSA-PSS) — RSA だが PSS padding。FAPI 推奨
- `ES256` (ECDSA P-256) — 短い署名、高速、FAPI 推奨
- `EdDSA`          — まだ JWT エコシステムで完全には浸透していない

FAPI 2.0 §5.7 は **PS256 / ES256 / EdDSA のみ** を許可している。

## 決定

本サンプル IdP は以下の方針を採用する:

1. アクセストークン・ID トークンの署名は **PS256 または ES256** のみ
2. デフォルトは **PS256**（RSA 鍵が運用上扱いやすいため）
3. クライアントメタデータの `id_token_signed_response_alg` で
   クライアントごとに選択可能
4. `none` アルゴリズムは **JWT ヘッダー検証段階で必ず拒否** する
   （`jose` ライブラリは設計上 `none` を受理しないが、念を入れて明示的にチェック）
5. `HS256` は仕様外として実装しない

これは `spec/discovery.json` の `id_token_signing_alg_values_supported` および
`token_endpoint_auth_signing_alg_values_supported` に反映される。

## 鍵のサイズと曲線

- RSA 鍵: 2048 ビット以上（FAPI 2.0 §5.7 推奨）
- ECDSA 曲線: P-256（NIST 標準、TLS との相互運用性が高い）

## アルゴリズム混乱攻撃への防御

JWS 検証ロジックは「期待アルゴリズム」を **必ず** 指定する。
ヘッダーの `alg` を信用して鍵を選ぶ実装は脆弱性の典型（CVE-2015-9235 等）。

- 各 `kid` には固定アルゴリズムを紐づける
- 検証時は `{ algorithms: [client.id_token_signed_response_alg] }` を `jose` に渡す

## 却下した代替案

- **EdDSA も許可**: ライブラリサポートが一部不完全。将来的に追加する（ADR を新規作成）
- **HS256 を内部トークン署名に使う**: 「内部 / 外部」の区別が運用ミスを生む。
  すべての JWT を非対称署名で統一する
- **アルゴリズムを `kid` 経由で動的に選ぶ**: 上記「混乱攻撃」のリスク。
  クライアントが期待アルゴリズムを宣言する設計のほうが安全

## 影響

- `adapters/crypto/jwt-signer.ts` は PS256 / ES256 のみを実装する
- 鍵生成スクリプト（運用時）も同上
- Discovery `id_token_signing_alg_values_supported` に反映する
- `jose` の `SignJWT` / `jwtVerify` に明示的に `algorithms` を渡す

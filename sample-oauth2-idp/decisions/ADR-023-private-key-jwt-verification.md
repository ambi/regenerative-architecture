# ADR-023: private_key_jwt クライアント認証の検証規則

## ステータス

採用（ADR-008 を実装に落とす）

## コンテキスト

ADR-008 で `private_key_jwt` を「推奨クライアント認証方式」と定め、
Discovery (`token_endpoint_auth_methods_supported`) と grant matrix
(`token_endpoint_auth_methods`) に宣言済みだった。しかし実装は「枠組みのみ」で、
Discovery が広告する方式を実際には検証できない（仕様が約束しているのに実装が応えない）
状態だった。FAPI クライアントはこの方式でしか認証できないため、これは適合性の欠落である。

`private_key_jwt` (RFC 7521 / RFC 7523) は、クライアントが自分の秘密鍵で署名した
JWT (`client_assertion`) をトークンエンドポイント等に提示する方式。共有秘密
(`client_secret_*`) より鍵漏洩時の被害が小さく、IdP 側に検証用の秘密を置かない。

## 決定

`client_assertion` の検証規則を以下に固定する（`adapters/http/client-authentication.ts`
の `verifyClientAssertion`）。

1. **alg は PS256 / ES256 のみ** — ADR-003 の署名方針と一致。`alg: none` や HMAC は拒否
   （アルゴリズム混乱攻撃の防止）。
2. **iss === sub === client_id** (RFC 7523 §3) — `sub` から登録クライアントを引き、
   jose の `issuer` / `subject` オプションで両クレームが client_id に一致することを強制。
3. **aud はこのサーバーを指す** — issuer 識別子・各エンドポイント URL・実リクエスト URL の
   いずれかに一致（`buildAcceptableAudiences`）。
4. **署名鍵はクライアント登録鍵から解決** — インライン `jwks` を優先し、無ければ `jwks_uri`
   を `createRemoteJWKSet` で取得。登録時に鍵の存在を必須化（`register-client.ts`）。
5. **exp 必須かつ寿命を有界化** — `exp` が無い assertion は拒否。寿命が
   `MAX_ASSERTION_LIFETIME_SECONDS`(=300) + クロックスキューを超えるものは拒否し、
   リプレイ窓を確定させる。
6. **jti 単回使用** — `ClientAssertionReplayStore` で jti のリプレイを検出。DPoP の jti
   とは別名前空間・別 TTL の独立ポートとする（責務が異なる）。
7. **複数認証方式の併用禁止** (RFC 6749 §2.3) — `client_assertion` と Basic/secret の
   同時提示は `invalid_request`。

## 影響

- `ClientAssertionReplayStore` ポート + memory / redis アダプタを追加。
- `authenticateClient` に `ClientAuthOptions { issuer, clientAssertionReplayStore }` を渡す。
  /token・/par・/introspect・/revoke の各ルートが issuer と replay store を保持する。
- `register-client.ts` は `private_key_jwt` クライアントに `jwks` または `jwks_uri` を要求し、
  secret は secret ベース方式のときだけ発行する。
- `verifyClientAssertion` は HTTP Context 非依存の純粋関数として切り出し、単体テスト可能
  （`adapters/http/client-authentication.test.ts`）。

## jwks_uri の SSRF 対策（実装メモ）

`jwks_uri` は登録時に許可ホスト/スキーム (https のみ・内部 IP 拒否) を検証する前提。
本サンプルはインライン `jwks` を主経路とし、`jwks_uri` は production の枠組みとして残す。

## 却下した代替案

- **client_secret_jwt (HMAC)**: ADR-008 で却下済み（対称鍵は漏洩時の被害が大きい）。
- **jti リプレイを DpopReplayStore と共用**: TTL・名前空間・監査意味論が異なるため分離した。
- **aud をトークンエンドポイント URL のみ許可**: issuer 識別子を aud とする実装が多く、
  相互運用性のため両方を受理する。

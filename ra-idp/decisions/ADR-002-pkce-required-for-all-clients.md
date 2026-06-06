# ADR-002: PKCE 必須要件の client type 階段化

## ステータス

採用 (2026-06: 「すべての authorization_code クライアントに必須」から
「public / FAPI クライアントは必須・confidential client は明示 opt-in」へ改訂)

## コンテキスト

OAuth 2.0 Security Best Current Practice (RFC 9700) §2.1.1 は、すべての
authorization_code クライアントに対して PKCE を要求している:

> Clients MUST prevent injection of authorization codes into the
> authorization response, by using PKCE [RFC7636] or, in the case of an
> OpenID Connect client, the nonce parameter.

FAPI 2.0 §5.1 はさらに強い表現で「PKCE は MUST」を要求する。OAuth 2.1 も
同方向 (PKCE を「optional but recommended for confidential」と段階化)。

歴史的に PKCE は「public クライアント (SPA・ネイティブアプリ) の代替防御」
として位置づけられていた。しかし `code injection attack` は confidential
クライアントに対しても成立しうる (attacker が code を盗み出して別のクライアントで
使う) ことが示されている。

ただし RFC 9700 / FAPI 2.0 は SHOULD/MUST の差を保ち、OAuth 2.0 (RFC 6749) /
OIDC Core 1.0 は PKCE を要求していない。本実装の旧版は client_type 不問で
PKCE 必須としていたが、これは原仕様の互換範囲を狭めるため、レガシー confidential
クライアントの移行コストを背負わせる。

## 決定

client metadata `require_pkce` を導入し、PKCE 要否を client 単位で決定する。
未指定時のデフォルトは client_type / fapi_profile から派生する:

| クライアント分類 | default `require_pkce` | 根拠 |
| ---------------- | ---------------------- | ---- |
| public           | true                   | client_secret を保持できず、PKCE が唯一の防御 |
| FAPI 2.0         | true                   | FAPI 2.0 §5.1 は MUST |
| confidential (legacy) | false             | RFC 6749 互換、移行配慮。明示で true 化推奨 |

`code_challenge_method` は引き続き `S256` のみサポート (plain は禁止)。

### 実装

- `spec/scl.yaml` `OAuth2Client.fields` および `ClientRegistrationRequest.fields` に
  `require_pkce: Boolean (optional)`
- `permissions.AuthorizeInitiate.allow_when` の PKCE 述語を
  `(actor.require_pkce == false) or (context.code_challenge != null)` に変更
- `src/oauth2/usecases/authorize-request.ts::resolveRequirePkce(client)` で
  default を解決し、policy へ流す
- `adapters/http/authorize-routes.ts` の事前 required 配列から `code_challenge` を
  外し、policy 経由で判定 (二重検証の削除)
- `src/oauth2/usecases/exchange-code-for-token.ts` で code に code_challenge が
  ない場合 verifier 検証をスキップ。verifier が送られてきたら downgrade として拒否
- migration 0003 で `clients.require_pkce` 列を追加 (NULL 許容)

## 認可コードの単一使用と短寿命 (変更なし)

認可コードは単一使用かつ短寿命 (TTL ≤ 60 秒)。理由は変わらず:

- 再利用検出時、関連トークンをすべて失効させる動作 (RFC 9700 §4.10) が
  「単一使用」の前提に依拠する
- 短寿命にすることで、コード漏洩時の攻撃ウィンドウを最小化する

## 却下した代替案

- **旧版「すべてに PKCE 必須」を維持**: RFC 6749 互換を壊し、レガシー confidential
  クライアントの移行コストを正当化できない
- **PKCE plain method 許可**: SHA-256 と plain で防御強度に差はあるが、plain は
  ログから verifier を回収できるリスクがある (FAPI 2.0 は S256 のみを許す)
- **`require_pkce` を必ず明示要求**: RFC 7591 のクライアント登録ペイロードに
  追加負荷を強いる。default を client_type / fapi_profile から派生させるほうが
  実装しやすく、ADR-008 の token_endpoint_auth_method デフォルトと同方針

## 影響

- 旧 confidential クライアントは PKCE なしで動く (旧仕様で運用していた
  デプロイの互換性回復)
- 新規 public / FAPI クライアントは引き続き PKCE 必須
- migration 0003 は加法的 (`ALTER TABLE clients ADD COLUMN require_pkce BOOLEAN`)
- 既存の `pkce_verification_passed` policy ルールは「challenge と verifier の
  整合性」のみを判定する純粋関数として残し、PKCE 省略 client の場合は
  verifier が来ていなければ Permit に変更
- PKCE 検証失敗は引き続き `400 invalid_grant` で拒否 (タイミング差を出さない原則)

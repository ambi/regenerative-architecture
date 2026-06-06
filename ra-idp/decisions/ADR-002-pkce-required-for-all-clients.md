# ADR-002: PKCE をすべての authorization_code フローで必須とする

## ステータス

採用

## コンテキスト

OAuth 2.0 Security Best Current Practice (RFC 9700) §2.1.1 は、すべての
authorization_code クライアントに対して PKCE を要求している:

> Clients MUST prevent injection of authorization codes into the
> authorization response, by using PKCE [RFC7636] or, in the case of an
> OpenID Connect client, the nonce parameter.

FAPI 2.0 §5.1 はさらに強い表現で「PKCE は MUST」を要求する。

歴史的に PKCE は「public クライアント（SPA・ネイティブアプリ）の代替防御」として
位置づけられていた。しかし `code injection attack` は confidential クライアントに対しても
成立しうる（attacker が code を盗み出して別のクライアントで使う）ことが示されており、
現在は client_type にかかわらず PKCE を要求するのが業界標準である。

## 決定

本アプリ IdP はすべての `authorization_code` グラントに対して `code_challenge` を必須とする。
例外なし。レガシークライアント互換のための「PKCE オプショナル」モードは実装しない。

実装:

- `spec/policy/client-authorization.json` の `authorize:initiate.rules` に
  `pkce_present` を必須ルールとして含める。
- `code_challenge_method` は `S256` のみをサポートする（`plain` は禁止）。
- 認可コードは `code_challenge` と一緒に保存され、`/token` 時に
  `SHA-256(code_verifier) == code_challenge` を厳密に検証する。

## 認可コードの単一使用と短寿命

PKCE と組み合わせて、認可コードを単一使用かつ短寿命（TTL ≤ 60 秒）にする
（`spec/slo.yaml` の `authorization_code_ttl_seconds = 60`）。

理由:

- 再利用検出時、関連トークンをすべて失効させる動作（RFC 9700 §4.10）が
  「単一使用」の前提に依拠する
- 短寿命にすることで、コード漏洩時の攻撃ウィンドウを最小化する

## 却下した代替案

- **PKCE オプショナル + nonce 必須**: OIDC のみのケースで成立するが、OAuth2 のみ
  （ID Token を発行しない）クライアントには nonce が無いため、PKCE が唯一の防御になる
- **PKCE plain method 許可**: SHA-256 と plain で防御強度に差はあるが、plain は
  ログから verifier を回収できるリスクがある（FAPI 2.0 は S256 のみを許す）

## 影響

- 既存のレガシークライアントは移行が必要
- PKCE 検証失敗は `400 invalid_grant` で拒否する（クライアントエラーを区別しない理由は
  `requirements.md §3` 末尾の「タイミング差を出さない」原則）

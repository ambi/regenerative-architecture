# ADR-061: ファーストパーティ portal を自分自身の IdP の OIDC RP にする

## ステータス

採用。[[wi-66-portals-as-oidc-rp]] が導く。

## コンテキスト

管理コンソール (`/admin/*`) とアカウントポータル (`/account/*`) の React SPA は、これまで
IdP 自身のファーストパーティ・ブラウザセッション (HttpOnly セッション cookie、
`POST /api/auth/login` で発行) でログインしていた。`/api/admin/*` と `/api/account/*` は
セッション cookie から `sub` を解決し、ロール / actor 境界 (ADR-031 / ADR-038 / ADR-042) で
認可する。両 SPA は OIDC RP ではなく IdP 内蔵アプリで、IdP が発行するトークンを消費して
いなかった。

デモ IdP として「IdP を本来の使い方 (OIDC RP) で自分自身に対して使う」ことを示す価値が
ある。Keycloak も admin console を master realm の OIDC クライアント
(`security-admin-console`) として保護しており、自己ドッグフーディングは確立された構成で
ある。一方で、IdP が自分自身を認証する循環依存は、OIDC クライアント設定や署名鍵の破壊が
管理コンソールのロックアウトに直結するという可用性リスクを生む。

トークンの持ち方には BFF (サーバ側保持) と pure SPA (ブラウザ保持) の選択肢があり、後者は
`ui/ARCHITECTURE.md` が掲げてきた no-token-in-JS 方針と衝突する。本 ADR はこの方針変更を
含めて判断を確定する。

## 決定

1. **両 portal を自分自身の IdP の OIDC RP にする**。`ra-admin-console` /
   `ra-account-portal` を public + authorization_code + PKCE 必須 + first-party
   (consent skip) クライアントとして登録し、`/authorize`→`/callback`→`/token` で
   access token (RFC 9068 JWT, [[ADR-012]]) を取得する。

2. **pure SPA RP を採用する**。トークンはブラウザ (sessionStorage) が保持し、
   `/api/admin/*` / `/api/account/*` を `Authorization: Bearer` のリソースサーバに
   する。BFF (サーバ側 token 保持) は採らない。これに伴い `ui/ARCHITECTURE.md` の
   no-token-in-JS 方針を更新する。

3. **トークン露出リスクは寿命と回転で抑える**。access token は短命 (600s, [[ADR-012]])、
   refresh は rotation + family revoke ([[ADR-004]])。UI API は `Cache-Control: no-store`
   を維持し、トークンは URL / ログ / DOM に出さない。

4. **リソースサーバ認可は既存境界を再利用する**。Bearer の `sub` を解決し、
   admin は `RequireAdmin` (ADR-031 / ADR-038)、account は self 境界 (ADR-042) を
   そのまま適用する。トークンは要求 scope (`ra.admin` / `ra.account`) と audience /
   realm の一致を満たすこと。いずれも満たさなければ fail-closed。

5. **ブートストラップ・ロックアウトを緩和する**。OIDC 経路が壊れても管理者が復旧できる
   よう、first-party セッションログイン (`POST /api/auth/login`) を緊急経路として残す。
   移行中の `ResolveAuthentication` は Bearer とセッションの dual-mode とし、両経路が
   同一のロール / actor 境界を通ることをテストで担保する。

6. **段階移行する**。resource server 化 (Bearer 受理 + クライアント seed) を後方互換な
   先行増分とし、SPA の RP クライアント実装と loadPageData / callback の差し替えを
   後続段階で行う ([[wi-66-portals-as-oidc-rp]] の stages)。

## 影響

- ブラウザがトークンを保持するため XSS によるトークン窃取が直接の資格情報漏洩になる。
  短命 access token・rotation refresh・no-store・CSP でリスクを抑える。
- `/api/{admin,account}/*` は Bearer を一級の認証手段として扱い、`WWW-Authenticate:
  Bearer` を返す。セッション経路は緊急用として縮退する。
- 循環依存により IdP 設定の破壊が管理面の可用性に影響する。緊急セッションログインを
  必ず残し、README に復旧手順を書く。
- discovery の `scopes_supported` に `ra.admin` / `ra.account` を広告する。

## 参照

- [[wi-66-portals-as-oidc-rp]] — 本 ADR を導く WI。
- [[ADR-012]] — access token を JWT、refresh を opaque とする決定。
- [[ADR-004]] — refresh token rotation と family revoke。
- [[ADR-031]] — 管理 API と RBAC (admin ロール境界)。
- [[ADR-038]] — group 由来の effective roles。
- [[ADR-042]] — account portal の self / admin trust boundary。

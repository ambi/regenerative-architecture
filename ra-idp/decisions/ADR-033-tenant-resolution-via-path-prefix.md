# ADR-033: テナント解決は `/realms/{tenant_id}` パスプレフィックスで行う

## ステータス

採用

## コンテキスト

ADR-032 で導入したテナント集約をリクエストに紐付ける手段を選ぶ必要がある。
主な選択肢は次の三つ:

1. **サブドメイン** `acme.idp.example.com` — Auth0 のカスタムドメイン等で
   定番。テナント単位の TLS / DNS / CDN 設定が要求される。
2. **パスプレフィックス** `/realms/{id}/...` — Keycloak が標準採用。
   既存 TLS 終端と DNS のままで動く。
3. **ヘッダ** (`X-Tenant-Id`) — 内部 API 向け。ブラウザフローでは
   伝搬が難しく、Redirect URI / `iss` claim との整合性が破綻する。

ra-idp はブラウザフロー (`/authorize` `/login` `/consent`) を扱うため、
`iss` claim と Discovery メタデータがテナントごとに一意である必要がある。
3 は機構的に不適。1 vs 2 のトレードオフを評価した。

## 決定

1. **すべてのプロトコルルートを `/realms/{tenant_id}/...` 配下に配置する**。
   対象: `/authorize` `/token` `/par` `/device_authorization` `/device`
   `/login` `/consent` `/totp` `/userinfo` `/introspect` `/revoke`
   `/register` `/end_session` `/account/password` `/forgot_password`
   `/reset_password` `/.well-known/openid-configuration`
   `/.well-known/oauth-authorization-server` `/jwks` `/admin/users`。
   system-wide な `/admin/tenants` は realm に所属しないため prefix 対象外。

2. **未 prefix のルートは `default` テナントへ解決する**。
   既存 RP / `demo.sh` / 既存 docs を破壊しないため。README の endpoint
   テーブルでは prefix 形式を正、bare 形式を deprecated と明示する。

3. **`iss` claim は `{base}/realms/{tenant_id}` を発行する**。
   token 発行・Discovery `issuer` フィールド・OIDC ID token の `iss`
   は同一文字列となる。

   **後方互換のための escape hatch:** 環境変数
   `LEGACY_BARE_ISSUER=true` を設定すると `default` テナントの未 prefix
   ルートが `iss = {base}` を発行する。1 リリース限定の暫定措置として
   提供し、その後削除する。

4. **`/realms/{tenant_id}/.well-known/openid-configuration`** は同 prefix の
   endpoint URL を返す。JWKS URI は `{base}/realms/{tenant_id}/jwks` だが、
   本フェーズでは同一の JWKS bytes を返す（per-tenant 鍵は Phase 8）。
   `issuer` フィールドは `{base}/realms/{tenant_id}` と一致する。

5. **`TenantResolver` middleware** が HTTP 境界で次を行う:
   - path から `tenant_id` を抽出 (正規表現 `^/realms/([a-z0-9][a-z0-9-]{0,62})(/|$)`)
   - 未 prefix は `defaultTenantId` (固定値 `default`) にフォールバック
   - `TenantRepository.findById(tenant_id)` で解決
   - 不在テナントは 404 + generic `tenant_not_found` を返す
   - disabled テナントは protocol route で 400 + generic `invalid_request`
     を返す。管理 API の GetTenant だけが status を取得できる
   - どちらも tenant ID や存在・状態の詳細を説明文へ含めない
   - Hono context に解決済み `Tenant` と `issuer` 文字列をセット

6. **subdomain 戦略は将来差し替え可能な slot として残す**。
   `TenantResolver` interface は path / subdomain / ヘッダ いずれの
   実装でも置換可能とし、本 ADR はその初期実装として path-prefix
   `PathPrefixTenantResolver` を採用する。

## 影響

- `bootstrap/config.ts` の `issuer` は **base URL** に意味が変わる
  (`{base}` を保存し、リクエスト時に `/realms/{id}` を後置)。
  既存 `ISSUER` env が `https://idp.example.com/realms/default` の形に
  なっていた場合、移行ガイドが必要。
- Hono のルートマウントが二重になる: `/realms/:id/...` と `/` (default
  フォールバック)。コード上は単一ルート群を二度マウントする方法と、
  middleware で path-rewrite する方法がある。本実装は前者を採用する
  （middleware で書き換えると downstream route のテスト容易性が落ちる）。
- Redirect URI 検証ロジックは無影響。RP が `iss` を pin している場合は
  ADR の `LEGACY_BARE_ISSUER` で 1 リリース猶予を与える。
- Discovery メタデータの `issuer` フィールドがリクエスト URL に依存して
  動的になる。CDN / 上流キャッシュは tenant prefix を含む URL ごとに
  キャッシュキーを分ける必要がある（Vary ヘッダではなく URL で分離）。

## 却下した代替案

- **サブドメイン解決のみ採用**: 開発・CI・K8s ingress のセットアップが
  重くなり、Phase 4 のスコープを超える。ホストアプリで TLS を per-tenant
  配備する SaaS では subdomain が望ましいが、現状の demo / dev workflow
  との両立を優先する。
- **ヘッダ (`X-Tenant-Id`) 解決**: ブラウザフロー (303 redirect) で
  伝搬できず、`iss` claim と URL の整合が崩れる。OIDC RP 実装が
  サーバから受け取った `iss` URL を JWKS / discovery 取得に再利用する
  ため、URL に tenant が現れない設計は事実上採れない。
- **URL prefix + iss を bare のまま固定**: iss = URL prefix の不一致は
  RP 側の検証ライブラリ (ex: `oidc-client-ts`) がデフォルトで弾く。
  単一 IdP インスタンスを RP から見て複数 IdP に見せる目的に反する。

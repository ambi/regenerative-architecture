# ADR-066: Application を唯一の編集面とし、低レベル client / RP 画面を撤去する

## ステータス

採用 (accepted)。[[ADR-064]] の Application 情報設計と
[[wi-69-application-catalog-aggregate-and-assignment]] の統合を引き継ぎ、編集導線を
Application に一本化する。実装は
[[wi-76-fold-advanced-protocol-settings-into-application-editor]] で行う。

## コンテキスト

[[wi-69-application-catalog-aggregate-and-assignment]] (commit a5e2ec8) は OAuth2 client /
WS-Federation relying party / service client を単一の「アプリケーション」へ統合し、サイドバー
導線を Application に一本化した。しかし低レベルの `AdminClients` (/admin/clients) と
`AdminWsFedRelyingParties` (/admin/wsfed/relying-parties) は「advanced 面」として残され、
サイドバー導線を持たず URL 直叩きでのみ到達する状態になった。

この残置は技術的負債であって設計意図ではない。Application 編集画面の編集インターフェース
(`ApplicationOidcConfigUpdateRequest` / `ApplicationWsFedConfigUpdateRequest`) が日常運用の
最小集合 (redirect_uris / scope, reply_urls / name_id) しか公開していないため、以下の高度な
設定が低レベル画面からしか編集できない。

- OIDC: `grant_types`, `response_types`, `token_endpoint_auth_method`, `jwks_uri` / `jwks`,
  `tls_client_auth_subject_dn`, `fapi_profile`, `require_pushed_authorization_requests`,
  `dpop_bound_access_tokens`
- WS-Fed: Entra domain federation 設定 (`ConfigureEntraFederation`)

結果として「Application が単一導線である」という [[ADR-064]] 6 項の主張が崩れ、URL を知る
管理者だけが advanced 設定に触れられる隠れ導線が残っている。これは情報設計の一貫性を損ない、
権限境界の説明を難しくする。

## 決定

1. **advanced なプロトコル設定を Application 編集画面に畳む。**
   低レベル画面でしか編集できなかった OIDC / WS-Fed の高度な項目を、Application 編集画面の
   「詳細設定」セクションとして編集可能にする。編集は背後の OAuth2 client / WS-Fed RP の
   既存更新 use case へ委譲し、PKCE 必須 ([[ADR-002]])、FAPI で PAR 必須 ([[ADR-006]])、
   認証方式 ([[ADR-008]]) などの不変条件を Application 経由でも同じ検証で守る。

2. **更新契約上の不変項目は編集対象に昇格させない。**
   `token_endpoint_auth_method` のように `AdminClientUpdateRequest` が変更対象から除外して
   いる項目は、Application 編集画面でも読み取り専用で表示し、編集 request に含めない。
   不変項目の変更は再作成またはローテーション専用導線で扱う。

3. **`AdminClients` / `AdminWsFedRelyingParties` の画面を撤去する。**
   SCL の screen 定義、各 variant の screen 列、これらを参照する assurance、UI route と page を
   撤去し、URL 直叩きの隠れ導線を無くす。Application が名実ともに単一の編集面になる。
   OIDC client の作成時専用項目 (client_type / token_endpoint_auth_method / jwks_uri /
   tls_client_auth_subject_dn) は Application 作成フローに畳む。

5. **Entra domain federation はテナント設定として独立させる。**
   Microsoft Entra domain federation は「検証済みドメイン」単位で relying party を upsert する
   テナント/ドメインレベルの操作であり、個別 Application の編集ではない。したがって per-RP の
   WS-Fed 設定 (audience / token_type / claim mapping) は Application 編集画面に畳む一方、
   Entra federation preset は専用のテナント設定画面に移し、`AdminWsFedRelyingParties` 撤去後も
   到達できるようにする。

4. **低レベル CRUD interface と HTTP endpoint は内部 API として残す。**
   `ListAdminClients` / `GetAdminClient` / `CreateAdminClient` / `UpdateAdminClient` /
   `DeleteAdminClient` と対応する `/api/admin/clients`、WS-Fed RP の同等 API は、Application
   provisioning と詳細編集の委譲先として内部的に必要なため残す。ただし UI からの直接到達導線
   (専用画面・サイドバー項目) は持たせない。SCL 上はこれらの interface の screen 参照を解消し、
   画面に紐づかない内部 interface として扱う。

## 影響

- SCL: `ApplicationOidcConfig` / `ApplicationOidcConfigUpdateRequest` と WS-Fed 等価型を
  advanced 項目で拡張する。`AdminClients` / `AdminWsFedRelyingParties` screen と参照を撤去する。
  生成 HTML と `internal/shared/spec` をロックステップ更新する。
- Go: application detail resolver が advanced 項目を読み出し、`UpdateApplicationOidcConfig` /
  `UpdateApplicationWsFedConfig` が背後 client / RP 更新へ委譲する。低レベル HTTP route は
  内部 API として残す。
- UI: `admin-applications` の編集画面に「詳細設定」を追加し、`admin-clients` /
  `admin-wsfed` feature・route を撤去する。`adminNav.ts` の advanced URL コメントを更新する。
- [[ADR-064]] 6・7 項の「Application を単一導線にする」方針を、編集面の観点で完了させる。

## 却下した代替案

- **低レベル画面を advanced 面として恒久的に残す。** 「単一導線」という情報設計の主張と矛盾し、
  URL を知る者だけが触れる隠れ導線という権限境界の説明困難を残す。
- **低レベル HTTP API も撤去する。** Application provisioning / 詳細編集は内部でこれらの
  use case に委譲しており、API ごと撤去すると委譲先を失う。UI 導線のみ撤去すれば隠れ導線の
  問題は解消する。
- **不変項目も編集可能にする。** `token_endpoint_auth_method` の変更は client 認証契約を
  破壊しうるため、更新契約の不変性 ([[ADR-008]]) を編集面でも維持する。

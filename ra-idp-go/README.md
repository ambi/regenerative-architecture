# ra-idp-go — IdP の Go 実装

仕様核 `spec/scl.yaml` をもとに Regenerative Architecture に従って開発している IdP アプリケーション。

## 範囲

- 全 SCL バインディング (vocabulary / models / state_machines / objectives / properties / scenarios / permissions / interfaces / annotations)
- 認可エンドポイント `/authorize` + React Login / Consent UI + End Session (`/end_session`)
- ブラウザ認証API `/api/auth/*` + Session Cookie + CSRF
- メールによるパスワードリセット + 単発・30分TTLトークン (ADR-030)
- RBACで保護された管理ユーザーAPI (`/admin/users`) + ユーザー無効化 (ADR-031)
- テナント内に閉じた管理クライアント CRUD (`/admin/clients`)
- テナント内に閉じた管理 consent 参照・撤回 API (`/admin/consents`)
- `/realms/{tenant_id}` による tenant 分離、tenant 管理 API、tenant-scoped persistence (ADR-032〜034)
- トークンエンドポイント `/token` (authorization_code, refresh_token, client_credentials, device_code)
- リフレッシュトークンのローテーション + ファミリー失効 (ADR-004)
- DPoP (RFC 9449) プルーフ検証
- private_key_jwt (RFC 7523) クライアント認証 (inline JWKS / `jwks_uri`)
- Pushed Authorization Request (RFC 9126) `/par`
- Device Authorization Grant (RFC 8628) `/device_authorization`, `/device`
- Token Introspection (RFC 7662) `/introspect`
- Token Revocation (RFC 7009) `/revoke`
- UserInfo (OIDC Core §5.3) `/userinfo`
- Dynamic Client Registration (RFC 7591) `/register`
- OIDC Discovery + JWKS (`/.well-known/openid-configuration`, `/jwks`)
- PS256 JWT 署名 + JWKS + in-memory / PostgreSQL KeyStore (RFC 7638 サムプリント kid)
- PostgreSQL durable state + Redis volatile state
- PostgreSQL outbox + Kafka relay
- OpenTelemetry OTLP/HTTP traces / metrics
- local / remote AuthZEN ポリシー評価
- Domain Event 発火 (console / outbox event sink)
- Zog schema によるモデル・HTTP入力・パスワードポリシー検証

## 起動

認証UIは TypeScript + Vite + React + Tailwind CSS + Radix UI + shadcn/ui +
TanStack Router / Table で実装する。Go APIとは別成果物・別プロセスとして配信し、
CaddyなどのGatewayから同一オリジンに統合する。
デザインと実装の判断基準は [`ui/README.md`](ui/README.md) に記載する。

開発時はGo APIとReact UIを別プロセスで起動する。

```bash
# terminal 1: Go API
ADDR=:8081 ISSUER=http://localhost:5173 go run ./cmd/ra-idp-go

# terminal 2: React UI (API proxy included)
cd ui
bun install
bun run dev
```

`http://localhost:5173/` を開き、「ローカルデモ認証を開始」を選ぶ。
ログイン画面は認可トランザクションを必要とするため、`/login` を直接開かない。

Docker ComposeではCaddyが `http://localhost:8080` でUIとAPIを公開する。

```bash
docker compose -f infra/docker/docker-compose.dev.yaml up --build
```

主要な OAuth 2.0 / OpenID Connect フローを実行する:

```bash
BASE=http://localhost:8080 \
./demo.sh
```

### メール配送をローカルで試す (mailpit)

`EMAIL_SENDER=console` の既定では reset リンクが stdout に出るだけだが、
SMTP adapter (ADR-035) も手元で簡単に試せる。
[mailpit](https://mailpit.axllent.org/) を「全宛先を捕まえる偽 inbox」として
使う:

```bash
# 1) mailpit を起動 (Homebrew の場合)
brew install mailpit
mailpit --smtp 127.0.0.1:1025 --listen 127.0.0.1:8025

# 2) 別ターミナルで ra-idp-go を SMTP モードで起動
export EMAIL_SENDER=smtp
export SMTP_HOST=127.0.0.1
export SMTP_PORT=1025
export SMTP_TLS=none
export SMTP_FROM=noreply@ra-idp.test
./dev.sh
```

起動直後の log に `email sender: smtp host=127.0.0.1 port=1025 tls=none from=...`
が出れば adapter は正しく切り替わっている。

UI の「パスワードを忘れた」から `alice@example.com` (demo seed) を入力すると、
`http://127.0.0.1:8025` の mailpit Web UI に reset リンク付きのメールが届く。
mailpit は宛先に関係なく全部内部に貯めるので、Gmail などの実 inbox には流れない。

`SMTP_TLS=none` は mailpit が TLS を喋らないためのローカル限定設定。
本番では `starttls` / `implicit` のいずれかを使う。

### 本番アダプタ構成

```bash
PERSISTENCE=postgres \
DATABASE_URL='postgres://ra_idp:ra_idp@localhost:5432/ra_idp?sslmode=disable' \
REDIS_URL='redis://localhost:6379/0' \
EVENT_SINK=outbox \
OBSERVABILITY=otel \
OTEL_EXPORTER_OTLP_ENDPOINT='http://localhost:4318' \
go run ./cmd/ra-idp-go
```

```bash
DATABASE_URL='postgres://ra_idp:ra_idp@localhost:5432/ra_idp?sslmode=disable' \
KAFKA_BROKERS='localhost:9092' \
go run ./cmd/ra-idp-relay
```

### 設定

| 環境変数             | 値 / 既定値                                                                       |
| -------------------- | --------------------------------------------------------------------------------- |
| `PERSISTENCE`        | `memory` / `postgres` (`memory`)                                                  |
| `DATABASE_URL`       | PostgreSQL接続先。`postgres`時に必須                                              |
| `REDIS_URL`          | Redis接続先。`postgres`時に必須                                                   |
| `AUTO_MIGRATE`       | 起動時migration (`true`)                                                          |
| `MIGRATIONS_DIR`     | `infra/migrations`                                                                |
| `EVENT_SINK`         | `console` / `outbox` (`console`)                                                  |
| `OBSERVABILITY`      | `noop` / `otel` (`noop`)                                                          |
| `AUTHZEN`            | `local` / `remote` (`local`)                                                      |
| `AUTHZEN_URL`        | remote AuthZENのbase URL                                                          |
| `KAFKA_BROKERS`      | relay用comma-separated broker                                                     |
| `SKIP_DEMO_SEED`     | 設定時はデモデータを保存しない                                                    |
| `LEGACY_BARE_ISSUER` | `true` の場合だけ bare route の default issuer を旧 `{base}` 形式にする (`false`) |
| `EMAIL_SENDER`       | `console` / `smtp` (`console`)。`smtp` 選択時は SMTP\_\* を読む                   |
| `SMTP_HOST`          | SMTP relay ホスト。`smtp` 時に必須                                                |
| `SMTP_PORT`          | ポート (`SMTP_TLS` 既定値: starttls→587 / implicit→465 / none→25)                 |
| `SMTP_USERNAME`      | PLAIN auth ユーザ名 (空なら認証なし)                                              |
| `SMTP_PASSWORD`      | PLAIN auth パスワード。ログには出さない (ADR-035 §10)                             |
| `SMTP_FROM`          | RFC 5322 `From:` / SMTP `MAIL FROM`。`smtp` 時に必須 (bare address)               |
| `SMTP_HELO`          | EHLO/HELO で使う local name (`localhost`)                                         |
| `SMTP_TLS`           | `starttls` / `implicit` / `none` (`starttls`)。`none` は開発専用                  |
| `SMTP_TIMEOUT_SECONDS` | dial + コマンドの timeout (`10`)                                                |

`jwks_uri` はHTTPSのみ許可し、private / loopback / link-localアドレス、
userinfo、fragmentを拒否する。取得は3秒timeout、1 MiB上限、5分cacheとする。

構造体の整合性と外部入力の型変換・形式検証には
[Zog](https://zog.dev/) を使う。登録済みredirect URIとの一致、スコープ許可、
状態遷移、PKCEなど実行時コンテキストを必要とする検証はusecase/domainに置く。

### マルチテナンシー

プロトコル route は `/realms/{tenant_id}/authorize`、
`/realms/{tenant_id}/token`、`/realms/{tenant_id}/.well-known/openid-configuration`
の形式を正とする。prefix のない既存 route は `default` tenant に解決される。
issuer は通常 `{ISSUER}/realms/{tenant_id}` となる。

tenant lifecycle API (`/realms/default/admin/tenants/...`) は cross-tenant 操作だが、
ADR-032 で `system_admin` を default control-plane tenant に所属させているため
default realm prefix 配下に置く (default tenant の session cookie path で覆えるため、
root への cookie 広げが不要になる)。`admin` role の `/admin/users` 操作は request
tenant 内に限定される。

Redis の一時状態 key は `tenant:{id}:` namespace に分離される。旧形式 key
は migration せず TTL により失効するため、切替直後の in-flight 認可・device
flow・session は再実行が必要になる場合がある。

**署名鍵はテナント間で共有される (Phase 8 で per-tenant 鍵化予定)。**
すべてのテナントが同じ `KeyStore` を参照し、`/realms/{tenant_id}/jwks` は同一の
JWKS を返す。トークン整合性は `iss` claim (`{base}/realms/{tenant_id}`) と
audience の検証に依存するため、RP / Resource Server が **`iss` を厳格に検証する
ことが前提条件** となる。`iss` を見ない RP は、テナント A 向けに発行された
アクセストークンをテナント B の RS に持ち込んでも署名検証が通ってしまう。
本番マルチテナント環境に投入する前に per-tenant 鍵への切替を完了させること。

## 検証

```bash
go test -race ./...
golangci-lint run
```

テストは認可コードのatomic redeem、Redis Lua、refresh family失効、Device Flow、クライアント認証、`jwks_uri` SSRF防御、AuthZEN wire契約、イベントwire形式を含む。

デモシード:

| 種類          | 値                               |
| ------------- | -------------------------------- |
| client_id     | `demo-client`                    |
| client_secret | `demo-client-secret`             |
| redirect_uri  | `http://localhost:3000/callback` |
| ユーザ名      | `alice`                          |
| パスワード    | `demo-password-1234`             |

## ディレクトリ構成

```text
ra-idp-go/
├── spec/                                    Layer 1: 仕様核 (SCL)
├── decisions/                               Layer 2: コンセプション / ADR
├── ui/                                      React SPA + Caddy reference configuration
├── cmd/ra-idp-go/main.go               起動
├── internal/spec/                      Layer 3: SCL バインディング + 状態機械
├── internal/oauth2/                    Layer 3: domain / ports / usecases
├── internal/authentication/            Layer 3: パスワードポリシー / セッション
├── internal/adapters/crypto/           Layer 4: Argon2id, PS256, DPoP, private_key_jwt
├── internal/adapters/persistence/      Layer 4: memory / PostgreSQL / Redis
├── internal/adapters/http/             Layer 4: Echo v5
├── internal/adapters/observability/    Layer 4: OpenTelemetry
├── internal/adapters/policy/           Layer 4: local / remote AuthZEN
├── internal/adapters/eventsink/        Layer 4: console / Kafka relay
└── infra/                              migrations / Docker Compose / OTel Collector
```

## 実装ロードマップ

現状は RA の仕様核・派生物・ユースケース・アダプタ・運用面を備えた IdP だが、
商用・社内共通基盤として本番投入するには次が不足している。依存関係・リスクの
大きさ・RA 的清潔さ（既存 port を完成 → 新 port を追加の順）の 3 軸で
フェーズ分けした実装ロードマップとして示す。

> Phase 番号は依存関係と推奨順序の目安。ユーザ価値が大きい項目は他 Phase より優先してよい（順序の根拠が壊れる場合のみ後述の「順序の根拠」を参照）。既に実装済みの領域は本表から除外し、残タスクだけを掲載している（旧 Phase 1 の Token / DPoP / mTLS / PKCE 階段化、旧 Phase 2 の UI 基盤一式はいずれも完了済み）。

### Phase 0 — 認証の土台

Argon2id ハッシャ + 長さ (12–128 chars) + ユーザー識別子との類似禁止 + 共通パスワード辞書
(bundled) + パスワード履歴再利用禁止 (`PasswordHistoryRepository`) のパスワードポリシーは
実装済 (ADR-026)。文字種要件 / periodic rotation は ADR-026 で意図的に採用しない。
per-account / per-IP のログイン試行レート制限とユーザー名列挙対策 (sentinel hash) も
ADR-029 で実装済。本番運用可能な `EmailSender` adapter (SMTP) も ADR-035 で実装済。
残るのは本番運用に必要な周辺強化。

| 領域                   | 不足している機能                                                                                                                                                                                                                     |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 漏洩パスワード検査     | HIBP k-anonymity 等のオンライン漏洩データベース連携 (`BreachedPasswordChecker` port の実装。現状は `NoopBreachedPasswordChecker` のみ)                                                                                               |
| ブルートフォース防御   | CAPTCHA / 行動分析                                                                                                                                                                                                                   |
| エンドポイント保護     | `/token` `/authorize` `/par` `/device_authorization` の一般 rate limit / bot 対策                                                                                                                                                    |
| アカウント整合性       | メール・電話番号検証                                                                                                                                                                                                                 |

### Phase 1 — Secret / 鍵のライフサイクル運用

旧 Phase 1（既存仕様の運用穴埋め）は Token (RFC 9207 + access token denylist)、
DPoP nonce + 全経路適用、mTLS 検証 + cnf.x5t#S256 バインド、PKCE 階段化 (ADR-002 改訂)
が完了済。残るのは鍵・シークレットの運用面のみ。実 KMS/HSM 差し替えは Phase 8 でカバーする。

| 領域                     | 不足している機能                                                                                                     |
| ------------------------ | -------------------------------------------------------------------------------------------------------------------- |
| client_secret rotation   | rotate use case + `/rotate_secret` 系エンドポイント、確認・通知フロー                                                |
| 署名鍵 rotation の自動化 | `rotateSigningKeyUseCase` と CLI は実装済。scheduler / k8s CronJob による定期実行、grace period 中の旧鍵保持運用は未 |

### Phase 2 — MFA / Passkey と acr/amr 体系

旧 Phase 2 (UI 基盤) のデザインシステム、ja/en i18n、a11y 基盤、ブランディング slot、
4 画面 (login / consent / device / error) はすべて実装済。TOTP (`urn:ra-idp:acr:mfa`
対応) と `acr_values` / `max_age` を消費する step-up 再認証も実装済。本 Phase から
は認証手段をさらに増やし、identity assurance 体系を整える。

| 領域     | 不足している機能                                                              |
| -------- | ----------------------------------------------------------------------------- |
| 認証手段 | WebAuthn / Passkey、バックアップコード、magic link / passwordless email       |
| 体系     | identity assurance (AAL/IAL) との対応                                         |
| step-up  | リスクベース / 適応認証の足場                                                 |
| 復旧     | アカウント復旧フロー                                                          |

### Phase 3 — セッション / OIDC ライフサイクル完成

| 領域       | 不足している機能                                                                                         |
| ---------- | -------------------------------------------------------------------------------------------------------- |
| ユーザー側 | セッション一覧・失効 UI、デバイス管理                                                                    |
| RP 側 SLO  | `id_token_hint` 署名検証・client 解決、Back-Channel Logout、Front-Channel Logout、Session Management 1.0 |
| 継続評価   | CAEP / Shared Signals Framework によるイベント連動セッション失効                                         |

### Phase 4 — 管理 / RBAC / マルチテナンシー

client / consent / key / audit-event の admin CRUD と RBAC (`admin` / `system_admin`
スコープ、`permissions` セクション)、realm / tenant 分離 (`/realms/{tenant_id}` + tenant
管理 API + tenant-scoped persistence)、tenant 別のパスワードポリシー上書き、上記 API の
上に乗る管理 UI (6 領域) は実装済。user は backend の create / read / update / disable と
一覧・ロール編集 UI に加え、属性編集 (`preferred_username` / `name` / `email` /
`email_verified`) の管理 UI も実装済。残るのは削除と group 集約。

| 領域                             | 不足している機能                                                                                                                                                            |
| -------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| ユーザ削除                       | DELETE `/admin/users/:sub` と anonymize cascade (consent / refresh-family / session / password history)、削除確認 UI、`user.deleted` event                                  |
| グループ                         | 新規 aggregate `Group` (tenant-scoped、roles 保持)、user-group membership、admin CRUD API + UI、effective roles = `user.roles ∪ ⋃ group.roles` の解決                       |
| Dynamic Client Registration 拡張 | registration_access_token、software_statement、client metadata 更新・削除（client_secret rotation 本体は Phase 1）                                                          |
| 委譲・代行                       | impersonation、delegation、guest access                                                                                                                                     |

### Phase 5 — 同意 / プライバシー

| 領域           | 不足している機能                                   |
| -------------- | -------------------------------------------------- |
| 同意管理       | 同意管理 UI、同意履歴参照、scope purpose 表示      |
| データ主体権利 | DSAR API（export / delete）                        |
| 保持           | PII purge バッチ、地域別保持ポリシー、データ最小化 |

### Phase 6 — Federation / プロビジョニング

ra-idp 自身が SAML 2.0 / WS-Federation を**しゃべる** outbound 方向と、外部 IdP との
inbound 連携を両方サポートする。SAML 2.0 IdP は現代の B2B SaaS / エンタープライズ
販売で事実上必須要件であり、OIDC のみでは最低ラインを満たさない。

| 領域                                    | 不足している機能                                                                                                                                                         |
| --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| ra-idp が IdP として振る舞う (outbound) | SAML 2.0 IdP（SP-Initiated / IdP-Initiated SSO、metadata 公開、Single Logout、assertion 署名・暗号化、attribute mapping）、WS-Federation Passive Requestor、WS-Trust STS |
| 外部 IdP との連携 (inbound)             | OIDC RP として外部 OIDC IdP、SAML SP として外部 SAML IdP、WS-Fed RP として外部 STS、social login、IdP discovery、broker パターン                                         |
| エンタープライズ (inbound)              | LDAP / AD bind、Kerberos / SPNEGO                                                                                                                                        |
| プロビジョニング                        | JIT provisioning、account linking、SCIM 2.0、deprovisioning、グループ同期                                                                                                |

### Phase 7 — 高保証プロファイル / プロトコル拡張

| 領域           | 不足している機能                                                                       |
| -------------- | -------------------------------------------------------------------------------------- |
| 認可リクエスト | JAR (RFC 9101)、Rich Authorization Requests (RFC 9396)                                 |
| 認可レスポンス | JARM、認可レスポンス署名、encrypted id_token (JWE)                                     |
| トークン       | Token Exchange (RFC 8693)、Resource Indicators (RFC 8707)、pairwise subject identifier |
| 認証フロー     | CIBA (OpenID CIBA Core 1.0)、Step-up Authentication Challenge Protocol (RFC 9470)      |
| FAPI / IDA     | FAPI 2.0 conformance suite、OpenID Connect for Identity Assurance                      |
| 仕様追跡       | OAuth 2.0 Security BCP / OAuth 2.1 の継続追従                                          |

### Phase 8 — 運用 / 可用性 / セキュリティ運用 / コンプライアンス

| 領域             | 不足している機能                                                                                         |
| ---------------- | -------------------------------------------------------------------------------------------------------- |
| 鍵               | HSM / KMS 実鍵管理（Phase 1 の抽象 port を本物に差し替え）                                               |
| 攻撃面           | SSRF 防御、WAF、bot 対策、異常検知（impossible travel 等）、侵害時 token revocation playbook             |
| 可用性           | Kubernetes 配備、監視・アラート、負荷試験、マルチリージョン、zero-downtime migration、バックアップ・リストア演習、DR、容量計画 |
| セキュリティ運用 | ペネトレーションテスト、bug bounty / responsible disclosure、chaos engineering、改竄防止監査ログ         |
| コンプライアンス | OIDC / FAPI certification、SOC2 / ISO27001 証跡、監査レポート、データ処理契約用エクスポート              |

### Phase 9 — 開発者体験 / テスト基盤 / 仕上げ

| 領域                   | 不足している機能                                                                                                                                                                                                                           |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 開発者向け             | SDK、クライアント設定テンプレート、well-known docs、エラー診断・トラブルシュート                                                                                                                                                           |
| SPA E2E スモークテスト | Playwright で `/authorize → login → consent → callback URL に code=` が乗るまでを 1 本のテストとして通す。SPA の DOM 描画と `fetch` の cross-origin redirect 挙動はバックエンドの bun test では捕まらないため、別レイヤが必要 (詳細は下記) |
| プロトコル conformance | OAuth / OIDC conformance smoke suite を CI に常駐                                                                                                                                                                                          |

**SPA E2E スモークテストの最小要件**:

- `@playwright/test` を `ra-idp-go/ui/` の devDependency に追加し、`ui/tests/e2e/` 配下に置く。
- フィクスチャでバックエンドを `memory` モードで起動し、`localhost:8080/callback` 相当のミニマムなコールバック収集サーバを Playwright `globalSetup` で別ポートに立てる。
- シナリオ 1 本:
  1. `http://localhost:3000/authorize?client_id=demo-web-app&...` を開く。
  2. `meta[name="ra-idp:page"][content="login"]` が描画され、`input[name="username"]` が見えることを assert (TanStack Router の dispatcher 回帰防止)。
  3. `alice` / `demo-password-1234` を入力して送信。
  4. consent 画面 (`meta[name="ra-idp:page"][content="consent"]`) が表示されることを assert。
  5. 「許可する」を押し、コールバックサーバが受け取った URL に `code=...&iss=...` が乗ることを assert (`fetch` の cross-origin redirect 挙動の回帰防止)。
- CI では Chromium だけインストール (`npx playwright install chromium`)。

これにより SPA の dispatcher と redirect モードの両方が回帰なく機能していることを 1 テストで保証できる。

# ra-idp-go — IdP の Go 実装

仕様核 `spec/scl.yaml` をもとに Regenerative Architecture に従って開発している IdP アプリケーション。

## 範囲

- 全 SCL バインディング (vocabulary / models / state_machines / objectives / properties / scenarios / permissions / interfaces / annotations)
- 認可エンドポイント `/authorize` + React Login / Consent UI + End Session (`/end_session`)
- ブラウザ認証API `/api/auth/*` + Session Cookie + CSRF
- メールによるパスワードリセット + 単発・30分TTLトークン (ADR-030)
- RBACで保護された管理ユーザーAPI (`/api/admin/users`) + ユーザー無効化 (ADR-031) と削除 (ADR-036, anonymize cascade)
- tenant-scoped なグループ集約 + user-group membership + admin CRUD (`/admin/groups`)、実効ロール `user.roles ∪ ⋃ group.roles` (ADR-038)
- SCL の管理ロール・権限・関連 HTTP interface を閲覧する管理 API / UI (`/api/admin/policy/roles`, `/admin/roles`)
- 所属テナント内 admin 向け設定 UI (`/api/admin/settings`, `/admin/settings`): 表示名 / password_policy_override の閲覧と更新
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
root への cookie 広げが不要になる)。`admin` role の `/api/admin/users` 操作は request
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

### User attribute model

`spec.User` は thin core + sparse attribute bag で構成する (ADR-039 / ADR-040、wi-19)。

- **型付き core**: 識別・認証・表示名 (`sub` / `preferred_username` / `email` /
  `name` 等)・`roles`・`lifecycle`。login / lookup / ID Token / 匿名化で
  頻用する分だけ。
- **lifecycle**: `UserLifecycle.status` (`active` / `disabled` / `deleted` /
  `locked` / `staged` / `suspended`) が無効化・削除の唯一の真実。旧
  `disabled_at` / `deleted_at` は `status` + `status_changed_at` に統合した。
- **attribute bag**: それ以外の OIDC §5.1 optional claim・SCIM 組織属性・
  tenant 定義 custom を `User.attributes` に sparse に格納する。OIDC / 組織の
  組み込み定義は `spec.BuiltinUserAttributeDefs()`、tenant 固有は
  `TenantUserAttributeSchema` が持ち、両者の実効定義に対して値を検証する。
  `pii` 属性は監査で hash 化する。使う属性だけが領域を消費する。
- **claim 露出**: `visibility=claim_exposed` の属性のみ UserInfo / ID Token に
  出る。開示は要求 scope で gating する (`profile` / `phone` / `address`、scope を
  持たない custom 属性は `custom_attribute`)。`address_*` は §5.1.1 の入れ子
  `address` オブジェクトへ再構成する (`spec.ClaimsForScopes`)。
- **編集経路**: admin は `/api/admin/users/{sub}` で属性全体を置換、end-user は
  `/api/account/profile` (self-service) で `editable_by_user=true` の属性のみを
  key 単位 merge で編集する。tenant の custom 定義は
  `/api/admin/tenant/user_attribute_schema` で管理する。

### End-user account portal

end-user 向けの「マイページ」を `/account` 配下に持つ (ADR-042 / wi-21)。admin
コンソールとは別の `AccountShell` で、admin 機能への導線は出さない。認証必須で、
未認証時は `/login` へ誘導し戻り先を保持する (wi-18 と同 pattern)。API は全て
`/api/account/` プレフィックスで、認証済みセッションの `actor.sub` のみを操作対象に
する (cross-user / cross-tenant 参照は不可)。

- **アカウント概要** (`/account`): `/api/account/summary` (`AccountSummary`、roles を
  含まない self 専用契約) を表示。最終ログイン・パスワード最終変更・MFA 状態・
  未対応の required actions を summary card で示す。
- **個人情報** (`/account/profile`): 表示名と `editable_by_user=true` の属性を編集
  (self-service、wi-19 の `/api/account/profile`)。
- **メールアドレス** (`/account/emails`): primary email の変更。新アドレスへワンタイム
  リンクを送り、`/account/email/verify` で確認するまで反映しない (ADR-030 と同じトークン
  方式 / EmailSender)。確認時に `email_verified=true` とし、`verify_email` の required
  action があれば自動解除する。
- **セキュリティ** (`/account/security`): パスワード変更 (`/account/password`) への導線と、
  認証アプリ (TOTP) の self-service 登録・解除。登録時は otpauth URI を QR コード (クライアント
  側生成の SVG) で提示し、スキャンできない場合のセットアップキー手動入力を併置する (wi-40)。
  登録は確認コード、解除は有効な TOTP コードによる所持証明を要求する
  (`POST /api/account/mfa/totp/enroll/start` | `…/enroll/confirm` | `…/remove`)。登録で
  `mfa_enrolled=true`、解除で false に戻す。WebAuthn / SMS OTP は後続ステージ (wi-26)。
- **アクティビティ** (`/account/activity`): 有効なセッションの一覧 (現在のセッションを明示) と
  個別の「終了」/「他のセッションを終了」、および直近のサインイン履歴 (日時 + 認証手段 `amr`) を
  新しい順に表示 (`GET /api/account/sessions` + `GET /api/account/signin_activity`)。IP /
  デバイス / 場所の表示は後続ステージ。
- **接続済みアプリ** (`/account/applications`): アクセスを許可した OAuth クライアント
  (active な Consent) の一覧と、個別の取り消し。取り消すと次回その client の認可で
  consent が再要求される (admin の Consent 取り消しと同じ論理撤回 + `ConsentRevoked`)。
- **データとプライバシー** (`/account/data`): 個人データの JSON エクスポート (GDPR 第15条
  right of access)。本ステージは同期生成で、profile と接続済みアプリ (consents) を含む
  (`GET /api/account/data_export`)。sign-in 履歴・セッションの同梱と非同期ジョブ/オブジェクト
  ストレージ連携は後続ステージ。

self が変更できるのは表示名 / 編集可能属性 / メールアドレス / パスワード / 認証アプリ (TOTP) /
接続済みアプリの取り消しのみ。`roles` / `status` / 組織属性 / `editable_by_user=false` の属性は
admin 専用 (ADR-042)。secondary/recovery email・WebAuthn/SMS の MFA は wi-21 の後続ステージで
追加する。アカウントの削除 / 退会は self-service では提供せず、ライフサイクルは admin 経路
(ADR-036) で管理する。

**step-up 再認証 (ADR-043 / wi-43)**: セッション乗っ取り時の被害を抑えるため、高 sensitivity な
操作 (パスワード変更 / MFA factor の解除 / primary email 変更 / 全セッション失効) は「直近 5 分
以内に password または MFA で再認証済み」であることを要求する。判定の時刻ソースは
`max(auth_time, step_up_at)` で、新規ログイン直後はそのまま step-up 済みとして扱う。未通過は
401 ではなく **403 + `step_up_required`** で返し、UI が再認証 modal を出して成功後に元の操作を
再試行する。再認証は `POST /api/account/step_up/start` (利用可能な factor を取得) →
`…/step_up/complete` (`{ method: "password"|"totp", … }`) で、成立すると session の `step_up_at`
を刻む。対象操作の表は SCL interface の `step_up: required` 注記と機械照合する。

### Authentication event history (サインイン履歴)

wi-20 の最初のスライスとして、サインイン履歴の参照を持つ。本スライスは新テーブルを作らず、
既存の監査イベントストアに蓄積済みの `UserAuthenticated` を発生時刻の降順で射影して返す:

- 本人: `GET /api/account/signin_activity?limit=`（認証済みセッションの `actor.sub` に固定）
- admin: `GET /api/admin/users/{sub}/signin_activity?limit=`（tenant 境界内）

返すのは発生時刻と認証手段 (`amr`) のみ。limit は既定 10 / 上限 50。IP (truncated/hash)・
User-Agent hash・session_id・client_id・country_code・device fingerprint hash・risk_score
等の付加属性は wi-44 で `UserAuthenticated` / `AuthenticationFailed` に後方互換で拡張した
(ADR-041 / ADR-046)。MFA・session・federation・impersonation の各イベント型も wi-44 で
SCL とストレージに用意した (use case 本体は各専用 WI)。end-user 向けの履歴 UI は wi-21 の
activity ステージで本 API を使って描く。

セッション管理 (wi-20 スライス 2) は本人の有効な LoginSession の一覧と失効を持つ:

- `GET  /api/account/sessions`（現在のセッションを `current=true` でマーク、開始時刻の降順）
- `POST /api/account/sessions/{id}/revoke`（本人のセッションのみ。他人のものは 404）
- `POST /api/account/sessions/revoke_others`（現在を除く全セッションを失効 = 他端末からのログアウト）

失効は LoginSession を物理削除して SSO セッションを終了し、`SessionEnded`
(reason: `self_revoke`) を発火する。OAuth クライアントへ発行済みの refresh token は
セッションに 1:1 で紐づかないため本スライスでは失効しない (per-session の refresh 失効と、
admin の全 tenant セッション一覧 UI は後続スライス)。end-user 向けのセッション UI は wi-21 の
activity ステージ (`/account/activity`) で本 API を使って描く。

失敗ログインの集約 (wi-20 スライス 3) は、クレデンシャル試行洪水時に監査の行が爆発する
のを防ぐ。per-account / per-IP の throttle が閾値に到達 (ADR-029 の lockout) すると、以後の
失敗は個別の `AuthenticationFailed` を出さず、`(tenantId, kind, keyHash, 5 分窓)` の bucket へ
集約する。1 つの窓につき `AuthenticationEventAggregated` を 1 件だけ発火し、以後の増分は
bucket の `count` に積む。bucket と throttle は同じ `keyHash` (username / IP の SHA-256) を
共有し、平文は audit に流さない。

- admin: `GET /api/admin/authentication_event_buckets?limit=`（tenant 境界内、新しい窓順。
  permission は `AdminAuditEventsRead` を再利用。limit 既定 50 / 上限 200）

永続ストアと admin 検索 (wi-44) は、上記を in-memory から Postgres に載せ替え、admin が
時系列で調査できる検索を加える (Keycloak の login events / Okta の System Log 相当)。

**認証イベントは監査ログと同一ストア (`audit_events`) の一系統**であり、別テーブル・別 API は
持たない。認証イベントは「監査ログを認証系の type 群に絞り込んだビュー」として扱う
(`signin_activity` と同じ grain)。よって調査は監査ログ検索の `kind` フィルタで行う。

- 監査イベントの読み出しモデル (`audit_events`) と bucket 集約 (`authentication_event_buckets`)
  を Postgres に永続化する (migration 0012)。bucket は upsert 1 回で「窓ごとの件数」と
  「その窓で最初の記録か」を返し、攻撃時も個別 INSERT を出さない (ADR-041)。
- admin 検索: `GET /api/admin/audit_events`（`category`
  = authentication / success / fail / aggregated / user / group / client / consent / token /
  tenant / key、`type` 完全一致、`sub` / `after` / `before` / `limit`）/ `GET .../{id}` /
  `GET .../export`。permission は `AdminAuditEventsRead`。認証系の調査は `category` で絞り込む。
- 保持期間 sweep (ADR-045): 成功 365 日 / 失敗詳細 30 日 / 集約・MFA・セッション 90 日。
  global cap を上限とし、impersonation は短縮しない。`RETENTION_SWEEP_INTERVAL` の周期 job が
  `occurred_at` の古い行を削除する。
- admin UI: `/admin/audit_events` にイベントカテゴリ (`category`、認証サブ分類 + 管理操作)
  の単一セレクト・種別バッジ・JSON エクスポートを統合した (認証専用ページは設けない)。

属性拡張・新規イベント型・bucket・retention は共通基盤として残し、UI/API の切り口だけを
監査ログに一本化した。username / IP を hash・truncated 値で相関検索する機能は、hash 化 (emit 側)
の実装後に「平文を入力 → サーバ側で hash 化して検索」する形で別途追加する (現状は ADR-046 の
フィールド確保のみで値が無いため検索フィルタには出さない)。admin の全テナント横断セッション
一覧 / 失効、GeoIP 連携、SIEM streaming、impersonation 機能本体と本人通知は後続 WI で扱う
(wi-28 / wi-30 ほか)。

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
│   └── src/features/                       UI feature 境界
├── cmd/ra-idp-go/main.go               起動
├── internal/spec/                      Layer 1 バインディング: SCL 構造体 + 状態機械
├── internal/tenancy/                   Layer 3: テナント (domain / ports / usecases)
├── internal/oauth2/                    Layer 3: OAuth2 (domain / ports / usecases)
├── internal/authentication/            Layer 3: 認証 (domain / ports / usecases)
├── internal/platform/                  Layer 4: コンテキスト横断アダプタ
│   ├── crypto/                         Argon2id, PS256, DPoP, private_key_jwt
│   ├── persistence/                    memory / PostgreSQL / Redis（リソース別ファイル）
│   ├── http/                           Echo v5（per-context 分割は wi-48）
│   ├── observability/                  OpenTelemetry
│   ├── policy/                         local / remote AuthZEN
│   ├── notification/                   メール送信
│   └── eventsink/                      console / Kafka relay
├── internal/bootstrap/                 Layer 5: 配線 (DI / seed / server)
└── infra/                              migrations / Docker Compose / OTel Collector
```

> 構造軸 (ADR-047): 水平の5層に加え、垂直の境界づけられたコンテキスト
> (`tenancy` / `authentication` / `oauth2`) で分割する (RA §3.6)。Layer 3 は各
> コンテキストが所有し、コンテキスト横断の Layer 4 アダプタは `internal/platform/`
> に集約する。

## 実装ロードマップ

現状は RA の仕様核・派生物・ユースケース・アダプタ・運用面を備えた IdP だが、
商用・社内共通基盤として本番投入するには次が不足している。依存関係・リスクの
大きさ・RA 的清潔さ（既存 port を完成 → 新 port を追加の順）の 3 軸で
フェーズ分けした実装ロードマップとして示す。

> Phase 番号は依存関係と推奨順序の目安。ユーザ価値が大きい項目は他 Phase より優先してよい。既に実装済みの領域、および `work-items/` で具体化済みの残タスク（HIBP / rate limit・bot 対策 / メール・電話検証 / client_secret・署名鍵 rotation / WebAuthn・recovery codes / セッション・OIDC logout / SAML / inbound federation / SCIM 双方向 / KMS・HSM / conformance / k8s・監視 など）は本表から除外し、まだ work item 化していない長期項目だけを掲載する。

### Phase 2 — MFA / Passkey と acr/amr 体系

| 領域     | 不足している機能                                                     |
| -------- | -------------------------------------------------------------------- |
| 認証手段 | magic link / passwordless email                                      |
| 体系     | identity assurance (AAL/IAL) との対応                                |
| step-up  | リスクベース / 適応認証の足場                                        |
| 復旧     | アカウント復旧フロー（recovery codes / recovery email を束ねる導線） |

### Phase 3 — セッション / OIDC ライフサイクル完成

| 領域       | 不足している機能                                                |
| ---------- | --------------------------------------------------------------- |
| ユーザー側 | デバイス管理                                                    |
| 継続評価   | CAEP / Shared Signals Framework によるイベント連動セッション失効 (Security Event Token / RFC 8417 を transport)。push / receiver 双方向 |

### Phase 4 — 管理 / RBAC / マルチテナンシー

| 領域                             | 不足している機能                                                                                               |
| -------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| Dynamic Client Registration 拡張 | registration_access_token、software_statement、client metadata 更新・削除（client_secret rotation 本体は別 WI） |
| 委譲・代行                       | impersonation、delegation、guest access                                                                        |

### Phase 5 — 同意 / プライバシー

| 領域           | 不足している機能                                   |
| -------------- | -------------------------------------------------- |
| 同意管理       | 同意管理 UI、同意履歴参照、scope purpose 表示      |
| データ主体権利 | DSAR API（export / delete）                        |
| 保持           | PII purge バッチ、地域別保持ポリシー、データ最小化 |

### Phase 6 — Federation / プロビジョニング

SAML 2.0 IdP・inbound federation・SCIM 双方向プロビジョニング (JIT / account
linking 含む) は work item 化済み。残るのはレガシー protocol とエンタープライズ
directory 連携。

| 領域                                    | 不足している機能                             |
| --------------------------------------- | -------------------------------------------- |
| ra-idp が IdP として振る舞う (outbound) | WS-Federation Passive Requestor、WS-Trust STS |
| エンタープライズ (inbound)              | LDAP / AD bind、Kerberos / SPNEGO            |

### Phase 7 — 高保証プロファイル / プロトコル拡張

| 領域           | 不足している機能                                                                       |
| -------------- | -------------------------------------------------------------------------------------- |
| 認可リクエスト | JAR (RFC 9101)、Rich Authorization Requests (RFC 9396)                                 |
| 認可レスポンス | JARM、認可レスポンス署名、encrypted id_token (JWE)                                     |
| トークン       | Token Exchange (RFC 8693)、Resource Indicators (RFC 8707)、pairwise subject identifier |
| 認証フロー     | CIBA (OpenID CIBA Core 1.0)、Step-up Authentication Challenge Protocol (RFC 9470)      |
| FAPI / IDA     | FAPI 2.0 conformance suite、OpenID Connect for Identity Assurance                      |
| 検証可能クレデンシャル | OID4VCI 発行 / OID4VP 検証 (wallet ベース、SD-JWT VC) — [[wi-47-verifiable-credentials-oid4vci-oid4vp]] で work item 化済み |
| ワークロード ID | SPIFFE / SPIRE 連携、workload identity federation (JWT-SVID / X.509-SVID 発行)、non-human identity 管理 |
| 仕様追跡       | OAuth 2.0 Security BCP / OAuth 2.1 の継続追従                                          |

### Phase 8 — 運用 / 可用性 / セキュリティ運用 / コンプライアンス

| 領域             | 不足している機能                                                                            |
| ---------------- | ------------------------------------------------------------------------------------------- |
| 攻撃面           | SSRF 防御、WAF、異常検知（impossible travel 等）、侵害時 token revocation playbook           |
| 可用性           | マルチリージョン、zero-downtime migration、バックアップ・リストア演習、DR、容量計画          |
| セキュリティ運用 | ペネトレーションテスト、bug bounty / responsible disclosure、chaos engineering、改竄防止監査ログ |
| コンプライアンス | OIDC / FAPI certification、SOC2 / ISO27001 証跡、監査レポート、データ処理契約用エクスポート  |

### Phase 9 — 開発者体験 / テスト基盤 / 仕上げ

| 領域       | 不足している機能                                                               |
| ---------- | ------------------------------------------------------------------------------ |
| 開発者向け | SDK、クライアント設定テンプレート、well-known docs、エラー診断・トラブルシュート |

### Phase 10 — AI Agent 向け ID / 認証 / 認可

Okta / Auth0・Microsoft Entra Agent ID・Google・Ping Identity・Keycloak の動向を踏まえ、AI
エージェント (非人間 ID / NHI) を第一級プリンシパルとして扱うための機能を work item 化した。
土台 (エージェント ID → 委譲) から積み上げ、利用シナリオ別に「ユーザー代行アクセス」「自律
ワークロード」「MCP / ツール連携」「ガバナンス / 統制」を満たす。Phase 7 に汎用機能として
挙げていた Token Exchange / RAR / CIBA / workload identity は、本フェーズで AI エージェント
観点に具体化した work item へ集約する。

| 領域 (利用シナリオ)      | 機能 / work item                                                                                                                       |
| ------------------------ | -------------------------------------------------------------------------------------------------------------------------------------- |
| エージェント ID (土台)   | 非人間プリンシパル・所有者・kill-switch — [[wi-49-agent-identity-first-class-principal]]                                                |
| ユーザー代行アクセス     | Token Exchange (RFC 8693) 委譲・actor チェーン — [[wi-50-token-exchange-delegation-actor-chain]]                                        |
| ユーザー代行アクセス     | Rich Authorization Requests (RFC 9396) で権限を束縛 — [[wi-51-rich-authorization-requests-agent-scopes]]                                |
| ユーザー代行アクセス     | CIBA による human-in-the-loop 承認 — [[wi-52-ciba-async-human-approval]]                                                                |
| ユーザー代行アクセス     | 関係ベース細粒度認可 (ReBAC / FGA) — [[wi-53-rebac-fine-grained-authorization]]                                                         |
| 自律ワークロード         | workload identity federation (SPIFFE 互換) — [[wi-54-workload-identity-federation-spiffe]]                                             |
| MCP / ツール連携         | Token Vault / federated connections — [[wi-55-token-vault-federated-connections]]                                                      |
| MCP / ツール連携         | MCP 認可サーバー (OAuth 2.1 / RFC 9728 / 8707) — [[wi-56-mcp-authorization-server]]                                                     |
| MCP / ツール連携         | Cross-App Access (Identity Assertion Grant) — [[wi-57-cross-app-access-identity-assertion-grant]]                                      |
| ガバナンス / 統制        | 継続的アクセス評価 (CAEP / SSF) と即時失効 — [[wi-58-continuous-access-evaluation-agent-revocation]]                                    |
| ガバナンス / 統制        | ガードレール・委譲チェーン監査・インベントリ — [[wi-59-agent-governance-guardrails-audit-inventory]]                                    |

> 依存順: wi-49 (土台) → wi-50 (委譲) → wi-51 / wi-52 / wi-53 (代行の細粒度・承認・データ認可) →
> wi-54 (自律ワークロード) → wi-55 / wi-56 → wi-57 (連携) → wi-58 / wi-59 (統制)。Transaction Tokens
> (Txn-Tokens draft) による内部サービス間の文脈伝播は wi-50 / wi-54 の out_of_scope に将来項目として記録した。

# ra-idp-go — IdP の Go 実装

仕様核 `spec/scl.yaml` をもとに Regenerative Architecture に従って開発している IdP アプリケーション。

## 範囲

OAuth 2.0 / OpenID Connect の認可サーバー兼 IdP として次を備える。各機能の設計判断は
`spec/scl.yaml` と `decisions/` の ADR・`work-items/` の各 work item に対応する。

### プロトコルエンドポイント

- 認可エンドポイント `/authorize`（OAuth 2.0 認可コードフロー RFC 6749 + PKCE RFC 7636、OpenID Connect Core 1.0）。ログイン / 同意画面と、RP 起点ログアウト `/end_session`（OpenID Connect RP-Initiated Logout 1.0）
- トークンエンドポイント `/token`（OAuth 2.0 RFC 6749 の authorization_code / refresh_token / client_credentials、Device Authorization Grant RFC 8628 の device_code の各付与方式）
- プッシュ型認可リクエスト `/par`（RFC 9126）
- デバイス認可付与 `/device_authorization`・`/device`（RFC 8628）
- トークンイントロスペクション `/introspect`（RFC 7662）
- トークン失効 `/revoke`（RFC 7009）
- トークン交換による委譲・代行と委譲チェーン（RFC 8693、`/token` の `urn:ietf:params:oauth:grant-type:token-exchange` 付与方式、Resource Indicators RFC 8707 を制約付きで併用、wi-50）
- リッチ認可リクエスト `authorization_details`（RFC 9396、同意画面での表示・イントロスペクションでの開示・トークン交換時のスコープ縮小・管理用の型レジストリ、wi-51）
- ユーザー情報エンドポイント `/userinfo`（OpenID Connect Core 1.0 §5.3）
- 動的クライアント登録 `/register`（RFC 7591）
- OpenID Connect Discovery `/.well-known/openid-configuration` と JWK Set `/jwks`（OpenID Connect Discovery 1.0、JWK Set RFC 7517）
- DPoP による送信者制約トークン（RFC 9449）
- private_key_jwt クライアント認証（RFC 7523、インライン JWKS / `jwks_uri`）
- WS-Federation passive requestor profile による IdP（IP-STS）`/wsfed`（`wa=wsignin1.0` のブラウザ SSO と `wsignout1.0` / `wsignoutcleanup1.0` のサインアウト、署名済み SAML 2.0 assertion を RSTR に包んで relying party へ自動 POST、wi-61）。relying party は wtrealm で識別し、許可 wreply の閉集合・claim 発行ポリシーを `/api/admin/wsfed/relying-parties` で管理する。claim は宣言的マッピング（ADR-059）、XML 署名は goxmldsig（ADR-060）

### 認証・アカウント・管理

- ブラウザ認証 API `/api/auth/*`（セッション Cookie + CSRF 対策）
- メールによるパスワード再設定（単発・30 分 TTL のトークン、ADR-030）
- ロールベースアクセス制御で保護した管理ユーザー API `/api/admin/users`、ユーザーの無効化（ADR-031）と削除（ADR-036、匿名化カスケード）
- グループ集約・ユーザーとグループの所属関係・管理 CRUD `/admin/groups`（テナント内に閉じる、実効ロール `user.roles ∪ ⋃ group.roles`、ADR-038）
- ロール・権限と関連 HTTP インターフェースを閲覧する管理 API / UI `/api/admin/policy/roles`・`/admin/roles`
- テナント内の管理設定 UI `/api/admin/settings`・`/admin/settings`（表示名・パスワードポリシー上書きの閲覧と更新）
- 管理クライアント CRUD `/admin/clients`（テナント内に閉じる）
- 同意の参照・撤回 API `/admin/consents`（テナント内に閉じる）
- AuthZEN によるポリシー評価（ローカル / リモート）
- AI エージェントを第一級の非人間プリンシパルとして扱う土台（所有者・緊急停止、wi-49 / セキュリティ是正 wi-60）

### テナント・基盤・運用

- `/realms/{tenant_id}` によるテナント分離、テナント管理 API、テナント単位の永続化（ADR-032〜034）
- リフレッシュトークンのローテーションとファミリー失効（ADR-004）
- PS256 による JWT 署名と JWK Set、メモリ / PostgreSQL の鍵ストア（RFC 7638 サムプリントの `kid`）
- PostgreSQL の永続状態と Valkey の揮発状態
- PostgreSQL アウトボックスと Kafka リレー（`ra-idp-relay`）
- OpenTelemetry OTLP/HTTP によるトレース / メトリクス
- ドメインイベントの発火（コンソール / アウトボックスのイベントシンク）
- Zog スキーマによるモデル・HTTP 入力・パスワードポリシーの検証

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

起動直後のログに `email sender: smtp host=127.0.0.1 port=1025 tls=none from=...`
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
VALKEY_URL='valkey://localhost:6379/0' \
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
| `DATABASE_URL`       | PostgreSQL 接続先。`postgres` 時に必須                                            |
| `VALKEY_URL`         | Valkey 接続先。`postgres` 時に必須                                                |
| `AUTO_MIGRATE`       | 起動時のマイグレーション (`true`)                                                 |
| `MIGRATIONS_DIR`     | `infra/migrations`                                                                |
| `EVENT_SINK`         | `console` / `outbox` (`console`)                                                  |
| `OBSERVABILITY`      | `noop` / `otel` (`noop`)                                                          |
| `AUTHZEN`            | `local` / `remote` (`local`)                                                      |
| `AUTHZEN_URL`        | リモート AuthZEN のベース URL                                                     |
| `KAFKA_BROKERS`      | リレー用のカンマ区切りブローカー                                                  |
| `SKIP_DEMO_SEED`     | 設定時はデモデータを保存しない                                                    |
| `LEGACY_BARE_ISSUER` | `true` の場合だけ接頭辞なしルートの既定 issuer を旧 `{base}` 形式にする (`false`) |
| `EMAIL_SENDER`       | `console` / `smtp` (`console`)。`smtp` 選択時は SMTP\_\* を読む                   |
| `SMTP_HOST`          | SMTP リレーホスト。`smtp` 時に必須                                                |
| `SMTP_PORT`          | ポート (`SMTP_TLS` 既定値: starttls→587 / implicit→465 / none→25)                 |
| `SMTP_USERNAME`      | PLAIN auth ユーザ名 (空なら認証なし)                                              |
| `SMTP_PASSWORD`      | PLAIN auth パスワード。ログには出さない (ADR-035 §10)                             |
| `SMTP_FROM`          | RFC 5322 `From:` / SMTP `MAIL FROM`。`smtp` 時に必須 (bare address)               |
| `SMTP_HELO`          | EHLO/HELO で使うローカル名 (`localhost`)                                          |
| `SMTP_TLS`           | `starttls` / `implicit` / `none` (`starttls`)。`none` は開発専用                  |
| `SMTP_TIMEOUT_SECONDS` | 接続とコマンドのタイムアウト (`10`)                                             |

`jwks_uri` は HTTPS のみ許可し、プライベート / ループバック / リンクローカルアドレス、
userinfo、フラグメントを拒否する。取得は 3 秒タイムアウト、1 MiB 上限、5 分キャッシュとする。

構造体の整合性と外部入力の型変換・形式検証には
[Zog](https://zog.dev/) を使う。登録済みリダイレクト URI との一致、スコープ許可、
状態遷移、PKCE など実行時コンテキストを必要とする検証はユースケース / ドメイン層に置く。

## 検証

```bash
go test -race ./...
golangci-lint run
```

テストは認可コードのアトミックな引き換え、Valkey Lua、リフレッシュファミリー失効、デバイスフロー、クライアント認証、`jwks_uri` の SSRF 防御、AuthZEN のワイヤ契約、イベントのワイヤ形式を含む。

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
├── internal/tenancy/                   Layer 3+4: テナント (domain / ports / usecases / adapters/http)
├── internal/oauth2/                    Layer 3+4: OAuth2 (domain / ports / usecases / adapters/http)
├── internal/authentication/            Layer 3+4: 認証 (domain / ports / usecases / adapters/http)
├── internal/platform/                  Layer 4: コンテキスト横断アダプタ
│   ├── crypto/                         Argon2id, PS256, DPoP, private_key_jwt
│   ├── persistence/                    memory / PostgreSQL / Valkey（リソース別ファイル）
│   ├── http/                           Echo v5 router + core（各 context の adapters/http を集約）
│   ├── observability/                  OpenTelemetry
│   ├── policy/                         local / remote AuthZEN
│   ├── notification/                   メール送信
│   └── eventsink/                      console / Kafka relay
├── internal/bootstrap/                 Layer 5: 配線 (DI / seed / server)
└── infra/                              migrations / Docker Compose / OTel Collector
```

> 構造軸 (ADR-047): 水平の5層に加え、垂直の境界づけられたコンテキスト
> (`tenancy` / `authentication` / `oauth2`) で分割する (RA §3.6)。Layer 3 と HTTP
> アダプタ (`adapters/http`) は各コンテキストが所有し、HTTP の共有基盤
> (依存集約 `core.Deps`・テナント解決 middleware・横断ヘルパ) は
> `internal/platform/http/core` に、コンテキスト横断のその他 Layer 4 アダプタは
> `internal/platform/` に集約する。`internal/platform/http` は各コンテキストの
> `RegisterRoutes` を束ねる router (wi-48)。

## 実装ロードマップ

RA の仕様核・派生物・ユースケース・アダプタ・運用面を備えた IdP だが、商用・社内共通基盤と
して本番投入するにはまだ機能が不足している。ここには、まだ work item 化していない長期的な
不足機能を領域ごとに挙げる。各領域はおおむね基盤に近い順に並べているが、ユーザー価値が
大きい項目を優先してよい。

### 認証・MFA・アカウント復旧

| 領域       | 不足している機能                                                       |
| ---------- | ---------------------------------------------------------------------- |
| 認証手段   | マジックリンク / パスワードレスメール                                  |
| 保証レベル | identity assurance (AAL / IAL) との対応                                |
| 適応認証   | リスクベース / 適応認証の足場（再認証本体は wi-43 で実装済み）          |
| 復旧       | アカウント復旧フローの統合導線（部品のリカバリコードは wi-26、リカバリメールは wi-41） |

### 管理・クライアント・委譲

| 領域                     | 不足している機能                                                                                       |
| ------------------------ | ------------------------------------------------------------------------------------------------------ |
| 動的クライアント登録拡張 | registration_access_token、software_statement、クライアントメタデータの更新・削除（client_secret ローテーション本体は wi-25） |
| 委譲・代行               | impersonation、ゲストアクセス（委譲 / 委譲チェーンは wi-50 で実装済み）                                 |

### 同意・プライバシー

| 領域           | 不足している機能                                                       |
| -------------- | ---------------------------------------------------------------------- |
| 同意管理       | 取得目的（scope purpose）の表示、目的別の同意グルーピング              |
| データ主体権利 | DSAR の非同期エクスポート / オブジェクトストレージ連携・完全削除の証跡 |
| 保持           | 地域別保持ポリシー、データ最小化の体系化                               |

### フェデレーション・プロビジョニング

| 領域                          | 不足している機能                              |
| ----------------------------- | --------------------------------------------- |
| IdP として振る舞う (outbound) | WS-Federation Passive Requestor（wi-61）、WS-Trust STS（wi-62）、federation metadata / claim mapping（wi-63）、Entra domain federation / Microsoft 365 SSO（wi-64、Hybrid Join デバイス登録は Okta 同様に範囲外）|
| エンタープライズ (inbound)    | Kerberos / SPNEGO inbound・無音 SSO（passive WIA / エージェントレス Desktop SSO）（wi-65）、LDAP / AD bind |

### プロトコル拡張・高保証プロファイル

| 領域           | 不足している機能                                                                       |
| -------------- | -------------------------------------------------------------------------------------- |
| 認可リクエスト | JAR (RFC 9101)                                                                         |
| 認可レスポンス | JARM、認可レスポンス署名、暗号化 ID Token (JWE)                                        |
| トークン       | Resource Indicators (RFC 8707) の汎用化（現状はトークン交換内に制約付きで実装）、pairwise subject identifier |
| 認証フロー     | Step-up Authentication Challenge Protocol (RFC 9470)                                   |
| FAPI / IDA     | OpenID Connect for Identity Assurance（FAPI conformance suite 本体は wi-33）           |
| 仕様追跡       | OAuth 2.0 Security BCP / OAuth 2.1 の継続追従                                          |

### 運用・可用性・セキュリティ運用・コンプライアンス

| 領域             | 不足している機能                                                                            |
| ---------------- | ------------------------------------------------------------------------------------------- |
| 攻撃面           | WAF、異常検知（impossible travel 等）、侵害時のトークン失効プレイブック（`jwks_uri` の SSRF 防御は実装済み） |
| 可用性           | マルチリージョン、無停止マイグレーション、バックアップ・リストア演習、DR、容量計画           |
| セキュリティ運用 | ペネトレーションテスト、bug bounty / responsible disclosure、chaos engineering、改竄防止監査ログ |
| コンプライアンス | OIDC / FAPI certification、SOC2 / ISO27001 証跡、監査レポート、データ処理契約用エクスポート  |

# ra-idp-go — Go 実装

`ra-idp/` の TypeScript (Bun) 実装と並行して、同じ仕様核 (`spec/scl.yaml` を symlink で共有) から OAuth 2.0 / OpenID Connect の core を Go で組み上げた。

## 範囲

TS 側と同じスコープを目標に実装した:

- 全 SCL バインディング (vocabulary / models / state_machines / objectives / properties / scenarios / permissions / interfaces / annotations)
- 認可エンドポイント `/authorize` + React Login / Consent UI + End Session (`/end_session`)
- ブラウザ認証API `/api/auth/*` + Session Cookie + CSRF
- メールによるパスワードリセット + 単発・30分TTLトークン (ADR-030)
- RBACで保護された管理ユーザーAPI (`/admin/users`) + ユーザー無効化 (ADR-031)
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

| 環境変数         | 値 / 既定値                          |
| ---------------- | ------------------------------------ |
| `PERSISTENCE`    | `memory` / `postgres` (`memory`)     |
| `DATABASE_URL`   | PostgreSQL接続先。`postgres`時に必須 |
| `REDIS_URL`      | Redis接続先。`postgres`時に必須      |
| `AUTO_MIGRATE`   | 起動時migration (`true`)             |
| `MIGRATIONS_DIR` | `infra/migrations`                   |
| `EVENT_SINK`     | `console` / `outbox` (`console`)     |
| `OBSERVABILITY`  | `noop` / `otel` (`noop`)             |
| `AUTHZEN`        | `local` / `remote` (`local`)         |
| `AUTHZEN_URL`    | remote AuthZENのbase URL             |
| `KAFKA_BROKERS`  | relay用comma-separated broker        |
| `SKIP_DEMO_SEED` | 設定時はデモデータを保存しない       |

`jwks_uri` はHTTPSのみ許可し、private / loopback / link-localアドレス、
userinfo、fragmentを拒否する。取得は3秒timeout、1 MiB上限、5分cacheとする。

構造体の整合性と外部入力の型変換・形式検証には
[Zog](https://zog.dev/) を使う。登録済みredirect URIとの一致、スコープ許可、
状態遷移、PKCEなど実行時コンテキストを必要とする検証はusecase/domainに置く。

## 検証

```bash
go test -race ./...
go vet ./...
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
├── spec/        → ../ra-idp/spec       (symlink — SCL を TS と共有)
├── decisions/   → ../ra-idp/decisions  (symlink — ADR を共有)
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

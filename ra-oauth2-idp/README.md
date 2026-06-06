# RA OAuth2 / OIDC IdP

OAuth 2.0 / OpenID Connect の Identity Provider を題材に、Regenerative Architecture
（リポジトリ root の `REGENERATIVE_ARCHITECTURE.md`）の設計原則をセキュリティ要件の濃い
ドメインで実証する IdP アプリケーション。

仕様核は `spec/scl.yaml` 1 ファイルに集約されている（SCL の語彙・形式は
`SPECIFICATION_CORE_LANGUAGE.md` を参照）。本 README は本アプリの構成と
実行方法、および本アプリ固有の意思決定（ADR）の索引を扱う。

---

## ディレクトリ構成

```text
ra-oauth2-idp/
├── spec/scl.yaml         ← Layer 1: 仕様核（単一ファイル）
├── gen/                  ← Layer 1 派生物（bun run gen:scl で再生成）
├── decisions/            ← Layer 2: ADR
├── src/                  ← Layer 3: spec-bindings / domain / ports / usecases
├── adapters/             ← Layer 4: http / crypto / persistence / policy
├── infra/                ← Layer 5: migrations / event-routing / scripts /
│                            observability / load-tests / docker / k8s / event-relay
└── main.ts               ← Layer 5: 起動
```

保存対象は `spec/scl.yaml` ・ `decisions/` ・ `infra/migrations/` ・
`infra/event-routing.yaml` ・ `infra/scripts/`。それ以外は派生物
（`gen/` ・ `src/` ・ `adapters/` ・ `infra/observability/` ・
`infra/load-tests/` ・ `main.ts` ・ `infra/docker/` ・ `infra/k8s/`）。

---

## サポートする RFC / 仕様

| 仕様               | 実装範囲                                                                            |
| ------------------ | ----------------------------------------------------------------------------------- |
| RFC 6749           | OAuth 2.0 — `authorization_code`, `refresh_token`, `client_credentials`             |
| RFC 6750           | Bearer Token Usage — `/userinfo` で受領                                             |
| RFC 7009           | Token Revocation — `/revoke`                                                        |
| RFC 7519           | JSON Web Token — access_token / id_token を JWT で発行                              |
| RFC 7523           | private_key_jwt クライアント認証（ADR-023）                                         |
| RFC 7591           | Dynamic Client Registration — `/register`                                           |
| RFC 7636           | PKCE — すべてのフローで必須（ADR-002）                                              |
| RFC 7662           | Token Introspection — `/introspect`                                                 |
| RFC 8414           | OAuth 2.0 Authorization Server Metadata                                             |
| RFC 8628           | Device Authorization Grant（ADR-025）                                               |
| RFC 9068           | JWT Profile for Access Tokens — `typ: "at+jwt"`                                     |
| RFC 9126           | Pushed Authorization Requests — `/par`                                              |
| RFC 9449           | DPoP — ヘッダー検証 + `cnf.jkt` バインド                                            |
| RFC 9700           | OAuth 2.0 Security BCP — 認可コード再利用検知 + ファミリー失効                      |
| OIDC Core 1.0      | `id_token` / nonce / at_hash / `/userinfo`                                          |
| OIDC Discovery 1.0 | `/.well-known/openid-configuration`                                                 |
| FAPI 2.0           | PAR 必須・PS256/ES256・センダー制約（ADR-006）                                      |

---

## リアルワールド IdP として不足している主な機能

現状は RA の仕様核・派生物・ユースケース・アダプタ・運用面を備えた IdP だが、
商用・社内共通基盤として本番投入するには次が不足している。

| 領域 | 不足している機能 |
| ---- | ---------------- |
| ユーザー認証 | パスワードログイン UI、MFA/WebAuthn、パスキー、アカウント復旧、セッション Cookie、CSRF 防御、ログイン試行レート制限 |
| OIDC セッション | `prompt=none`、`max_age`、`acr_values`、`id_token_hint`、RP-Initiated Logout、Back-Channel Logout、Front-Channel Logout、Session Management |
| Federation | SAML / OIDC 外部 IdP 連携、social login、JIT provisioning、IdP discovery、account linking |
| 管理機能 | 管理 API / 管理 UI、ユーザー・クライアント・同意・鍵・監査イベントの運用画面、RBAC/ABAC 管理者権限 |
| Dynamic Client Registration | 登録アクセストークン、software_statement、登録後の client metadata 更新・削除、client_secret rotation |
| トークン運用 | access token denylist、JWT / opaque token の本番選択、token exchange (RFC 8693)、JWT Secured Authorization Response Mode (JARM)、JWT Secured Authorization Request (JAR) |
| センダー制約 | mTLS の実証明書検証、証明書バインド、DPoP nonce、DPoP proof の HTTP adapter 全経路適用 |
| FAPI / 高保証 | FAPI 2.0 conformance suite、PAR/JAR/JARM 組み合わせ、認可レスポンス署名、金融 API 向け詳細 profile 検証 |
| SCIM / ライフサイクル | SCIM 2.0、ユーザープロビジョニング、deprovisioning、グループ同期、組織/テナント管理 |
| Consent / Privacy | 同意管理 UI、同意履歴参照、scope purpose 表示、データ主体要求、PII purge バッチ、地域別保持ポリシー |
| セキュリティ運用 | HSM/KMS 実鍵管理、secret rotation、SSRF 防御、WAF/rate limit、bot 対策、異常検知、侵害時 token revocation playbook |
| 可用性 / 運用 | マルチリージョン、zero-downtime migration、バックアップ/リストア演習、DR、容量計画、SLO burn-rate alert |
| コンプライアンス | OIDC/FAPI certification、SOC2/ISO27001 証跡、監査レポート、データ処理契約に対応するエクスポート |
| 開発者体験 | 管理コンソール、SDK、クライアント設定テンプレート、well-known docs、エラー診断、conformance smoke suite |

---

## エンドポイント

| メソッド | パス                                    | 説明                                              |
| -------- | --------------------------------------- | ------------------------------------------------- |
| GET      | /.well-known/openid-configuration       | OIDC Discovery                                    |
| GET      | /.well-known/oauth-authorization-server | OAuth 2.0 AS Metadata                             |
| GET      | /jwks                                   | 公開鍵 (JWKS)                                     |
| POST     | /register                               | Dynamic Client Registration                       |
| POST     | /device_authorization                   | Device Authorization Request                      |
| GET/POST | /device                                 | デバイス認可の verification_uri                   |
| GET      | /authorize                              | 認可エンドポイント                                |
| POST     | /consent                                | コンセント UI のフォーム受領                      |
| POST     | /par                                    | Pushed Authorization Request                      |
| POST     | /token                                  | トークンエンドポイント                            |
| POST     | /introspect                             | トークン introspection                            |
| POST     | /revoke                                 | トークン失効                                      |
| GET/POST | /userinfo                               | UserInfo (OIDC)                                   |
| GET      | /health                                 | ヘルスチェック                                    |
| GET      | /events                                 | 監査イベントログ（デモ用）                        |

---

## 認可ポリシー

すべての認可規則は `spec/scl.yaml` の `permissions` セクションに集中する。各 HTTP
エンドポイントは `evaluate({ subject, action, resource, context })` を呼ぶだけ。

```text
authorize:initiate                   client_registered / redirect_uri_registered /
                                     scope_subset_of_client_scope / pkce_present /
                                     par_required_if_fapi
token:grant_authorization_code       client_must_declare_grant / pkce_verification_passed /
                                     redirect_uri_exact_match /
                                     code_not_redeemed / code_not_expired
token:grant_refresh                  client_must_declare_grant / token_active /
                                     token_within_absolute_ttl / sender_constraint_satisfied
token:grant_client_credentials       client_is_confidential / client_must_declare_grant
userinfo:read                        token_has_openid_scope / token_active
```

ローカル評価エンジンを **リモート AuthZEN / OPA / Cedar に差し替える** には
`adapters/policy/local-authzen-adapter.ts` の `authorize()` だけを変える（ADR-010）。

---

## 開発・実行

```bash
bun install
bun test                  # 141 tests（invariants / golden / property / policy / contract）
bun run typecheck
bun run lint
bun run gen:all           # gen:scl → gen:prometheus → gen:grafana → gen:k6
bun run check:coherence
bun run dev               # in-memory adapters でサーバ起動
./demo.sh                 # 別ターミナル: Discovery 〜 監査ログまで通し実行
```

`bun run dev` 起動後はデモクライアント `demo-web-app` とデモユーザー `alice`
（sub=`user_alice`）が登録された状態で待ち受ける。`./demo.sh` は Discovery /
JWKS 取得 → Authorization Code + PKCE → token 一式 → refresh ローテーション →
旧 refresh 再利用でファミリー失効 → 認可コード再利用検知 → client_credentials →
PAR → revoke → 監査ログまでを順に確認する。

### Postgres + Redis（durable + volatile state）

```bash
docker compose -f infra/docker/docker-compose.dev.yaml up -d postgres redis
export DATABASE_URL=postgres://idp:idp@localhost:5432/idp
export REDIS_URL=redis://localhost:6379
bun run migrate:up
PERSISTENCE=postgres EVENT_SINK=outbox bun run dev
```

InMemory / Postgres+Redis を同じ契約テスト群（`src/spec-bindings/persistence-contract.test.ts`）
で検証することで「アダプタは差し替えられる」を実コードで担保する（ADR-003 / ADR-016）。

### Kafka 配送

```bash
docker compose -f infra/docker/docker-compose.dev.yaml --profile with-kafka up -d
KAFKA_BROKERS=localhost:9092 DATABASE_URL=... bun run event-relay &
```

outbox → Kafka リレーは `infra/event-relay/main.ts`。トピック割り当ては
`infra/event-routing.yaml` が権威。

### 観測

```bash
OBSERVABILITY=otel \
  OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318 \
  bun run dev
docker compose -f infra/docker/docker-compose.dev.yaml --profile obs up -d
# Grafana http://localhost:3001 / Prometheus http://localhost:9090 / Jaeger http://localhost:16686
```

監査ログは EventSink → outbox → Kafka → SIEM、アプリログは stdout JSON + OTel logger
の 2 系統に分離（ADR-018）。PII フィールドはアプリログでは自動 redact。

### コンテナ / Kubernetes

```bash
docker build -f infra/docker/Dockerfile -t ra-oauth2-idp:dev .
kubectl apply -k infra/k8s/overlays/dev
kubectl apply -k infra/k8s/overlays/prod    # Argo Rollouts canary + SLO-aware ロールバック (ADR-021)
```

multi-stage build (Bun builder → distroless runtime) / non-root / multi-arch。
base manifests は Deployment + HPA + PodDisruptionBudget + NetworkPolicy +
ServiceMonitor + Rollout（prod のみ）。

### CI / CD (`.github/workflows/ra-oauth2-idp-*.yaml`)

| ワークフロー   | トリガー            | 内容                                                                                |
| -------------- | ------------------- | ----------------------------------------------------------------------------------- |
| `ci.yaml`      | PR / push to main   | typecheck + 全テスト + 派生物 drift 検知 + Trivy + CodeQL                           |
| `nightly.yaml` | cron 03:00 JST      | k6 負荷試験 (SLO 違反で fail)                                                       |
| `release.yaml` | tag `ra-oauth2-idp-v*` | multi-arch build → cosign 署名 → CycloneDX SBOM 添付 → SLSA Level 3 provenance 生成 |

---

## 意思決定の索引（ADR）

参照実装固有の意思決定はすべて `decisions/` の ADR にある。RA / SCL の採用そのものに
関する選択は本アプリでは前提として扱い、ADR として残していない。

| カテゴリ           | ADR                                                                       |
| ------------------ | ------------------------------------------------------------------------- |
| 状態機械           | ADR-001 認可リクエスト / デバイスコードの状態機械                         |
| プロトコル必須事項 | ADR-002 PKCE 必須 / ADR-006 FAPI クライアント PAR 必須                    |
| トークン           | ADR-003 JWT 署名アルゴリズム / ADR-004 refresh ローテーション / ADR-012 opaque vs JWT |
| センダー制約       | ADR-005 DPoP をデフォルト                                                 |
| コンセント         | ADR-007 コンセントモデル                                                  |
| クライアント認証   | ADR-008 認証方式 / ADR-023 private_key_jwt 検証                           |
| 鍵管理             | ADR-009 鍵回転 / ADR-024 署名鍵を durable + 共有                          |
| 認可ポリシー       | ADR-010 AuthZEN スタイル評価                                              |
| メタデータ         | ADR-011 Discovery は派生物                                                |
| 永続化             | ADR-016 アダプタ選定（memory / postgres / redis）                         |
| 観測               | ADR-017 OpenTelemetry / ADR-018 監査 vs アプリログ                        |
| 運用               | ADR-019 ランタイム選定 / ADR-020 サプライチェーン保護 / ADR-021 段階的配送 |
| デバイスフロー     | ADR-025 Device Authorization Grant                                        |

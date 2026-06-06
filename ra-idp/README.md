# RA IdP

Identity Provider を題材に、Regenerative Architecture（リポジトリ root の
`REGENERATIVE_ARCHITECTURE.md`）の設計原則をセキュリティ要件の濃いドメインで実証する
IdP アプリケーション。現状の対応プロトコルは OAuth 2.0 / OpenID Connect。
SAML 2.0 / WS-Federation の追加もコンポーネント境界として想定している（ログインセッションは
プロトコル非依存に統一）。

仕様核は `spec/scl.yaml` 1 ファイルに集約されている（SCL の語彙・形式は
`SPECIFICATION_CORE_LANGUAGE.md` を参照）。本 README は本アプリの構成と
実行方法、および本アプリ固有の意思決定（ADR）の索引を扱う。

---

## ディレクトリ構成

```text
ra-idp/
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
| OIDC Core 1.0      | `id_token` / nonce / at_hash / `/userinfo` / `prompt=none` / `prompt=login` / `max_age` |
| OIDC Discovery 1.0 | `/.well-known/openid-configuration`                                                 |
| OIDC RP-Initiated Logout 1.0 | `/end_session` v1 — `post_logout_redirect_uri` 検証 + `state` 伝播         |
| FAPI 2.0           | PAR 必須・PS256/ES256・センダー制約（ADR-006）                                      |

---

## 不足している主な機能と実装ロードマップ

現状は RA の仕様核・派生物・ユースケース・アダプタ・運用面を備えた IdP だが、
商用・社内共通基盤として本番投入するには次が不足している。依存関係・リスクの
大きさ・RA 的清潔さ（既存 port を完成 → 新 port を追加の順）の 3 軸で
フェーズ分けした実装ロードマップとして示す。

### Phase 0 — 認証の土台

| 領域 | 不足している機能 |
| ---- | ---------------- |
| パスワード hashing | Argon2id 等の本番用 hashing |
| パスワードポリシー | 最小長 / 最大長、文字種要件（記号・大文字小文字混在など、組織ポリシーや規制依存）、ユーザー名・メールアドレスとの類似禁止、よくあるパスワード辞書チェック、直近 N 件のパスワード履歴再利用禁止 |
| 漏洩パスワード検査 | HIBP k-anonymity 等のオンライン漏洩データベース連携 |
| ブルートフォース防御 | per-account / per-IP のログイン試行レート制限、CAPTCHA / 行動分析、ユーザー名列挙対策 |
| エンドポイント保護 | `/token` `/authorize` `/par` `/device_authorization` の一般 rate limit / bot 対策 |
| アカウント整合性 | メール・電話番号検証 |

### Phase 1 — 既存仕様の運用穴埋めと規格適合性

| 領域 | 不足している機能 |
| ---- | ---------------- |
| Token | access token denylist（JWT 即時失効）、AS Issuer Identification (RFC 9207, mix-up 防御) |
| DPoP | DPoP nonce、DPoP proof の HTTP adapter 全経路適用 |
| mTLS | クライアント証明書の実検証、証明書バインド |
| Secret / 鍵 | client_secret rotation、署名鍵 rotation の運用化（実 KMS/HSM 差し替えは Phase 9） |
| 規格適合性 | ~~PKCE 必須要件の階段化（ADR-002 改訂: public client 必須・FAPI クライアント必須・confidential client は推奨）。client metadata `require_pkce` の導入、`permissions` の `pkce_present` を client type 条件付きに変更~~ ✅ 実装済 (ADR-002 改訂) |

### Phase 2 — UI / フロントエンド基盤

| 領域 | 不足している機能 |
| ---- | ---------------- |
| デザイン基盤 | デザインシステム、コンポーネントライブラリ、レイアウトテンプレート |
| 国際化 / アクセシビリティ | i18n、WCAG 2.x AA 準拠 (a11y) |
| 既存 UI のリアルワールド化 | ログイン画面、同意 (consent) 画面、デバイス認可 (verification_uri) 画面、エラー / 中断画面 |
| ブランディング基盤 | ロゴ・カラー・文言の差し替え機構（テナント単位の適用は Phase 5 と接続） |

### Phase 3 — MFA / Passkey と acr/amr 体系

| 領域 | 不足している機能 |
| ---- | ---------------- |
| 認証手段 | WebAuthn / Passkey、TOTP、バックアップコード、magic link / passwordless email |
| 体系 | acr/amr 体系の SCL 確立、identity assurance (AAL/IAL) との対応 |
| step-up | `acr_values` / `max_age` を消費する再認証、リスクベース / 適応認証の足場 |
| 復旧 | アカウント復旧フロー |

### Phase 4 — セッション / OIDC ライフサイクル完成

| 領域 | 不足している機能 |
| ---- | ---------------- |
| ユーザー側 | セッション一覧・失効 UI、デバイス管理 |
| RP 側 SLO | `id_token_hint` 署名検証・client 解決、Back-Channel Logout、Front-Channel Logout、Session Management 1.0 |
| 継続評価 | CAEP / Shared Signals Framework によるイベント連動セッション失効 |

### Phase 5 — 管理 / RBAC / マルチテナンシー

| 領域 | 不足している機能 |
| ---- | ---------------- |
| 管理 API | user / client / consent / key / audit-event の CRUD |
| 認可 | RBAC/ABAC（`permissions` セクションに admin scope を追加） |
| Dynamic Client Registration 拡張 | registration_access_token、software_statement、client metadata 更新・削除、client_secret rotation |
| 委譲・代行 | impersonation、delegation、guest access |
| テナント | realm / tenant 分離（client / user / 鍵 / ポリシー / ブランディングをテナント単位に） |
| 管理 UI | 上記 API の上の運用画面 |

### Phase 6 — 同意 / プライバシー

| 領域 | 不足している機能 |
| ---- | ---------------- |
| 同意管理 | 同意管理 UI、同意履歴参照、scope purpose 表示 |
| データ主体権利 | DSAR API（export / delete） |
| 保持 | PII purge バッチ、地域別保持ポリシー、データ最小化 |

### Phase 7 — Federation / プロビジョニング

ra-idp 自身が SAML 2.0 / WS-Federation を**しゃべる** outbound 方向と、外部 IdP との
inbound 連携を両方サポートする。SAML 2.0 IdP は現代の B2B SaaS / エンタープライズ
販売で事実上必須要件であり、OIDC のみでは最低ラインを満たさない。

| 領域 | 不足している機能 |
| ---- | ---------------- |
| ra-idp が IdP として振る舞う (outbound) | SAML 2.0 IdP（SP-Initiated / IdP-Initiated SSO、metadata 公開、Single Logout、assertion 署名・暗号化、attribute mapping）、WS-Federation Passive Requestor、WS-Trust STS |
| 外部 IdP との連携 (inbound) | OIDC RP として外部 OIDC IdP、SAML SP として外部 SAML IdP、WS-Fed RP として外部 STS、social login、IdP discovery、broker パターン |
| エンタープライズ (inbound) | LDAP / AD bind、Kerberos / SPNEGO |
| プロビジョニング | JIT provisioning、account linking、SCIM 2.0、deprovisioning、グループ同期 |

### Phase 8 — 高保証プロファイル / プロトコル拡張

| 領域 | 不足している機能 |
| ---- | ---------------- |
| 認可リクエスト | JAR (RFC 9101)、Rich Authorization Requests (RFC 9396) |
| 認可レスポンス | JARM、認可レスポンス署名、encrypted id_token (JWE) |
| トークン | Token Exchange (RFC 8693)、Resource Indicators (RFC 8707)、pairwise subject identifier |
| 認証フロー | CIBA (OpenID CIBA Core 1.0)、Step-up Authentication Challenge Protocol (RFC 9470) |
| FAPI / IDA | FAPI 2.0 conformance suite、OpenID Connect for Identity Assurance |
| 仕様追跡 | OAuth 2.0 Security BCP / OAuth 2.1 の継続追従 |

### Phase 9 — 運用 / 可用性 / セキュリティ運用 / コンプライアンス

| 領域 | 不足している機能 |
| ---- | ---------------- |
| 鍵 | HSM / KMS 実鍵管理（Phase 1 の抽象 port を本物に差し替え） |
| 攻撃面 | SSRF 防御、WAF、bot 対策、異常検知（impossible travel 等）、侵害時 token revocation playbook |
| 可用性 | マルチリージョン、zero-downtime migration、バックアップ・リストア演習、DR、容量計画、SLO burn-rate alert |
| セキュリティ運用 | ペネトレーションテスト、bug bounty / responsible disclosure、chaos engineering、改竄防止監査ログ |
| コンプライアンス | OIDC / FAPI certification、SOC2 / ISO27001 証跡、監査レポート、データ処理契約用エクスポート |

### Phase 10 — 開発者体験 / 仕上げ

| 領域 | 不足している機能 |
| ---- | ---------------- |
| 開発者向け | SDK、クライアント設定テンプレート、well-known docs、エラー診断・トラブルシュート |
| テスト | conformance smoke suite を CI に常駐 |

#### 順序の根拠

- **Phase 0 → 1 → 2 → 3 は順序固定**: acr/amr 語彙を SCL に入れる前に Federation / SLO を組むと後で書き直しになる。UI 基盤も Phase 3 以降で大量に増える認証 UI（WebAuthn 登録、デバイス一覧、セッション管理、管理画面）の前に整える。
- **Phase 1 を Phase 3 (MFA) より先**: 既存 port の完成（spec↔impl drift 解消）と OAuth/OIDC 規格適合性の修正は新 port 追加より低リスクで、RA のデモとしても「閉じてから増やす」を示せる。PKCE 必須化は OAuth 2.0 / OIDC 1.0 では仕様外であり、互換性のため client type に応じた階段化に戻す（RFC 9700 / OAuth 2.1 と整合）。
- **Phase 5 が分水嶺**: 管理 port と tenant 概念がここで入る。Phase 6–7 はそれを前提に乗る。tenant を Phase 7 まで遅らせると、admin / DCR / Federation を全部 retrofit することになる。
- **Phase 7 で SAML 2.0 IdP**: outbound（ra-idp が SAML IdP として SP に対応）は B2B SaaS では事実上必須。OIDC のみではエンタープライズ販売の最低ラインを満たさない。inbound と outbound を同じ Phase で扱うのは、両方が "プロトコル間の境界" として同じ port 構造を取るため。
- **Phase 9 の HSM/KMS**: Phase 1 で抽象 port を据えてあれば adapter 差し替えのみ。RA の「port を切っておけば後で差し替えるだけ」を最も鮮明に示す機会。

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
| POST     | /login                                  | パスワードログイン + セッション Cookie 発行       |
| POST     | /consent                                | コンセント UI のフォーム受領                      |
| GET/POST | /end_session                            | RP-Initiated Logout v1                           |
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
docker build -f infra/docker/Dockerfile -t ra-idp:dev .
kubectl apply -k infra/k8s/overlays/dev
kubectl apply -k infra/k8s/overlays/prod    # Argo Rollouts canary + SLO-aware ロールバック (ADR-021)
```

multi-stage build (Bun builder → distroless runtime) / non-root / multi-arch。
base manifests は Deployment + HPA + PodDisruptionBudget + NetworkPolicy +
ServiceMonitor + Rollout（prod のみ）。

### CI / CD (`.github/workflows/ra-idp-*.yaml`)

| ワークフロー   | トリガー            | 内容                                                                                |
| -------------- | ------------------- | ----------------------------------------------------------------------------------- |
| `ci.yaml`      | PR / push to main   | typecheck + 全テスト + 派生物 drift 検知 + Trivy + CodeQL                           |
| `nightly.yaml` | cron 03:00 JST      | k6 負荷試験 (SLO 違反で fail)                                                       |
| `release.yaml` | tag `ra-idp-v*` | multi-arch build → cosign 署名 → CycloneDX SBOM 添付 → SLSA Level 3 provenance 生成 |

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

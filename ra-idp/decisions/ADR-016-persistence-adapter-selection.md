# ADR-016: 永続化アダプタは Postgres (durable) + Redis (volatile) + Postgres outbox → Kafka (events) で構成する

## ステータス

採用

## コンテキスト

ADR-003 (Adapter Replaceability) は「永続化アダプタは積極的に差し替えられる」と宣言した。
しかし `adapters/persistence/in-memory-*.ts` しか実在しない状態では、
この宣言は **コード上の主張に留まり、実証されていない**。

Regenerative Architecture の検証ループ (REGENERATIVE_ARCHITECTURE.md §7) は
「再生成された外層が、仕様核から見て等価な振る舞いを示すか」を機械検証することを要求する。
本 ADR は、in-memory に加えて「もう一種類」のアダプタを並置することで、

1. **`src/oauth2/usecases/*` と `adapters/http/*` を 1 行も変更せずに永続化を入れ替えられる**
2. **同じ契約テスト群を 2 種類のアダプタで pass させる** (再生成等価性の実証)

を達成することを目的とする。

OAuth2 / OIDC IdP の状態は性質が大きく分かれる:

| 状態種別             | 例                                                          | 特性                            |
| -------------------- | ----------------------------------------------------------- | ------------------------------- |
| **Durable**          | Client / User / Consent / RefreshToken / AuditLog           | 長期保管・トランザクション必要  |
| **Volatile**         | AuthorizationRequest / AuthorizationCode / PAR / DPoPReplay | 短 TTL・高スループット・揮発で OK |
| **Event broadcast**  | DomainEvent (events.schema.json)                            | at-least-once 配送・SIEM 連携  |

それぞれに最適な技術が異なる。

## 決定

### 1. Durable state は PostgreSQL

理由:

- **ACID トランザクション**: 「認可コード redeem + refresh token 発行 + 監査イベント emit」
  を 1 つの不可分操作にしたい (RFC 9700 §4.10 のリプレイ検出の正確性に必須)
- **SELECT FOR UPDATE / advisory lock** によるリフレッシュトークンの原子的 rotate
- **JSONB** で OAuth クライアントメタデータ (RFC 7591) のような半構造化フィールドを扱える
- **論理レプリケーション** によるリードレプリカと CDC 対応
- **コンプライアンス** (PCI / SOC2) で監査ログの不変保管要件 (`slo.yaml audit_log_days = 2555`)
  に対し append-only テーブル + `pg_audit` で対応可能

### 2. Volatile state は Redis

理由:

- **TTL 自動失効** が `slo.yaml token_lifetimes.*_ttl_seconds` と素直に対応する
  (authorization_code_ttl_seconds=60, par_request_uri_ttl_seconds=600 等)
- **SETNX + Lua** で「認可コードの atomic redeem」「DPoP jti のリプレイ検出」が
  単一ラウンドトリップで完了 (introspect の `p99_latency_ms = 50` 要件に対する設計余裕)
- **クラスタモードで水平スケール** (`scalability.token_requests_per_second_max = 5000` を満たす)
- 揮発で構わない: TTL を過ぎた認可コードは仕様上「invalid_grant」として
  再認証フローへ誘導されるべきもの

### 3. Domain Events は Postgres outbox → Kafka 二段配送

理由:

- **dual-write 問題の回避**: ユースケース内で「DB INSERT + Kafka publish」を直接行うと、
  片方成功・片方失敗のスプリットブレイン状態を許容してしまう。
  Postgres トランザクション内で outbox テーブルにイベントを INSERT し、
  別プロセス (event-relay) がそれを tail して Kafka に publish することで
  **at-least-once 配送と原子的整合性を両立**できる
- **再生 (replay) 可能**: outbox テーブルは consumer offset を持つので、
  Kafka が落ちても再起動後に再配送できる (RA §3.1「データの連続性」)
- **SIEM 連携の差し替え容易性**: consumer 側 (Kafka topic) は Splunk / Datadog / 自社 SIEM に
  独立して接続できる。outbox スキーマは AsyncAPI で固定される (`spec/event-stream.asyncapi.yaml`)

開発・テスト時は `console` シンク (現状の console.log と等価) も選択可能とする。

## 構成と選択切替

`main.ts` の合成ルートに環境変数スイッチを置く:

```text
PERSISTENCE = memory | postgres        (default: memory)
EVENT_SINK  = console | outbox | kafka (default: console)
```

`PERSISTENCE=postgres` のとき、durable は Postgres、volatile は Redis をセットで使う。
これは「OAuth2 IdP の本番想定構成」が暗黙にこの組み合わせを前提とするため。

## データライフサイクル制約の実装写像

`spec/slo.yaml` の値は、次のように DB 設定へ転写される。
これは `infra/scripts/check-spec-coherence.ts` で機械検証される (drift 検知)。

| spec/slo.yaml                          | 実装上の写像                                        |
| -------------------------------------- | --------------------------------------------------- |
| `audit_log_days = 2555`                | `audit_log` テーブルの partition retention 7 年      |
| `pii_purge_after_deletion_days = 30`   | `users.deleted_at` から 30 日後の物理削除バッチ     |
| `consent_records_days = 2555`          | `consents` テーブルの保管 7 年                       |
| `signing_key_archive_days = 2555`      | `signing_keys` テーブルの archive 列、7 年保管       |
| `authorization_code_ttl_seconds = 60`  | Redis `EX 60`                                       |
| `par_request_uri_ttl_seconds = 600`    | Redis `EX 600`                                      |
| `dpop_jti_replay_window_minutes = 10`  | Redis `EX 600`                                      |
| `refresh_token_ttl_seconds`            | `refresh_tokens.expires_at` 列、cron で物理削除      |
| `refresh_token_absolute_ttl_seconds`   | `refresh_tokens.absolute_expires_at` 列            |

スキーマ進化は加法的変更を原則とする (`spec/migrations/README.md` を参照)。

## 却下した代替案

- **DynamoDB / ScyllaDB**: 単一テーブルでの水平スケールは優秀だが、
  「認可コード redeem + refresh token 発行 + audit emit」を 1 トランザクションで
  原子的に行えない。条件付き書き込みを多段で組む実装は複雑かつバグの温床。
  本アプリの規模 (RPS 5000) では Postgres で十分。

- **CockroachDB / Spanner**: 分散 SQL は魅力的だが、レイテンシ要件 (`p99 = 300ms` for /token)
  に対しコミットレイテンシの分布が広い。グローバル分散が必須になる規模 (>10k RPS) で再検討。

- **KeyDB / Dragonfly (Redis 互換)**: 性能上の優位はあるが、エコシステム成熟度と
  クライアントライブラリの安定性で Redis OSS が現時点では妥当。
  Redis 互換 API を維持する限り、差し替えは ADR-016a で容易に行える。

- **NATS / Pulsar (Kafka 代替)**: Kafka を選んだのは
  「at-least-once + ordered + 長期保管」の三拍子が標準で揃うため。
  NATS JetStream は ordered の保証が partition より弱く、ordering を要求する
  RefreshTokenRotated イベントの consumer に追加実装を強いる。

- **Postgres LISTEN/NOTIFY を outbox 配送に使う**: 単一 DB クラスタを超えた
  consumer を持つと拡張性が破綻する。outbox + Kafka なら、後で Datadog 直接連携や
  別 Bounded Context への放出も追加できる。

- **ORM (Drizzle / Prisma) を使う**: 「永続化アダプタが差し替え可能」であることを
  示す参照実装において、ORM 抽象が SQL の表現力を制限すると、
  RFC 9700 で要求される `SELECT FOR UPDATE` の正確な制御が阻害される。
  本アプリは `pg` (node-postgres) を直接使い、SQL を可視化する。

## 影響

- 新規依存: `pg`, `ioredis`, `kafkajs`
- 新規ポート: `src/shared/ports/event-sink.ts` `src/shared/ports/transaction.ts`
- 新規仕様: `spec/migrations/0001_init.sql`, `spec/event-stream.asyncapi.yaml`
- 新規 ADR: 本 ADR (ADR-016)
- 新規アダプタ: `adapters/persistence/postgres/*` `adapters/persistence/redis/*` `adapters/event-sink/*`
- 既存 `adapters/persistence/in-memory-*.ts` は `adapters/persistence/memory/*.ts` に移設し、
  ルートから re-export して既存 import を保つ
- `src/oauth2/usecases/*` と `adapters/http/*` と `spec/*` は **完全に無変更**
- 契約テスト (`src/spec-bindings/persistence-contract.test.ts`) で 2 種類のアダプタを
  同じ仕様で検証することで ADR-003 の「Adapter 差し替え可能」を実コード化する

## 関連

- ADR-003 (Adapter Replaceability) — 本 ADR がこれを実コードで証明する
- ADR-004 (Refresh Token Rotation) — atomic rotate の実装要求の源泉
- ADR-009 (Key Rotation) — `signing_keys` テーブルが派生
- `spec/slo.yaml` — DB 設定値の唯一の権威

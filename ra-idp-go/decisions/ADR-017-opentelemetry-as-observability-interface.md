# ADR-017: 観測性インターフェースとして OpenTelemetry を採用する

## ステータス

採用

## コンテキスト

`spec/slo.yaml` には p99 レイテンシ・エラー率・スループット上限が定義されている。
これらが**運用上の合否判定基準**として機能するためには、

1. 実装が指定されたメトリクスを出力する
2. アラート / ダッシュボード / 負荷試験が同じメトリクスを参照する
3. ベンダ (Jaeger, Tempo, Datadog, NewRelic) の差し替えが ADR 1 本で完結する

の 3 つを満たさなければならない。

ベンダロックインを避けるため、観測データの「生成・伝搬・出力」を標準化する
**OpenTelemetry** (OTel) を採用する。OTel は trace / metric / log の 3 シグナルを
カバーする CNCF Graduated プロジェクトで、各言語 SDK と OTLP プロトコルが安定している。

## 決定

### 1. 観測 IF は OpenTelemetry API

アプリケーションコード (`src/oauth2/usecases/*`) は `@opentelemetry/api` の
`Tracer` `Meter` `Logger` のみに依存する。

ベンダ側 (Jaeger / Datadog / etc) は Runtime 層の OTel Collector が引き受ける。
Collector の設定変更で Vendor を切り替えられる。

### 2. trace / metric の命名は spec/scl.yaml が権威

メトリクス名 / span 名 / log fields は `spec/scl.yaml` の `objectives.observability` で定義され、
コードはそこから派生した literal union 型を使う。

これにより:

- Prometheus 録ルールの metric 名と実装の metric 名がずれない (CI で検証)
- `spec/slo.yaml` の `performance.endpoints.token.p99_latency_ms = 300` が
  Prometheus アラートと Grafana ダッシュボードと k6 閾値の **3 箇所すべて** に
  自動同期される

### 3. 自動計装は使うが、ユースケース境界は手動 span を切る

- HTTP / Postgres / Redis は自動計装 (`@opentelemetry/instrumentation-*`)
- `src/oauth2/usecases/*` の各関数は手動 span を切る (ビジネスドメイン語彙で trace が読める)

span 名は `spec/observability.yaml` 由来:

```text
oauth2.authorize
oauth2.par.consume
oauth2.token.exchange
oauth2.token.refresh
oauth2.introspect
oauth2.userinfo
oauth2.revoke
```

### 4. メトリクス命名規則

OpenMetrics / Prometheus に沿う:

```text
oauth2_<endpoint>_<measurement>_<unit>
  oauth2_token_requests_total                  counter (labels: grant_type, client_id, result)
  oauth2_token_request_duration_seconds        histogram
  oauth2_authorization_codes_redeemed_total    counter
  oauth2_refresh_tokens_rotated_total          counter
  oauth2_refresh_token_reuse_detected_total    counter
  oauth2_signing_key_rotations_total           counter
```

ヒストグラムバケットは `spec/slo.yaml` の p99 を含むよう
`[5, 10, 25, 50, 100, 200, 300, 500, 1000]` ms を採用。

### 5. context propagation

W3C Trace Context (`traceparent` / `tracestate`) を採用。
OAuth クライアントが当該ヘッダを送ってきた場合、サーバーは継続する。

## 却下した代替案

- **Prometheus client 直接**: メトリクスは取れるが trace と log を統一できず、
  trace_id → metric → log の相互ジャンプができない。
- **vendor (Datadog/NewRelic) SDK 直接**: ロックイン。アプリ層が vendor 依存になる。
- **手書きの観測コード**: trace 伝搬と context のスコープ管理は車輪の再発明。
- **OpenTracing**: deprecated。OTel に統合済み。

## 影響

- 新規依存:
  `@opentelemetry/api` `@opentelemetry/sdk-node` `@opentelemetry/exporter-trace-otlp-http`
  `@opentelemetry/exporter-metrics-otlp-http`
  `@opentelemetry/instrumentation-http` `@opentelemetry/instrumentation-pg`
  `@opentelemetry/instrumentation-redis-4`
- 新規ポート: `src/shared/ports/observer.ts`
- 新規アダプタ: `adapters/observability/otel.ts`, `adapters/observability/noop.ts`
- 新規ミドルウェア: `adapters/http/middleware/trace-middleware.ts`, `metrics-middleware.ts`
- 新規仕様: `spec/observability.yaml` (メトリクス/span/log カタログ)
- 派生成果物: `infra/observability/prometheus/*`, `infra/observability/grafana/*`
- 環境変数: `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_SERVICE_NAME`, `OBSERVABILITY=otel|noop`

## 関連

- ADR-018 (監査ログ vs アプリログの分離)
- `spec/slo.yaml` — メトリクスの閾値の権威
- `spec/observability.yaml` — メトリクス/span/log の名前空間の権威

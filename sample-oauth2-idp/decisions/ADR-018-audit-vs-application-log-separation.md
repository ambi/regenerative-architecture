# ADR-018: 監査ログとアプリケーションログを分離する

## ステータス

採用

## コンテキスト

OAuth2 / OIDC IdP には性質の異なる 2 種類のログがある:

1. **監査ログ (audit log)**
   - `spec/events.schema.json` の DomainEvent
   - **不変** (UPDATE / DELETE 禁止、append-only)
   - **長期保管** (`spec/slo.yaml audit_log_days = 2555` = 7 年)
   - **法的証拠能力** (SOC2 / PCI / FAPI 監査)
   - SIEM (Splunk / Datadog SIEM / Sentinel) の主要 source

2. **アプリケーションログ (application log)**
   - debug / info / warn / error
   - **可変** (PII マスキング・改行整形等の加工が許容される)
   - **短期保管** (30 日程度)
   - インシデント時の調査に使う

これらを同じパスで扱うと、

- 監査ログに debug 文字列が混入して肥大化
- アプリログに PII が漏れて compliance 違反
- リテンション要件が衝突 (7 年 vs 30 日)

という問題が生じる。

## 決定

### 1. 出力先を物理的に分離

- 監査ログ:
  - **同期的に** `Postgres outbox` テーブルに書く (ADR-016)
  - その後 Kafka topic に publish され、SIEM が consume
  - フォーマットは `spec/events.schema.json` (JSON Schema 強制)
- アプリログ:
  - 非同期に **stdout JSON Lines** に書く
  - OTel Collector の filelog receiver / fluentbit が拾い、Loki / OpenSearch へ
  - フォーマットは構造化ログ規約 (下記)

### 2. ポート分離

`Logger` ポートは:

```text
Logger.audit(event: DomainEvent)    // → EventSink ポート経由で同期配送
Logger.info|warn|error(msg, attrs)   // → OTel logger 経由で非同期配送
```

`audit` は背後で `EventSink.publish` を呼ぶ。
これによりユースケース層から見れば「ログ 1 つの API」だが、出力先は分離される。

### 3. 構造化ログのフィールド規約

すべてのアプリログは以下のフィールドを必ず持つ:

```text
timestamp        ISO 8601
level            debug | info | warn | error
service          oauth2-idp
trace_id         OTel trace id (なければ空文字)
span_id          OTel span id
event_type       (audit のみ。DomainEvent.type)
client_id        (該当時)
sub              (該当時)
message          人間可読
```

OTel logs SDK が trace_id / span_id を自動注入する。

### 4. PII マスキング

`spec/user.schema.json` の `x-pii: true` フィールド (`email` 等) が
**アプリログ** に出力されることを禁止する。
監査ログ側は法的に必要なため、PII を含むことが許される代わりに
KMS 暗号化 + 厳格なアクセス制御で保護する (Phase 3 で本格化)。

CI で `infra/scripts/check-no-pii-in-logs.ts` が grep ベースの簡易検査を行う。

### 5. リテンション

| ログ種別             | 保管先         | 期間  | 根拠                                 |
| -------------------- | -------------- | ----- | ------------------------------------ |
| 監査ログ             | Postgres + S3  | 7 年  | `spec/slo.yaml audit_log_days = 2555` |
| 監査用署名鍵情報     | Postgres       | 7 年  | `signing_key_archive_days = 2555`     |
| アプリログ           | Loki / OS      | 30 日 | 運用上の調査要件                     |
| メトリクス時系列     | Prometheus     | 30 日 | SLO 計測ウィンドウ                   |
| trace 詳細           | Tempo / Jaeger | 7 日  | コスト最適化                         |

## 却下した代替案

- **単一ログストリームに mix**: 上記の理由 (リテンション衝突、PII 漏洩)
- **監査ログを stdout に**: コンテナログ消失リスク。compliance 不可
- **アプリログを Postgres に**: 書き込み量で IdP の token endpoint レイテンシを潰す

## 影響

- 新規ポート: `Logger` (`audit` / `info` / `warn` / `error`)
- 新規アダプタ: `adapters/observability/otel-logger.ts`
- 既存の `EventSink` ポート (ADR-016) と統合
- `infra/scripts/check-no-pii-in-logs.ts` を CI に追加 (Phase 3)
- `spec/observability.yaml` に `log_fields` カタログを追加

## 関連

- ADR-016 (Postgres outbox)
- ADR-017 (OpenTelemetry)
- `spec/events.schema.json`
- `spec/slo.yaml data_retention.*`

/**
 * Layer 3 — Application Logic (ポート定義)
 *
 * Observer ポート: OpenTelemetry の trace / metric / log を抽象化。
 *
 * 実装側:
 *   - OtelObserver  (adapters/observability/otel.ts)   本番
 *   - NoopObserver  (adapters/observability/noop.ts)   テスト・無効化用
 *
 * span 名 / metric 名 / log fields は `spec/scl.yaml` の interfaces.*.{spans,metrics,log_fields} を権威とする。
 * 名前のずれは src/spec-bindings/invariants.test.ts で検証される。
 *
 * 設計方針:
 *   - context 伝搬は実装側の責務 (OtelObserver は AsyncLocalStorage を使う)
 *   - エラー時にユースケースを失敗させない (noop fallback)
 *   - シャットダウン時のフラッシュは Runtime 層 (main.ts) が tracer.shutdown を呼ぶ
 */

import type { DomainEvent } from '../../spec-bindings/schemas'

// ---------------------------------------------------------------
// Tracer
// ---------------------------------------------------------------

export interface Span {
  setAttribute(key: string, value: string | number | boolean): void
  setStatus(status: { code: 'ok' | 'error'; message?: string }): void
  recordException(error: Error): void
  end(): void
}

export interface Tracer {
  /** span を作成して fn 実行中アクティブにする。 */
  startActiveSpan<T>(
    name: string,
    fn: (span: Span) => Promise<T>,
    attrs?: Record<string, string | number | boolean>,
  ): Promise<T>

  /** 手動 span 管理用。fn を使わない場合。 */
  startSpan(name: string, attrs?: Record<string, string | number | boolean>): Span
}

// ---------------------------------------------------------------
// Meter
// ---------------------------------------------------------------

export interface Counter {
  add(value: number, labels?: Record<string, string>): void
}

export interface Histogram {
  record(value: number, labels?: Record<string, string>): void
}

export interface Meter {
  counter(name: string, opts?: { description?: string; unit?: string }): Counter
  histogram(name: string, opts?: { description?: string; unit?: string }): Histogram
}

// ---------------------------------------------------------------
// Logger (ADR-018 audit vs application 分離)
// ---------------------------------------------------------------

export interface Logger {
  /** 監査イベント (events.schema.json 準拠)。EventSink 経由で同期配送。 */
  audit(event: DomainEvent): Promise<void>

  info(message: string, attrs?: Record<string, unknown>): void
  warn(message: string, attrs?: Record<string, unknown>): void
  error(message: string, attrs?: Record<string, unknown>): void
  debug?(message: string, attrs?: Record<string, unknown>): void
}

// ---------------------------------------------------------------
// 統合的アクセス点
// ---------------------------------------------------------------

export interface Observer {
  tracer: Tracer
  meter: Meter
  logger: Logger
  /** プロセス終了時にバッチを flush。Runtime 層 main.ts が呼ぶ。 */
  shutdown(): Promise<void>
}

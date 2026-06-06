/**
 * Layer 4 — Adapter Layer (OpenTelemetry Observer)
 *
 * ADR-017 が選定した OpenTelemetry の実装。
 * trace は OTLP/HTTP exporter で OTel Collector に送出。
 * metric は OTLP/HTTP exporter で送出 (Collector が Prometheus remote_write に変換)。
 * application log は OTel logs SDK + stdout JSON Lines (両方)。
 * audit log は EventSink に委譲 (ADR-018)。
 *
 * 環境変数:
 *   OTEL_EXPORTER_OTLP_ENDPOINT  e.g. http://otel-collector:4318
 *   OTEL_SERVICE_NAME            default: SCL 由来の observability ビューの service.name
 *
 * Note: OpenTelemetry のフル機能を使うには Bun が Node 互換の AsyncLocalStorage を
 * 提供している必要がある (Bun 1.0+ で利用可)。
 */

import type {
  Observer,
  Tracer,
  Meter,
  Logger,
  Span,
  Counter,
  Histogram,
} from '../../src/shared/ports/observer'
import type { EventSink } from '../../src/shared/ports/event-sink'
import type { DomainEvent } from '../../src/spec-bindings/schemas'

// OTel API を dynamic import で抽象化 (optionalDependencies)
type OtelApi = any
type OtelSdk = any

export interface OtelObserverConfig {
  serviceName: string
  serviceVersion?: string
  serviceNamespace?: string
  otlpEndpoint?: string // default: http://localhost:4318
  eventSink: EventSink
}

class OtelSpanWrapper implements Span {
  constructor(private readonly inner: any) {}
  setAttribute(key: string, value: string | number | boolean): void {
    this.inner.setAttribute(key, value)
  }
  setStatus(status: { code: 'ok' | 'error'; message?: string }): void {
    // SpanStatusCode.OK = 1, SpanStatusCode.ERROR = 2
    this.inner.setStatus({ code: status.code === 'ok' ? 1 : 2, message: status.message })
  }
  recordException(error: Error): void {
    this.inner.recordException(error)
  }
  end(): void {
    this.inner.end()
  }
}

class OtelTracerWrapper implements Tracer {
  constructor(private readonly tracer: any) {}

  async startActiveSpan<T>(
    name: string,
    fn: (span: Span) => Promise<T>,
    attrs?: Record<string, string | number | boolean>,
  ): Promise<T> {
    return await this.tracer.startActiveSpan(
      name,
      { attributes: attrs },
      async (innerSpan: any) => {
        const wrapped = new OtelSpanWrapper(innerSpan)
        try {
          const result = await fn(wrapped)
          wrapped.setStatus({ code: 'ok' })
          return result
        } catch (err) {
          wrapped.recordException(err as Error)
          wrapped.setStatus({ code: 'error', message: (err as Error).message })
          throw err
        } finally {
          wrapped.end()
        }
      },
    )
  }

  startSpan(name: string, attrs?: Record<string, string | number | boolean>): Span {
    return new OtelSpanWrapper(this.tracer.startSpan(name, { attributes: attrs }))
  }
}

class OtelCounter implements Counter {
  constructor(private readonly counter: any) {}
  add(value: number, labels?: Record<string, string>): void {
    this.counter.add(value, labels)
  }
}

class OtelHistogram implements Histogram {
  constructor(private readonly histogram: any) {}
  record(value: number, labels?: Record<string, string>): void {
    this.histogram.record(value, labels)
  }
}

class OtelMeterWrapper implements Meter {
  constructor(private readonly meter: any) {}
  counter(name: string, opts?: { description?: string; unit?: string }): Counter {
    return new OtelCounter(this.meter.createCounter(name, opts))
  }
  histogram(name: string, opts?: { description?: string; unit?: string }): Histogram {
    return new OtelHistogram(this.meter.createHistogram(name, opts))
  }
}

class OtelLogger implements Logger {
  constructor(
    private readonly eventSink: EventSink,
    private readonly otelApi: OtelApi,
    private readonly forbidden: Set<string>,
  ) {}

  async audit(event: DomainEvent): Promise<void> {
    // 監査ログは EventSink に委譲 (ADR-018)。
    // 同時に traceId を付加した structured info も出す (運用観測用)。
    await this.eventSink.publish(event)
    this.emit('info', `audit:${event.type}`, { event_type: event.type })
  }

  info(message: string, attrs?: Record<string, unknown>): void {
    this.emit('info', message, attrs)
  }
  warn(message: string, attrs?: Record<string, unknown>): void {
    this.emit('warn', message, attrs)
  }
  error(message: string, attrs?: Record<string, unknown>): void {
    this.emit('error', message, attrs)
  }
  debug(message: string, attrs?: Record<string, unknown>): void {
    this.emit('debug', message, attrs)
  }

  private emit(level: string, message: string, attrs?: Record<string, unknown>): void {
    // PII マスキング (ADR-018): forbidden fields を redact
    const safe = attrs ? this.redact(attrs) : undefined
    // 現在のスパンから trace_id / span_id を取得
    const span = this.otelApi.trace.getActiveSpan()
    const ctx = span?.spanContext()
    const record = {
      timestamp: new Date().toISOString(),
      level,
      message,
      ...safe,
      trace_id: ctx?.traceId ?? '',
      span_id: ctx?.spanId ?? '',
    }
    // stdout JSON Lines (本番では OTel Collector の filelog receiver が拾う)
    // eslint-disable-next-line no-console
    console.log(JSON.stringify(record))
  }

  private redact(attrs: Record<string, unknown>): Record<string, unknown> {
    const out: Record<string, unknown> = {}
    for (const [k, v] of Object.entries(attrs)) {
      if (this.forbidden.has(k)) {
        out[k] = '[REDACTED]'
      } else {
        out[k] = v
      }
    }
    return out
  }
}

export class OtelObserver implements Observer {
  tracer!: Tracer
  meter!: Meter
  logger!: Logger
  private sdk: OtelSdk | null = null

  static async create(config: OtelObserverConfig): Promise<OtelObserver> {
    const obs = new OtelObserver()
    await obs.init(config)
    return obs
  }

  private async init(config: OtelObserverConfig): Promise<void> {
    // 動的読込: optionalDependencies のため未 install 環境でも型エラーを出さない。
    // 引数を間接化して TS の module resolve を回避する。
    const dynImport = (m: string): Promise<any> => import(/* @vite-ignore */ m as any)
    const otelApi = await dynImport('@opentelemetry/api')
    const sdkNode = await dynImport('@opentelemetry/sdk-node')
    const resources = await dynImport('@opentelemetry/resources')
    const semConv = await dynImport('@opentelemetry/semantic-conventions')
    const otlpTrace = await dynImport('@opentelemetry/exporter-trace-otlp-http')
    const otlpMetric = await dynImport('@opentelemetry/exporter-metrics-otlp-http')
    const sdkMetrics = await dynImport('@opentelemetry/sdk-metrics')

    const endpoint =
      config.otlpEndpoint ?? process.env.OTEL_EXPORTER_OTLP_ENDPOINT ?? 'http://localhost:4318'

    const resource = new resources.Resource({
      [semConv.SemanticResourceAttributes?.SERVICE_NAME ?? 'service.name']: config.serviceName,
      [semConv.SemanticResourceAttributes?.SERVICE_VERSION ?? 'service.version']:
        config.serviceVersion ?? '0.2.0',
      [semConv.SemanticResourceAttributes?.SERVICE_NAMESPACE ?? 'service.namespace']:
        config.serviceNamespace ?? 'identity',
    })

    const sdk = new sdkNode.NodeSDK({
      resource,
      traceExporter: new otlpTrace.OTLPTraceExporter({
        url: `${endpoint}/v1/traces`,
      }),
      metricReader: new sdkMetrics.PeriodicExportingMetricReader({
        exporter: new otlpMetric.OTLPMetricExporter({
          url: `${endpoint}/v1/metrics`,
        }),
        exportIntervalMillis: 10000,
      }),
    })
    sdk.start()
    this.sdk = sdk

    const observabilitySpec = await loadObservabilitySpec()
    const forbidden = new Set<string>(
      (observabilitySpec.logs?.application?.forbidden_fields ?? []) as string[],
    )

    const tracer = otelApi.trace.getTracer(config.serviceName, config.serviceVersion)
    const meter = otelApi.metrics.getMeter(config.serviceName, config.serviceVersion)
    this.tracer = new OtelTracerWrapper(tracer)
    this.meter = new OtelMeterWrapper(meter)
    this.logger = new OtelLogger(config.eventSink, otelApi, forbidden)
  }

  async shutdown(): Promise<void> {
    if (this.sdk) {
      await this.sdk.shutdown()
      this.sdk = null
    }
  }
}

async function loadObservabilitySpec(): Promise<any> {
  const mod = await import('../../infra/scripts/load-specs')
  return mod.loadObservability()
}

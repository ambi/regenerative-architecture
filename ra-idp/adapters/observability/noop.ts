/**
 * Layer 4 — Adapter Layer (Noop Observer)
 *
 * テスト・無効化用。すべての span / metric を捨てる。
 * audit ログだけは EventSink に委譲する (これは ADR-018 のセキュリティ要件)。
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

const noopSpan: Span = {
  setAttribute: () => undefined,
  setStatus: () => undefined,
  recordException: () => undefined,
  end: () => undefined,
}

class NoopTracer implements Tracer {
  async startActiveSpan<T>(_name: string, fn: (span: Span) => Promise<T>): Promise<T> {
    return fn(noopSpan)
  }
  startSpan(): Span {
    return noopSpan
  }
}

class NoopCounter implements Counter {
  add(): void {
    /* noop */
  }
}
class NoopHistogram implements Histogram {
  record(): void {
    /* noop */
  }
}

class NoopMeter implements Meter {
  counter(): Counter {
    return new NoopCounter()
  }
  histogram(): Histogram {
    return new NoopHistogram()
  }
}

class NoopLogger implements Logger {
  constructor(private readonly eventSink: EventSink) {}
  async audit(event: DomainEvent): Promise<void> {
    await this.eventSink.publish(event)
  }
  info(): void {
    /* noop */
  }
  warn(): void {
    /* noop */
  }
  error(): void {
    /* noop */
  }
  debug(): void {
    /* noop */
  }
}

export class NoopObserver implements Observer {
  readonly tracer = new NoopTracer()
  readonly meter = new NoopMeter()
  readonly logger: Logger
  constructor(eventSink: EventSink) {
    this.logger = new NoopLogger(eventSink)
  }
  async shutdown(): Promise<void> {
    /* noop */
  }
}

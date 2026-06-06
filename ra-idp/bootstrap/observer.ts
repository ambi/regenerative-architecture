/**
 * Layer 5 — Runtime: Observability (ADR-017) の組み立て。
 *
 * OBSERVABILITY=otel のときだけ OpenTelemetry を動的 import する。
 */

import { NoopObserver } from '../adapters/observability/noop'
import type { EventSink } from '../src/shared/ports/event-sink'
import type { Observer } from '../src/shared/ports/observer'
import type { RuntimeConfig } from './config'

export async function assembleObserver(
  config: RuntimeConfig,
  eventSink: EventSink,
): Promise<Observer> {
  if (config.observabilityMode === 'otel') {
    const { OtelObserver } = await import('../adapters/observability/otel')
    return await OtelObserver.create({
      serviceName: process.env.OTEL_SERVICE_NAME ?? 'ra-idp',
      eventSink,
    })
  }
  return new NoopObserver(eventSink)
}

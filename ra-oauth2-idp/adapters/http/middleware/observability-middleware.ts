/**
 * Layer 4 — Adapter Layer (HTTP observability middleware)
 *
 * 各リクエストを span 化 + メトリクス記録。
 * - span 名は HTTP メソッド + ルートパス
 * - メトリクス名は spec/scl.yaml interfaces.*.metrics の `oauth2_<endpoint>_*` を使用
 *
 * ルートパス → エンドポイント名のマップは spec/scl.yaml interfaces.* と一致させる。
 */

import type { Context, Next } from 'hono'
import type { Observer } from '../../../src/ports/observer'

// パス → spec/scl.yaml interfaces.* の metric 名のプレフィックス
const PATH_TO_METRIC: Array<{ pattern: RegExp; endpoint: string }> = [
  { pattern: /^\/authorize/, endpoint: 'authorize' },
  { pattern: /^\/par/, endpoint: 'par' },
  { pattern: /^\/token/, endpoint: 'token' },
  { pattern: /^\/introspect/, endpoint: 'introspect' },
  { pattern: /^\/revoke/, endpoint: 'revoke' },
  { pattern: /^\/userinfo/, endpoint: 'userinfo' },
  { pattern: /^\/jwks/, endpoint: 'jwks' },
  { pattern: /^\/\.well-known\/openid-configuration/, endpoint: 'discovery' },
  { pattern: /^\/\.well-known\/oauth-authorization-server/, endpoint: 'discovery' },
]

function resolveEndpoint(path: string): string | null {
  for (const m of PATH_TO_METRIC) {
    if (m.pattern.test(path)) return m.endpoint
  }
  return null
}

export function createObservabilityMiddleware(observer: Observer) {
  // メトリクスを観測対象パスごとに準備
  const counters: Record<string, ReturnType<Observer['meter']['counter']>> = {}
  const histograms: Record<string, ReturnType<Observer['meter']['histogram']>> = {}

  function getCounter(endpoint: string): ReturnType<Observer['meter']['counter']> {
    const name = `oauth2_${endpoint}_requests_total`
    if (!counters[name]) {
      counters[name] = observer.meter.counter(name, {
        description: `${endpoint} エンドポイントのリクエスト数`,
      })
    }
    return counters[name]
  }
  function getHistogram(endpoint: string): ReturnType<Observer['meter']['histogram']> {
    const name = `oauth2_${endpoint}_request_duration_seconds`
    if (!histograms[name]) {
      histograms[name] = observer.meter.histogram(name, {
        description: `${endpoint} エンドポイントの所要時間 (秒)`,
        unit: 's',
      })
    }
    return histograms[name]
  }

  return async function observabilityMiddleware(c: Context, next: Next) {
    const path = c.req.path
    const method = c.req.method
    const endpoint = resolveEndpoint(path)
    const start = performance.now()

    if (!endpoint) {
      // observability 対象外のパス (/, /health, /events) は素通し
      await next()
      return
    }

    await observer.tracer.startActiveSpan(`http.${method} /${endpoint}`, async (span) => {
      span.setAttribute('http.method', method)
      span.setAttribute('http.target', path)
      try {
        await next()
        const status = c.res.status
        span.setAttribute('http.status_code', status)
        const elapsedS = (performance.now() - start) / 1000
        getHistogram(endpoint).record(elapsedS, { status: String(status) })
        getCounter(endpoint).add(1, {
          result: status < 400 ? 'success' : 'error',
          status: String(status),
        })
      } catch (err) {
        span.recordException(err as Error)
        getCounter(endpoint).add(1, { result: 'error', status: '500' })
        throw err
      }
    })
  }
}

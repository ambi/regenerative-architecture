/**
 * SCL-derived views for generators (gen-prometheus-rules / gen-grafana-dashboard / gen-k6-thresholds)
 *
 * spec/scl.yaml から SLO / Observability ビューを派生させる共通ヘルパー。
 *
 * Bun の組み込み YAML loader を使うため、本ファイルは Bun ランタイム前提。
 */

import sclDoc from '../../spec/scl.yaml'

type SclLike = {
  vocabulary: Record<string, { aliases?: string[] }>
  models: Record<
    string,
    { kind: string; payload?: Record<string, { type: string; annotations?: { pii?: boolean } }> }
  >
  interfaces: Record<
    string,
    {
      description?: string
      bindings?: Array<{ kind: string; method?: string; path?: string }>
      emits?: string[]
    }
  >
  objectives: Record<string, ObjectiveRaw>
}

type ObjectiveBaseRaw = {
  description?: string
  reference?: string
}

type RetentionPolicy =
  | 'keep_indefinitely'
  | 'keep'
  | 'append_only'
  | 'delete_after'
  | 'purge_pii_after'
  | 'archive_after'
  | 'archive'

type SecurityPolicy =
  | 'max_age'
  | 'min_jwks_overlap'
  | 'rate_limit_per_minute'
  | 'clock_skew_seconds'
  | 'replay_window_minutes'
  | 'alert_window_seconds'

type ObjectivePolicy = RetentionPolicy | SecurityPolicy

type SloMetric =
  | 'latency_p50'
  | 'latency_p95'
  | 'latency_p99'
  | 'availability'
  | 'error_rate'
  | 'throughput'

type SloObjectiveRaw = ObjectiveBaseRaw & {
  kind: 'slo'
  metric: SloMetric
  target: string
  interface?: string
  window?: string
}

type RetentionObjectiveRaw = ObjectiveBaseRaw & {
  kind: 'retention'
  target: string
  policy: RetentionPolicy
  retention?: string
}

type LifetimeObjectiveRaw = ObjectiveBaseRaw & {
  kind: 'lifetime'
  target: string
  ttl: string
}

type SecurityObjectiveRaw = ObjectiveBaseRaw & {
  kind: 'security'
  policy: SecurityPolicy
  value: number | string
  target?: string
}

type ObjectiveRaw =
  | SloObjectiveRaw
  | RetentionObjectiveRaw
  | LifetimeObjectiveRaw
  | SecurityObjectiveRaw

const scl = sclDoc as unknown as SclLike

function toWire(name: string): string {
  const entry = scl.vocabulary[name]
  if (entry?.aliases) {
    for (const a of entry.aliases) if (/^[a-z][a-z0-9_:.-]*$/.test(a)) return a
  }
  return name
}

// ===============================================================
// SLO view（scl.objectives + scl.interfaces を SLO 構造に射影）
// ===============================================================

export interface SloEndpoint {
  method: string
  path: string
  p99_latency_ms: number
  error_rate_max: number
}

export interface SloSpec {
  performance: { endpoints: Record<string, SloEndpoint> }
  availability: Record<string, { target: number; measurement_window: string }>
  scalability: Record<string, number>
  token_lifetimes: Record<string, number>
  security: Record<string, number>
  data_retention: Record<string, number>
}

const DURATION_TO_SECONDS: Record<string, number> = {
  s: 1,
  m: 60,
  h: 3600,
  d: 86400,
  y: 31_536_000,
}

function parseDurationSeconds(d: string): number {
  const m = d.match(/^(\d+)\s*([smhdy])$/)
  if (!m) throw new Error(`bad duration: ${d}`)
  return Number(m[1]) * DURATION_TO_SECONDS[m[2]]
}

function parseDurationMs(d: string): number {
  const m = d.match(/^(\d+)(ms|s|m|h|d)$/)
  if (!m) throw new Error(`bad duration: ${d}`)
  const v = Number(m[1])
  switch (m[2]) {
    case 'ms':
      return v
    case 's':
      return v * 1000
    case 'm':
      return v * 60_000
    case 'h':
      return v * 3_600_000
    case 'd':
      return v * 86_400_000
    default:
      return v
  }
}

function parseDurationDays(d: string): number {
  if (d === 'keep_indefinitely') return Number.POSITIVE_INFINITY
  const sec = parseDurationSeconds(d)
  return Math.round(sec / 86400)
}

function parseTargetMs(target: string): number {
  const m = target.match(/^[<>]=?\s*(\d+)\s*(ms|s)$/)
  if (!m) throw new Error(`bad latency target: ${target}`)
  return Number(m[1]) * (m[2] === 's' ? 1000 : 1)
}

function parseTargetRate(target: string): number {
  const m = target.match(/^[<>]=?\s*(0?\.\d+)$/)
  if (!m) throw new Error(`bad rate target: ${target}`)
  return Number(m[1])
}

function parseTargetPercent(target: string): number {
  const m = target.match(/^[<>]=?\s*(\d+(?:\.\d+)?)%$/)
  if (!m) throw new Error(`bad percent target: ${target}`)
  return Number(m[1]) / 100
}

function parseTargetThroughput(target: string): number {
  const m = target.match(/^[<>]=?\s*(\d+)\s*rps$/)
  if (!m) throw new Error(`bad throughput target: ${target}`)
  return Number(m[1])
}

/**
 * SCL `interfaces` から「endpoint コード」（slo.performance.endpoints のキー）を割り当てる。
 * 規約: interface 名から末尾の動詞を消し、HTTP path の最後のセグメント
 * （または PAR/PushAuthorizationRequest 等の特殊エイリアス）を返す。
 */
const INTERFACE_TO_ENDPOINT: Record<string, string> = {
  Authorize: 'authorize',
  PushAuthorizationRequest: 'par',
  Token: 'token',
  Introspect: 'introspect',
  Revoke: 'revoke',
  UserInfo: 'userinfo',
  GetJwks: 'jwks',
  GetOpenidConfiguration: 'discovery',
  GetOauthAuthorizationServer: 'discovery',
  RegisterClient: 'register',
  DeviceAuthorization: 'device_authorization',
  Health: 'health',
}

function endpointFor(iface: string): string | null {
  return INTERFACE_TO_ENDPOINT[iface] ?? null
}

function httpBinding(iface?: SclLike['interfaces'][string]) {
  return iface?.bindings?.find((binding) => binding.kind === 'http')
}

export function loadSlo(): SloSpec {
  const performance: Record<string, SloEndpoint> = {}
  const availability: Record<string, { target: number; measurement_window: string }> = {}
  const scalability: Record<string, number> = {}
  const tokenLifetimes: Record<string, number> = {}
  const security: Record<string, number> = {}
  const dataRetention: Record<string, number> = {}

  for (const [name, o] of Object.entries(scl.objectives)) {
    if (o.kind === 'slo' && o.metric?.startsWith('latency_p99') && o.interface) {
      const ep = endpointFor(o.interface)
      if (!ep) continue
      const iface = scl.interfaces[o.interface]
      const http = httpBinding(iface)
      performance[ep] ??= {
        method: http?.method ?? 'GET',
        path: http?.path ?? '',
        p99_latency_ms: 0,
        error_rate_max: 0,
      }
      performance[ep].p99_latency_ms = parseTargetMs(String(o.target))
    } else if (o.kind === 'slo' && o.metric === 'error_rate' && o.interface) {
      const ep = endpointFor(o.interface)
      if (!ep) continue
      const iface = scl.interfaces[o.interface]
      const http = httpBinding(iface)
      performance[ep] ??= {
        method: http?.method ?? 'GET',
        path: http?.path ?? '',
        p99_latency_ms: 0,
        error_rate_max: 0,
      }
      performance[ep].error_rate_max = parseTargetRate(String(o.target))
    } else if (o.kind === 'slo' && o.metric === 'availability') {
      const key = o.interface ? (endpointFor(o.interface) ?? 'overall') : 'overall'
      const labelKey = key === 'token' ? 'token_endpoint' : key
      availability[labelKey] = {
        target: parseTargetPercent(String(o.target)),
        measurement_window: o.window ?? '30d',
      }
    } else if (o.kind === 'slo' && o.metric === 'throughput' && o.interface) {
      const ep = endpointFor(o.interface)
      if (!ep) continue
      scalability[`${ep}_requests_per_second_max`] = parseTargetThroughput(String(o.target))
    } else if (o.kind === 'lifetime') {
      // TTL を「<lower_name>_ttl_seconds」キーで slo に並べる
      const key = `${name.replace(/([a-z])([A-Z])/g, '$1_$2').toLowerCase()}_seconds`
      if (typeof o.ttl === 'string') tokenLifetimes[key] = parseDurationSeconds(o.ttl)
    } else if (o.kind === 'security') {
      // 各種値を flatten。policy が key の末尾と重なる場合は重複を避ける（例:
      // ClientAuthFailureRateLimit + policy=rate_limit_per_minute →
      // client_auth_failure_rate_limit_per_minute）。
      const key = name.replace(/([a-z])([A-Z])/g, '$1_$2').toLowerCase()
      const suffix = securityKeySuffix(o.policy)
      if (typeof o.value === 'number') {
        security[appendNoOverlap(key, suffix)] = o.value
      } else if (typeof o.value === 'string') {
        if (o.value.endsWith('d')) {
          security[`${key}_days`] = parseDurationDays(o.value)
        }
      }
    } else if (o.kind === 'retention') {
      const key = name.replace(/([a-z])([A-Z])/g, '$1_$2').toLowerCase() + '_days'
      if (typeof o.retention === 'string') {
        dataRetention[key] = parseDurationDays(o.retention)
      } else if (o.policy === 'keep_indefinitely') {
        dataRetention[key] = -1
      }
    }
  }

  // 旧 slo.yaml 互換キー（既存テストが見ているもの）
  const legacy: Record<string, number> = {}
  if (tokenLifetimes.authorization_code_lifetime_seconds !== undefined)
    legacy.authorization_code_ttl_seconds = tokenLifetimes.authorization_code_lifetime_seconds
  if (tokenLifetimes.par_request_uri_lifetime_seconds !== undefined)
    legacy.par_request_uri_ttl_seconds = tokenLifetimes.par_request_uri_lifetime_seconds
  if (tokenLifetimes.access_token_lifetime_seconds !== undefined)
    legacy.access_token_ttl_seconds = tokenLifetimes.access_token_lifetime_seconds
  if (tokenLifetimes.id_token_lifetime_seconds !== undefined)
    legacy.id_token_ttl_seconds = tokenLifetimes.id_token_lifetime_seconds
  if (tokenLifetimes.refresh_token_lifetime_seconds !== undefined)
    legacy.refresh_token_ttl_seconds = tokenLifetimes.refresh_token_lifetime_seconds
  if (tokenLifetimes.refresh_token_absolute_lifetime_seconds !== undefined)
    legacy.refresh_token_absolute_ttl_seconds =
      tokenLifetimes.refresh_token_absolute_lifetime_seconds
  if (tokenLifetimes.device_code_lifetime_seconds !== undefined)
    legacy.device_code_ttl_seconds = tokenLifetimes.device_code_lifetime_seconds
  if (tokenLifetimes.user_code_lifetime_seconds !== undefined)
    legacy.user_code_ttl_seconds = tokenLifetimes.user_code_lifetime_seconds
  if (tokenLifetimes.device_code_polling_default_interval_seconds !== undefined)
    legacy.device_code_default_polling_interval_seconds =
      tokenLifetimes.device_code_polling_default_interval_seconds

  // 旧名互換: audit_log_days を data_retention 直下にも置く
  if (dataRetention.audit_log_retention_days !== undefined) {
    dataRetention.audit_log_days = dataRetention.audit_log_retention_days
  }
  if (dataRetention.pii_purge_after_deletion_days !== undefined) {
    // すでに正しい名
  }
  if (dataRetention.consent_records_retention_days !== undefined) {
    dataRetention.consent_records_days = dataRetention.consent_records_retention_days
  }
  if (dataRetention.signing_key_archive_retention_days !== undefined) {
    dataRetention.signing_key_archive_days = dataRetention.signing_key_archive_retention_days
  }

  return {
    performance: { endpoints: performance },
    availability,
    scalability,
    token_lifetimes: { ...tokenLifetimes, ...legacy },
    security,
    data_retention: dataRetention,
  }
}

function securityKeySuffix(policy?: ObjectivePolicy): string {
  switch (policy) {
    case 'max_age':
      return '_max_age'
    case 'min_jwks_overlap':
      return '_min_jwks_overlap'
    case 'rate_limit_per_minute':
      return '_rate_limit_per_minute'
    case 'clock_skew_seconds':
      return '_seconds'
    case 'replay_window_minutes':
      return '_minutes'
    case 'alert_window_seconds':
      return '_seconds'
    default:
      return ''
  }
}

/**
 * suffix の先頭部分が key の末尾と重複していれば、その重複を捨てて連結する。
 * 例: appendNoOverlap('client_auth_failure_rate_limit', '_rate_limit_per_minute')
 *     → 'client_auth_failure_rate_limit_per_minute'
 *     (suffix の先頭 '_rate_limit' が key 末尾と一致するため省く)
 */
function appendNoOverlap(key: string, suffix: string): string {
  if (!suffix) return key
  for (let cut = suffix.length; cut > 0; cut--) {
    if (key.endsWith(suffix.slice(0, cut))) {
      return key + suffix.slice(cut)
    }
  }
  return key + suffix
}

// ===============================================================
// Observability view（scl.interfaces.*.metrics + scl.objectives.alerts を射影）
// ===============================================================

export interface MetricSpec {
  type: 'counter' | 'histogram' | 'gauge'
  description?: string
  labels?: string[]
  buckets_ms?: number[]
  slo_threshold_p99_ms?: number
  maps_to_slo?: string
  alert?: string
  alert_threshold_per_minute?: number
}

export interface AlertSpec {
  name: string
  severity: string
  expression?: string
  expression_template?: string
  for?: string
  runbook?: string
}

export interface ObservabilitySpec {
  service: { name: string; namespace?: string; version?: string }
  metrics: Record<string, MetricSpec>
  traces?: { spans: Array<{ name: string; description?: string; attributes?: string[] }> }
  logs?: {
    audit?: { source_schema?: string; sink?: string; retention_days?: number }
    application?: {
      sink?: string
      retention_days?: number
      required_fields?: string[]
      forbidden_fields?: string[]
    }
  }
  alerts?: AlertSpec[]
}

const DEFAULT_BUCKETS_MS = [1, 5, 10, 25, 50, 100, 200, 500, 1000, 2000]

function bucketsForSlo(p99: number): number[] {
  // SLO 閾値を必ず含む単調増加の bucket 集合
  const set = new Set<number>(DEFAULT_BUCKETS_MS)
  set.add(p99)
  return [...set].sort((a, b) => a - b)
}

export function loadObservability(): ObservabilitySpec {
  const slo = loadSlo()
  const metrics: Record<string, MetricSpec> = {}
  const traces: { name: string; description?: string; attributes?: string[] }[] = []

  for (const [iface, def] of Object.entries(scl.interfaces)) {
    const ep = endpointFor(iface)
    if (!ep || ep === 'health') continue
    const sloEntry = slo.performance.endpoints[ep]
    const http = httpBinding(def)
    metrics[`oauth2_${ep}_requests_total`] = {
      type: 'counter',
      description: `${http?.path ?? ep} へのリクエスト数`,
      labels: ['client_id', 'result'],
      maps_to_slo: `performance.endpoints.${ep}`,
    }
    metrics[`oauth2_${ep}_request_duration_seconds`] = {
      type: 'histogram',
      description: `${http?.path ?? ep} 所要時間`,
      labels: ['client_id'],
      buckets_ms: sloEntry ? bucketsForSlo(sloEntry.p99_latency_ms) : DEFAULT_BUCKETS_MS,
      slo_threshold_p99_ms: sloEntry?.p99_latency_ms,
      maps_to_slo: `performance.endpoints.${ep}`,
    }
    traces.push({
      name: `oauth2.${ep}`,
      description: def.description,
      attributes: ['client_id'],
    })
  }

  // ドメインカウンタ（ビジネス指標）
  const domainCounters: Array<[string, MetricSpec]> = [
    [
      'oauth2_authorization_codes_issued_total',
      { type: 'counter', description: '発行された認可コード数', labels: ['client_id'] },
    ],
    [
      'oauth2_authorization_codes_redeemed_total',
      { type: 'counter', description: '交換成功した認可コード数', labels: ['client_id'] },
    ],
    [
      'oauth2_refresh_tokens_rotated_total',
      {
        type: 'counter',
        description: 'ローテーションされた refresh token 数',
        labels: ['client_id'],
      },
    ],
    [
      'oauth2_refresh_token_reuse_detected_total',
      {
        type: 'counter',
        description: 'refresh token 再利用検知（セキュリティインシデント）',
        labels: ['client_id'],
        alert: 'critical',
      },
    ],
    [
      'oauth2_signing_key_rotations_total',
      { type: 'counter', description: '署名鍵のローテーション数', labels: ['old_kid', 'new_kid'] },
    ],
    [
      'oauth2_dpop_replay_detected_total',
      {
        type: 'counter',
        description: 'DPoP jti リプレイ検知',
        labels: ['client_id'],
        alert: 'warning',
      },
    ],
    [
      'oauth2_client_auth_failures_total',
      {
        type: 'counter',
        description: 'クライアント認証失敗（Brute-force 検知用）',
        labels: ['client_id', 'method'],
        alert_threshold_per_minute: slo.security.client_auth_failure_rate_limit_per_minute,
      },
    ],
  ]
  for (const [name, spec] of domainCounters) metrics[name] = spec

  const alerts: AlertSpec[] = [
    {
      name: 'oauth2_token_p99_breach',
      severity: 'warning',
      expression_template:
        'histogram_quantile(0.99, rate(oauth2_token_request_duration_seconds_bucket[5m])) > {{slo.performance.endpoints.token.p99_latency_ms / 1000}}',
      for: '5m',
      runbook: 'docs/runbooks/token-latency.md',
    },
    {
      name: 'oauth2_refresh_reuse_detected',
      severity: 'critical',
      expression: 'increase(oauth2_refresh_token_reuse_detected_total[1m]) > 0',
      for: '0s',
      runbook: 'docs/runbooks/refresh-reuse.md',
    },
    {
      name: 'oauth2_client_auth_failure_burst',
      severity: 'warning',
      expression_template:
        'rate(oauth2_client_auth_failures_total[1m]) > {{slo.security.client_auth_failure_rate_limit_per_minute / 60}}',
      for: '1m',
      runbook: 'docs/runbooks/client-auth-failure.md',
    },
    {
      name: 'oauth2_signing_key_too_old',
      severity: 'warning',
      expression_template:
        'time() - oauth2_signing_key_active_age_seconds > {{slo.security.signing_key_max_age_days * 86400}}',
      for: '1h',
      runbook: 'docs/runbooks/key-rotation.md',
    },
  ]

  return {
    service: { name: 'ra-oauth2-idp', namespace: 'identity', version: '0.3.0' },
    metrics,
    traces: { spans: traces },
    logs: {
      audit: {
        source_schema: 'spec/scl.yaml#models[kind=event]',
        sink: 'outbox+kafka',
        retention_days: slo.data_retention.audit_log_days,
      },
      application: {
        sink: 'stdout+otel',
        retention_days: 30,
        required_fields: ['timestamp', 'level', 'service', 'trace_id', 'span_id', 'message'],
        forbidden_fields: piiForbiddenFields(),
      },
    },
    alerts,
  }
}

function piiForbiddenFields(): string[] {
  const out = new Set<string>(['password', 'password_hash'])
  // SCL の全 model から annotations.pii==true のフィールド名を収集
  for (const m of Object.values(scl.models)) {
    const fields = (m as any).fields ?? (m as any).payload
    if (!fields) continue
    for (const [name, f] of Object.entries(
      fields as Record<string, { annotations?: { pii?: boolean } }>,
    )) {
      if (f.annotations?.pii) out.add(name)
    }
  }
  return [...out].sort()
}

// ===============================================================
// テンプレート展開（{{slo.path}} / {{slo.path / X}}）
// ===============================================================

export function expandTemplate(tpl: string, slo: SloSpec): string {
  return tpl.replace(/\{\{([^}]+)\}\}/g, (_, expr) => {
    const trimmed = String(expr).trim()
    const divMatch = trimmed.match(/^(.+?)\s*\/\s*([0-9]+(?:\.[0-9]+)?)$/)
    if (divMatch) return String(resolvePath(divMatch[1].trim(), slo) / Number(divMatch[2]))
    const mulMatch = trimmed.match(/^(.+?)\s*\*\s*([0-9]+(?:\.[0-9]+)?)$/)
    if (mulMatch) return String(resolvePath(mulMatch[1].trim(), slo) * Number(mulMatch[2]))
    return String(resolvePath(trimmed, slo))
  })
}

function resolvePath(path: string, slo: SloSpec): number {
  const segments = path.split('.').slice(1)
  let v: any = slo
  for (const seg of segments) {
    v = v?.[seg]
    if (v === undefined) throw new Error(`SLO path not found: ${path}`)
  }
  return Number(v)
}

// 互換のための再エクスポート
export { parseDurationMs, parseDurationDays, parseDurationSeconds, parseTargetMs }
export { toWire }

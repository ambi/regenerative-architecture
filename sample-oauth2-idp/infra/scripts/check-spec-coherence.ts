/**
 * Spec coherence checker
 *
 * spec/scl.yaml + infra/ + gen/ 内のクロスリファレンスを検証する。
 * RA §2.3 (単一上流ソース原則) のドリフト検知を CI に焼く実装。
 *
 * 検証内容:
 *  1. gen/openapi.yaml のパスが SCL `interfaces.<*>.bindings[kind=http].path` の全集合と一致
 *  2. infra/event-routing.yaml の event_to_topic が SCL `models[kind=event]` を網羅
 *  3. infra/migrations/0001_init.sql のテーブルカラム名が SCL entity の fields と整合
 *  4. observability ↔ SLO（SCL objectives）整合
 *
 * 実行: bun run check:coherence
 */

import { readFile } from 'fs/promises'
import { join } from 'path'

import sclDoc from '../../spec/scl.yaml'
import eventRouting from '../event-routing.yaml'
import { loadSlo, loadObservability } from './load-specs'

type SclLike = {
  vocabulary: Record<string, { aliases?: string[] }>
  models: Record<
    string,
    {
      kind: string
      fields?: Record<string, { type: string; optional?: boolean }>
      payload?: Record<string, { type: string; optional?: boolean }>
    }
  >
  interfaces: Record<
    string,
    { bindings?: Array<{ kind: string; method?: string; path?: string }>; emits?: string[] }
  >
}

const scl = sclDoc as unknown as SclLike

interface CheckResult {
  ok: boolean
  message: string
}

const results: CheckResult[] = []
const ok = (m: string) => results.push({ ok: true, message: m })
const bad = (m: string) => results.push({ ok: false, message: m })

function httpBinding(iface: SclLike['interfaces'][string]) {
  return iface.bindings?.find((binding) => binding.kind === 'http')
}

// ---------------------------------------------------------------
// 1. OpenAPI ↔ SCL interfaces
// ---------------------------------------------------------------
async function checkOpenApiVsSclInterfaces() {
  let openapi: string
  try {
    openapi = await readFile(join(import.meta.dir, '../../gen/openapi.yaml'), 'utf-8')
  } catch {
    bad('gen/openapi.yaml が存在しない (bun run gen:scl を実行)')
    return
  }
  const paths = new Set([...openapi.matchAll(/^ {2}(\/[^\s:]+):/gm)].map((m) => m[1]))
  const expected = new Set(
    Object.values(scl.interfaces)
      .map((i) => httpBinding(i)?.path)
      .filter((p): p is string => Boolean(p)),
  )
  for (const p of expected) {
    if (!paths.has(p)) bad(`gen/openapi.yaml に SCL interfaces の path ${p} がない`)
    else ok(`gen/openapi.yaml ⊇ SCL interfaces path:${p}`)
  }
  for (const p of paths) {
    if (!expected.has(p)) bad(`gen/openapi.yaml の path ${p} が SCL interfaces にない`)
  }
}

// ---------------------------------------------------------------
// 2. event-routing ↔ SCL events
// ---------------------------------------------------------------
function checkEventRoutingVsScl() {
  const r = eventRouting as {
    event_to_topic?: Record<string, string>
    topics?: Record<string, unknown>
  }
  const sclEvents = Object.entries(scl.models)
    .filter(([, m]) => m.kind === 'event')
    .map(([name]) => name)
  const topicMap = r.event_to_topic ?? {}
  const knownTopics = new Set(Object.keys(r.topics ?? {}))

  for (const ev of sclEvents) {
    if (topicMap[ev]) ok(`infra/event-routing.event_to_topic に ${ev} が存在`)
    else bad(`infra/event-routing.event_to_topic に SCL event ${ev} がない`)
  }
  for (const ev of Object.keys(topicMap)) {
    if (!sclEvents.includes(ev))
      bad(`event-routing.event_to_topic に SCL にない ${ev} が宣言されている`)
    if (!knownTopics.has(topicMap[ev]))
      bad(`event-routing.event_to_topic.${ev} = ${topicMap[ev]} が topics に未定義`)
  }
}

// ---------------------------------------------------------------
// 3. Migrations ↔ SCL entities
// ---------------------------------------------------------------
async function checkMigrationsVsScl() {
  const sql = await readFile(join(import.meta.dir, '../migrations/0001_init.sql'), 'utf-8')

  type Mapping = { table: string; model: string }
  const tables: Mapping[] = [
    { table: 'clients', model: 'OAuth2Client' },
    { table: 'users', model: 'User' },
  ]
  for (const { table, model } of tables) {
    const cols = extractColumns(sql, table)
    const m = scl.models[model]
    if (m?.kind !== 'entity') {
      bad(`SCL に ${model} (entity) が見つからない`)
      continue
    }
    const fields = Object.keys(m.fields ?? {})
    for (const f of fields) {
      if (cols.has(f)) ok(`migrations.${table} ⊇ SCL ${model}.${f}`)
      else bad(`migrations.${table} に SCL ${model}.${f} がない`)
    }
  }
}

function extractColumns(sql: string, table: string): Set<string> {
  const re = new RegExp(`CREATE TABLE (?:IF NOT EXISTS )?${table}\\s*\\(([\\s\\S]*?)\\n\\);`, 'i')
  const m = sql.match(re)
  if (!m) return new Set()
  const body = m[1]
  const cols = new Set<string>()
  for (const line of body.split('\n')) {
    const t = line.trim()
    if (!t || t.startsWith('--') || /^(CHECK|PRIMARY|FOREIGN|UNIQUE|CONSTRAINT|COMMENT)/i.test(t))
      continue
    const m2 = t.match(/^([a-zA-Z_][a-zA-Z0-9_]*)/)
    if (m2) cols.add(m2[1])
  }
  return cols
}

// ---------------------------------------------------------------
// 4. Observability ↔ SLO（SCL objectives）整合
// ---------------------------------------------------------------
function checkObservabilityVsSlo() {
  const slo = loadSlo()
  const obs = loadObservability()
  for (const [name, m] of Object.entries(obs.metrics)) {
    if (!m.maps_to_slo) continue
    const parts = m.maps_to_slo.split('.')
    let v: any = slo
    for (const p of parts) v = v?.[p]
    if (!v) {
      bad(`observability.metrics.${name}.maps_to_slo = ${m.maps_to_slo} は SLO に存在しない`)
      continue
    }
    if (m.slo_threshold_p99_ms !== undefined && v.p99_latency_ms !== m.slo_threshold_p99_ms) {
      bad(
        `observability.metrics.${name}.slo_threshold_p99_ms = ${m.slo_threshold_p99_ms} が SLO ${v.p99_latency_ms} と一致しない`,
      )
    } else if (m.slo_threshold_p99_ms !== undefined) {
      ok(`observability.${name} 閾値 ↔ slo.${m.maps_to_slo}.p99_latency_ms = ${v.p99_latency_ms}`)
    }
  }
  const auditRet = obs.logs?.audit?.retention_days
  if (auditRet && auditRet !== slo.data_retention.audit_log_days) {
    bad(
      `observability.logs.audit.retention_days = ${auditRet} が slo.audit_log_days = ${slo.data_retention.audit_log_days} と一致しない`,
    )
  } else {
    ok(`observability.audit.retention_days ↔ slo.audit_log_days`)
  }
}

// ---------------------------------------------------------------
// Main
// ---------------------------------------------------------------
async function main() {
  await checkOpenApiVsSclInterfaces()
  checkEventRoutingVsScl()
  await checkMigrationsVsScl()
  checkObservabilityVsSlo()

  const failed = results.filter((r) => !r.ok)
  const passed = results.length - failed.length

  for (const r of results) {
    // eslint-disable-next-line no-console
    console.log(`${r.ok ? '✓' : '✗'} ${r.message}`)
  }

  // eslint-disable-next-line no-console
  console.log(`\nResult: ${passed} ok, ${failed.length} failed (${results.length} total)`)
  process.exit(failed.length > 0 ? 1 : 0)
}

main().catch((e) => {
  console.error(e)
  process.exit(2)
})

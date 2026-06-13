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

import { readFile, readdir } from 'fs/promises'
import { join } from 'path'

import sclDoc from '../../spec/scl.yaml'
import eventRouting from '../event-routing.yaml'
import { loadSlo, loadObservability } from './load-specs'

type SclLike = {
  annotations?: {
    scenario_coverage?: Record<
      string,
      {
        status: 'covered' | 'partial' | 'manual' | 'missing'
        evidence?: Array<{ file: string; test?: string }>
        note?: string
      }
    >
  }
  standards?: Record<
    string,
    {
      title: string
      version: string
      url: string
      roles: string[]
      scope: string
      requirements: Array<{
        id: string
        section?: string
        strength: string
        adoption: string
        statement: string
        reason?: string
        relates_to?: Record<string, string[]>
      }>
    }
  >
  vocabulary: Record<string, { aliases?: string[] }>
  models: Record<
    string,
    {
      kind: string
      values?: string[]
      fields?: Record<string, { type: string; optional?: boolean }>
      payload?: Record<string, { type: string; optional?: boolean }>
    }
  >
  interfaces: Record<
    string,
    {
      bindings?: Array<{
        kind: string
        method?: string
        path?: string
        request_form?: 'body' | 'query' | 'form'
      }>
      input?: Record<string, { type: string; optional?: boolean }>
      emits?: string[]
    }
  >
  states: Record<
    string,
    {
      initial: string
      terminal?: string[]
      transitions: Array<{ from: string; event: string; to: string }>
    }
  >
  invariants: Record<string, unknown>
  scenarios: Record<string, unknown>
  permissions: Record<string, unknown>
  objectives: Record<string, unknown>
  user_experience?: {
    accessibility?: { standard: string; level: string }
    locales?: string[]
    screens: Record<
      string,
      { route: string; purpose: string; interfaces?: string[]; states?: string[] }
    >
    transitions?: Array<{
      from?: string
      to?: string
      trigger: string
      interface?: string
      external?: boolean
    }>
    requirements?: Array<{
      id: string
      category: string
      adoption: string
      statement: string
      reason?: string
      screens?: string[]
      interfaces?: string[]
      standards?: string[]
      scenarios?: string[]
      invariants?: string[]
    }>
  }
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

function operationId(interfaceName: string): string {
  return interfaceName[0].toLowerCase() + interfaceName.slice(1)
}

// ---------------------------------------------------------------
// 0. Vocabulary ↔ semantic names
// ---------------------------------------------------------------
function checkVocabularyCompleteness() {
  const vocabulary = new Set(Object.keys(scl.vocabulary))
  for (const [modelName, model] of Object.entries(scl.models)) {
    if (model.kind !== 'enum') continue
    for (const value of model.values ?? []) {
      if (vocabulary.has(value)) ok(`vocabulary ⊇ enum ${modelName}.${value}`)
      else bad(`vocabulary に enum ${modelName}.${value} がない`)
    }
  }

  for (const [machineName, machine] of Object.entries(scl.states)) {
    const names = new Set<string>([machine.initial, ...(machine.terminal ?? [])])
    for (const transition of machine.transitions) {
      names.add(transition.from)
      names.add(transition.event)
      names.add(transition.to)
    }
    for (const name of names) {
      if (vocabulary.has(name)) ok(`vocabulary ⊇ state ${machineName}.${name}`)
      else bad(`vocabulary に state ${machineName}.${name} がない`)
    }
  }
}

// ---------------------------------------------------------------
// 0.1 Standards / User Experience internal references
// ---------------------------------------------------------------
function checkStandardsAndUserExperience() {
  const adoptionValues = new Set(['required', 'optional', 'excluded'])
  const strengthValues = new Set(['MUST', 'MUST NOT', 'SHOULD', 'SHOULD NOT', 'MAY'])
  const sectionTargets: Record<string, Set<string>> = {
    vocabulary: new Set(Object.keys(scl.vocabulary)),
    models: new Set(Object.keys(scl.models)),
    interfaces: new Set(Object.keys(scl.interfaces)),
    states: new Set(Object.keys(scl.states)),
    invariants: new Set(Object.keys(scl.invariants)),
    scenarios: new Set(Object.keys(scl.scenarios)),
    permissions: new Set(Object.keys(scl.permissions)),
    objectives: new Set(Object.keys(scl.objectives)),
  }
  const standardIds = new Set<string>()

  for (const [standardName, standard] of Object.entries(scl.standards ?? {})) {
    if (!standard.title || !standard.version || !standard.scope || !standard.roles?.length) {
      bad(`standards.${standardName} に title/version/scope/roles が揃っていない`)
    }
    try {
      const url = new URL(standard.url)
      if (url.protocol !== 'https:') bad(`standards.${standardName}.url は HTTPS ではない`)
      else ok(`standards.${standardName}.url は HTTPS`)
    } catch {
      bad(`standards.${standardName}.url が不正: ${standard.url}`)
    }

    for (const requirement of standard.requirements ?? []) {
      if (standardIds.has(requirement.id)) {
        bad(`standards requirement id が重複: ${requirement.id}`)
      } else {
        standardIds.add(requirement.id)
        ok(`standards requirement id ${requirement.id} は一意`)
      }
      if (!strengthValues.has(requirement.strength)) {
        bad(`standards.${standardName}.${requirement.id}.strength が不正: ${requirement.strength}`)
      }
      if (!adoptionValues.has(requirement.adoption)) {
        bad(`standards.${standardName}.${requirement.id}.adoption が不正: ${requirement.adoption}`)
      }
      if (requirement.adoption === 'excluded' && !requirement.reason) {
        bad(`standards.${standardName}.${requirement.id} は excluded だが reason がない`)
      }
      if (!requirement.statement) {
        bad(`standards.${standardName}.${requirement.id}.statement がない`)
      }
      for (const [section, names] of Object.entries(requirement.relates_to ?? {})) {
        const known = sectionTargets[section]
        if (!known) {
          bad(
            `standards.${standardName}.${requirement.id}.relates_to.${section} は未知のセクション`,
          )
          continue
        }
        for (const name of names) {
          if (known.has(name)) ok(`${requirement.id} ↔ ${section}.${name}`)
          else bad(`${requirement.id} が存在しない ${section}.${name} を参照`)
        }
      }
    }
  }

  const ux = scl.user_experience
  if (!ux) return
  const screens = new Set(Object.keys(ux.screens))
  const interfaces = sectionTargets.interfaces
  const standards = new Set(Object.keys(scl.standards ?? {}))
  const uxRequirementIds = new Set<string>()

  if (ux.accessibility) {
    if (!standards.has(ux.accessibility.standard)) {
      bad(`user_experience.accessibility.standard が存在しない: ${ux.accessibility.standard}`)
    } else {
      ok(`user_experience accessibility ↔ standards.${ux.accessibility.standard}`)
    }
    if (ux.accessibility.level !== 'AA') {
      bad(`user_experience.accessibility.level は AA ではない: ${ux.accessibility.level}`)
    }
  }
  if (!ux.locales?.length) bad('user_experience.locales が空')

  for (const [screenName, screen] of Object.entries(ux.screens)) {
    if (!screen.route.startsWith('/')) bad(`user_experience.screens.${screenName}.route が不正`)
    if (!screen.purpose) bad(`user_experience.screens.${screenName}.purpose がない`)
    for (const interfaceName of screen.interfaces ?? []) {
      if (interfaces.has(interfaceName)) ok(`screen ${screenName} ↔ interfaces.${interfaceName}`)
      else bad(`screen ${screenName} が存在しない interfaces.${interfaceName} を参照`)
    }
  }

  for (const transition of ux.transitions ?? []) {
    if (!transition.from && !transition.to) {
      bad(`user_experience transition ${transition.trigger} に from/to がない`)
    }
    if (transition.from && !screens.has(transition.from)) {
      bad(`user_experience transition ${transition.trigger}.from が不正: ${transition.from}`)
    }
    if (transition.to && !screens.has(transition.to)) {
      bad(`user_experience transition ${transition.trigger}.to が不正: ${transition.to}`)
    }
    if (!transition.to && !transition.external) {
      bad(`user_experience transition ${transition.trigger} は to も external もない`)
    }
    if (transition.interface && !interfaces.has(transition.interface)) {
      bad(`user_experience transition ${transition.trigger}.interface が不正`)
    }
  }

  for (const requirement of ux.requirements ?? []) {
    if (uxRequirementIds.has(requirement.id)) {
      bad(`user_experience requirement id が重複: ${requirement.id}`)
    } else {
      uxRequirementIds.add(requirement.id)
      ok(`user_experience requirement id ${requirement.id} は一意`)
    }
    if (!adoptionValues.has(requirement.adoption)) {
      bad(`user_experience.${requirement.id}.adoption が不正: ${requirement.adoption}`)
    }
    if (requirement.adoption === 'excluded' && !requirement.reason) {
      bad(`user_experience.${requirement.id} は excluded だが reason がない`)
    }
    for (const screen of requirement.screens ?? []) {
      if (!screens.has(screen))
        bad(`user_experience.${requirement.id} が未知の screen ${screen} を参照`)
    }
    for (const interfaceName of requirement.interfaces ?? []) {
      if (!interfaces.has(interfaceName)) {
        bad(`user_experience.${requirement.id} が未知の interface ${interfaceName} を参照`)
      }
    }
    for (const standard of requirement.standards ?? []) {
      if (!standards.has(standard)) {
        bad(`user_experience.${requirement.id} が未知の standard ${standard} を参照`)
      }
    }
    for (const scenario of requirement.scenarios ?? []) {
      if (!sectionTargets.scenarios.has(scenario)) {
        bad(`user_experience.${requirement.id} が未知の scenario ${scenario} を参照`)
      }
    }
    for (const invariant of requirement.invariants ?? []) {
      if (!sectionTargets.invariants.has(invariant)) {
        bad(`user_experience.${requirement.id} が未知の invariant ${invariant} を参照`)
      }
    }
  }
}

function openApiOperationBlock(openapi: string, interfaceName: string): string | null {
  const marker = `operationId: ${operationId(interfaceName)}`
  const start = openapi.indexOf(marker)
  if (start < 0) return null
  const nextPath = openapi.indexOf('\n  /', start + marker.length)
  const nextMethodMatch = openapi
    .slice(start + marker.length)
    .match(/\n    (get|post|put|patch|delete|options|head|trace):\n/)
  const nextMethod =
    nextMethodMatch?.index === undefined
      ? -1
      : start + marker.length + nextMethodMatch.index
  const boundaries = [nextPath, nextMethod].filter((position) => position >= 0)
  return openapi.slice(start, boundaries.length > 0 ? Math.min(...boundaries) : undefined)
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

  for (const [name, iface] of Object.entries(scl.interfaces)) {
    const http = httpBinding(iface)
    if (!http?.method || !http.path) continue
    const block = openApiOperationBlock(openapi, name)
    if (!block) {
      bad(`gen/openapi.yaml に operationId ${operationId(name)} がない`)
      continue
    }
    const method = http.method.toLowerCase()
    const operationMarker = `operationId: ${operationId(name)}`
    const operationPosition = openapi.indexOf(operationMarker)
    const pathPosition = openapi.lastIndexOf(`\n  ${http.path}:\n`, operationPosition)
    const methodPosition = openapi.lastIndexOf(`\n    ${method}:\n`, operationPosition)
    if (pathPosition >= 0 && methodPosition > pathPosition) {
      ok(`gen/openapi.yaml ${http.method} ${http.path} ↔ SCL ${name}`)
    } else {
      bad(`gen/openapi.yaml に SCL ${name} の ${http.method} ${http.path} がない`)
    }

    if (!iface.input) continue
    const pathParameters = [...http.path.matchAll(/\{([^}]+)\}/g)].map((match) => match[1])
    for (const paramName of pathParameters) {
      if (
        block.includes(`name: ${paramName}`) &&
        block.includes('in: path') &&
        block.includes('required: true')
      ) {
        ok(`gen/openapi.yaml ${name} path parameter:${paramName}`)
      } else {
        bad(`gen/openapi.yaml ${name} が path parameter ${paramName} を生成していない`)
      }
    }
    const bodyInputNames = Object.keys(iface.input).filter(
      (name) => !pathParameters.includes(name),
    )
    const requestForm = http.request_form ?? 'body'
    if (bodyInputNames.length === 0) {
      if (block.includes('requestBody:'))
        bad(`gen/openapi.yaml ${name} に不要な requestBody がある`)
      else ok(`gen/openapi.yaml ${name} は path parameter 以外の request body を持たない`)
      continue
    }
    if (requestForm === 'query') {
      if (block.includes('parameters:')) ok(`gen/openapi.yaml ${name} request_form=query`)
      else bad(`gen/openapi.yaml ${name} が query parameters を生成していない`)
      if (block.includes('requestBody:'))
        bad(`gen/openapi.yaml ${name} に不要な requestBody がある`)
    } else if (requestForm === 'form') {
      if (block.includes('application/x-www-form-urlencoded:'))
        ok(`gen/openapi.yaml ${name} request_form=form`)
      else bad(`gen/openapi.yaml ${name} が form requestBody を生成していない`)
    } else if (block.includes('application/json:')) {
      ok(`gen/openapi.yaml ${name} request_form=body`)
    } else {
      bad(`gen/openapi.yaml ${name} が JSON requestBody を生成していない`)
    }
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
  // infra/migrations/README.md §「加法的変更を原則とする」より、後続マイグレーションは
  // 連番不変で ALTER TABLE で列を足す。CREATE TABLE と ALTER TABLE ADD COLUMN を
  // 通番でマージして SCL と突き合わせる。
  const migrationsDir = join(import.meta.dir, '../migrations')
  const files = (await readdir(migrationsDir)).filter((f) => f.endsWith('.sql')).sort()
  const composed: Record<string, Set<string>> = {}
  for (const file of files) {
    const sql = await readFile(join(migrationsDir, file), 'utf-8')
    mergeColumns(composed, sql)
  }

  type Mapping = { table: string; model: string }
  const tables: Mapping[] = [
    { table: 'clients', model: 'OAuth2Client' },
    { table: 'users', model: 'User' },
  ]
  for (const { table, model } of tables) {
    const cols = composed[table] ?? new Set()
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

function mergeColumns(into: Record<string, Set<string>>, sql: string): void {
  // CREATE TABLE 全体
  const createRe = /CREATE TABLE (?:IF NOT EXISTS )?([a-zA-Z_][a-zA-Z0-9_]*)\s*\(([\s\S]*?)\n\);/gi
  for (let m = createRe.exec(sql); m !== null; m = createRe.exec(sql)) {
    const table = m[1]
    const cols = into[table] ?? new Set<string>()
    for (const line of m[2].split('\n')) {
      const t = line.trim()
      if (!t || t.startsWith('--') || /^(CHECK|PRIMARY|FOREIGN|UNIQUE|CONSTRAINT|COMMENT)/i.test(t))
        continue
      const m2 = t.match(/^([a-zA-Z_][a-zA-Z0-9_]*)/)
      if (m2) cols.add(m2[1])
    }
    into[table] = cols
  }
  // ALTER TABLE … ADD COLUMN
  const alterRe =
    /ALTER TABLE\s+([a-zA-Z_][a-zA-Z0-9_]*)\s+ADD COLUMN(?:\s+IF NOT EXISTS)?\s+([a-zA-Z_][a-zA-Z0-9_]*)/gi
  for (let m = alterRe.exec(sql); m !== null; m = alterRe.exec(sql)) {
    const table = m[1]
    const col = m[2]
    const cols = into[table] ?? new Set<string>()
    cols.add(col)
    into[table] = cols
  }
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
// 5. SCL scenarios ↔ executable/manual coverage
// ---------------------------------------------------------------
async function checkScenarioCoverage() {
  const coverage = scl.annotations?.scenario_coverage ?? {}
  const scenarioNames = Object.keys(scl.scenarios)
  const knownScenarios = new Set(scenarioNames)

  for (const name of scenarioNames) {
    const entry = coverage[name]
    if (!entry) {
      bad(`annotations.scenario_coverage に SCL scenario "${name}" がない`)
      continue
    }
    if (!['covered', 'partial', 'manual', 'missing'].includes(entry.status)) {
      bad(`scenario_coverage.${name}.status が不正: ${entry.status}`)
      continue
    }
    if (entry.status === 'missing') {
      ok(`scenario_coverage.${name} は missing として明示`)
      continue
    }
    if (!entry.evidence?.length) {
      bad(`scenario_coverage.${name} に evidence がない`)
      continue
    }
    for (const evidence of entry.evidence) {
      const path = join(import.meta.dir, '../..', evidence.file)
      let content: string
      try {
        content = await readFile(path, 'utf-8')
      } catch {
        bad(`scenario_coverage.${name}.evidence.file が存在しない: ${evidence.file}`)
        continue
      }
      if (evidence.test && !content.includes(evidence.test)) {
        bad(
          `scenario_coverage.${name}.evidence.test が ${evidence.file} に見つからない: ${evidence.test}`,
        )
      } else {
        ok(
          `scenario_coverage.${name} ↔ ${evidence.file}${evidence.test ? `#${evidence.test}` : ''}`,
        )
      }
    }
  }

  for (const name of Object.keys(coverage)) {
    if (!knownScenarios.has(name)) {
      bad(`annotations.scenario_coverage に SCL scenarios に存在しない "${name}" がある`)
    }
  }
}

// ---------------------------------------------------------------
// Main
// ---------------------------------------------------------------
async function main() {
  checkVocabularyCompleteness()
  checkStandardsAndUserExperience()
  await checkOpenApiVsSclInterfaces()
  checkEventRoutingVsScl()
  await checkMigrationsVsScl()
  checkObservabilityVsSlo()
  await checkScenarioCoverage()

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

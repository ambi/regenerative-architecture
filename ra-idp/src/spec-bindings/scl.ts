/**
 * Layer 3 — Specification Binding (TypeScript)
 *
 * spec/scl.yaml （単一上流ソース）を読み込み、TypeScript の型と派生ビューを提供する。
 *
 * 命名規約:
 *  - SCL の名前 = PascalCase（型・状態・イベント・アクション）
 *  - ワイヤ形式 = snake_case（vocabulary[].aliases[0] から取得）
 *
 * 別言語に移植する場合は spec/scl.yaml を直接消費し、本ファイルを該当言語版で置き換える。
 */

import sclDoc from '../../spec/scl.yaml'

// ===============================================================
// 型
// ===============================================================

export type VocabularyEntry = {
  definition: string
  aliases?: string[]
  context?: string
  not_to_confuse_with?: Array<{ term: string; reason: string }>
  annotations?: Record<string, unknown>
}

export type FieldDef = {
  type: string
  optional?: boolean
  default?: unknown
  constraints?: Array<string | Record<string, unknown>>
  description?: string
  annotations?: Record<string, unknown>
}

export type Model =
  | {
      kind: 'entity'
      identity?: string | string[]
      fields: Record<string, FieldDef>
      description?: string
      annotations?: Record<string, unknown>
    }
  | {
      kind: 'value_object'
      fields: Record<string, FieldDef>
      description?: string
      annotations?: Record<string, unknown>
    }
  | { kind: 'enum'; values: string[]; description?: string; annotations?: Record<string, unknown> }
  | {
      kind: 'event'
      payload?: Record<string, FieldDef>
      description?: string
      annotations?: Record<string, unknown>
    }
  | {
      kind: 'error'
      payload?: Record<string, FieldDef>
      description?: string
      annotations?: Record<string, unknown>
    }

export type Binding =
  | {
      kind: 'http'
      method: string
      path: string
      successful_status_codes?: string[]
      request_form?: 'body' | 'query' | 'form'
      headers?: Record<string, FieldDef>
      description?: string
    }
  | {
      kind: 'grpc'
      service: string
      method: string
      streaming?: 'unary' | 'client' | 'server' | 'bidi'
      description?: string
    }
  | {
      kind: 'event'
      channel: string
      direction: 'produce' | 'consume'
      delivery?: 'at_most_once' | 'at_least_once' | 'exactly_once'
      ordering?: 'none' | 'per_key' | 'global'
      partition_key?: string
      description?: string
    }
  | {
      kind: 'graphql'
      operation: 'query' | 'mutation' | 'subscription'
      field: string
      description?: string
    }
  | {
      kind: 'cli'
      command: string
      args?: Array<{ name: string; position: number }>
      flags?: Array<{ name: string; short?: string; required?: boolean; repeatable?: boolean }>
      stdin?: string
      stdout?: string
      exit_codes?: Record<string, number>
      description?: string
    }
  | {
      kind: 'sdk'
      function: string
      description?: string
    }
  | {
      kind: 'schedule'
      cron?: string
      every?: string
      description?: string
    }
  | { kind: string; [key: string]: unknown }

export type Interface = {
  description?: string
  steps?: string[]
  input?: Record<string, FieldDef>
  output?: Record<string, FieldDef>
  errors?: string[]
  emits?: string[]
  idempotent?: boolean
  read_only?: boolean
  bindings?: Binding[]
  annotations?: Record<string, unknown>
}

export type Transition = {
  from: string
  event: string
  to: string
  guard?: unknown
  effect?: string[]
}

export type StateMachine = {
  description?: string
  target: string
  initial: string
  terminal?: string[]
  transitions: Transition[]
  polling?: Record<string, unknown>
  annotations?: Record<string, unknown>
}

export type Property = {
  description?: string
  target?: string
  assuming?: unknown
  always?: unknown
  never?: unknown
  eventually?: unknown
  within?: string
  severity?: 'must' | 'should'
  annotations?: Record<string, unknown>
}

export type Scenario = {
  description?: string
  steps: string[]
  where?: Array<Record<string, unknown>>
  tags?: string[]
  annotations?: Record<string, unknown>
}

export type Permission = {
  description?: string
  actor: string
  action: string
  resource: string
  allow_when?: unknown
  deny_when?: unknown
  annotations?: Record<string, unknown>
}

export type RetentionPolicy =
  | 'keep_indefinitely'
  | 'keep'
  | 'append_only'
  | 'delete_after'
  | 'purge_pii_after'
  | 'archive_after'
  | 'archive'

export type SecurityPolicy =
  | 'max_age'
  | 'min_jwks_overlap'
  | 'rate_limit_per_minute'
  | 'clock_skew_seconds'
  | 'replay_window_minutes'
  | 'alert_window_seconds'

export type ObjectivePolicy = RetentionPolicy | SecurityPolicy

export type SloMetric =
  | 'latency_p50'
  | 'latency_p95'
  | 'latency_p99'
  | 'availability'
  | 'error_rate'
  | 'throughput'

type ObjectiveBase = {
  description?: string
  reference?: string
  note?: string
  annotations?: Record<string, unknown>
}

export type SloObjective = ObjectiveBase & {
  kind: 'slo'
  metric: SloMetric
  target: string
  interface?: string
  window?: string
}

export type RetentionObjective = ObjectiveBase & {
  kind: 'retention'
  target: string
  policy: RetentionPolicy
  retention?: string
}

export type LifetimeObjective = ObjectiveBase & {
  kind: 'lifetime'
  target: string
  ttl: string
  single_use?: boolean
}

export type SecurityObjective = ObjectiveBase & {
  kind: 'security'
  policy: SecurityPolicy
  value: number | string
  target?: string
}

export type Objective = SloObjective | RetentionObjective | LifetimeObjective | SecurityObjective

export type Adoption = 'required' | 'optional' | 'excluded'

export type SclReferences = Partial<
  Record<
    | 'vocabulary'
    | 'models'
    | 'interfaces'
    | 'state_machines'
    | 'properties'
    | 'scenarios'
    | 'permissions'
    | 'objectives',
    string[]
  >
>

export type StandardRequirement = {
  id: string
  section?: string
  strength: 'MUST' | 'MUST NOT' | 'SHOULD' | 'SHOULD NOT' | 'MAY'
  adoption: Adoption
  statement: string
  reason?: string
  relates_to?: SclReferences
}

export type Standard = {
  title: string
  version: string
  url: string
  roles: string[]
  scope: string
  requirements: StandardRequirement[]
}

export type UserExperienceScreen = {
  route: string
  purpose: string
  interfaces?: string[]
  states?: string[]
}

export type UserExperienceTransition = {
  from?: string
  to?: string
  trigger: string
  interface?: string
  external?: boolean
}

export type UserExperienceRequirement = {
  id: string
  category: 'security' | 'accessibility' | 'privacy' | 'localization' | 'usability'
  adoption: Adoption
  statement: string
  reason?: string
  screens?: string[]
  interfaces?: string[]
  standards?: string[]
  scenarios?: string[]
  properties?: string[]
}

export type UserExperience = {
  accessibility?: { standard: string; level: string }
  locales?: string[]
  screens: Record<string, UserExperienceScreen>
  transitions?: UserExperienceTransition[]
  requirements?: UserExperienceRequirement[]
}

export type SclDocument = {
  system: string
  spec_version: string
  standards?: Record<string, Standard>
  vocabulary: Record<string, VocabularyEntry>
  models: Record<string, Model>
  interfaces: Record<string, Interface>
  state_machines: Record<string, StateMachine>
  properties: Record<string, Property>
  scenarios: Record<string, Scenario>
  permissions: Record<string, Permission>
  objectives: Record<string, Objective>
  user_experience?: UserExperience
  annotations?: Record<string, unknown>
}

export const scl = sclDoc as unknown as SclDocument

export type HttpBinding = Extract<Binding, { kind: 'http' }>

export function httpBinding(iface: Interface): HttpBinding | undefined {
  return iface.bindings?.find((binding): binding is HttpBinding => binding.kind === 'http')
}

// ===============================================================
// 命名変換: PascalCase → ワイヤ形式
// ===============================================================

/**
 * SCL の PascalCase 名をワイヤ形式（snake_case / 標準仕様の値）に変換する。
 * 規則:
 *  1. vocabulary に登録され、最初の wire 形式 alias を返す（snake_case、URN を含む）
 *  2. それ以外は元の名前をそのまま返す（PS256, ES256, S256 のような頭字語・トークン）
 *
 * wire 形式とは「英小文字で始まり、英小文字・数字・`_:.-` のみを含む」もの。
 * 例: `device_code`, `client_secret_basic`, `urn:ietf:params:oauth:grant-type:device_code`
 */
const WIRE_ALIAS_PATTERN = /^[a-z][a-z0-9_:.-]*$/

export function toWire(name: string): string {
  const entry = scl.vocabulary[name]
  if (entry?.aliases) {
    for (const alias of entry.aliases) {
      if (WIRE_ALIAS_PATTERN.test(alias)) return alias
    }
  }
  return name
}

/** PascalCase 名のリストをワイヤ形式に変換 */
export function toWireAll(names: string[]): string[] {
  return names.map(toWire)
}

// ===============================================================
// 派生ビュー
// ===============================================================

/** モデルが enum なら値のリストを SCL 形式（PascalCase）で返す */
export function enumValues(modelName: string): string[] {
  const m = scl.models[modelName]
  if (m?.kind !== 'enum') throw new Error(`${modelName} is not an enum`)
  return m.values
}

/** enum 値のリストをワイヤ形式（snake_case 等）で返す */
export function enumWireValues(modelName: string): string[] {
  return toWireAll(enumValues(modelName))
}

/** state machine の全状態を SCL 形式で取得（initial + terminal + transitions の from/to を結合） */
export function statesOf(machineName: string): string[] {
  const sm = scl.state_machines[machineName]
  if (!sm) throw new Error(`state machine ${machineName} not found`)
  const set = new Set<string>([sm.initial, ...(sm.terminal ?? [])])
  for (const t of sm.transitions) {
    set.add(t.from)
    set.add(t.to)
  }
  return [...set]
}

/** state machine の全イベントを SCL 形式で取得 */
export function eventsOf(machineName: string): string[] {
  const sm = scl.state_machines[machineName]
  if (!sm) throw new Error(`state machine ${machineName} not found`)
  return [...new Set(sm.transitions.map((t) => t.event))]
}

/** state machine の遷移を {state: {event: state}} の入れ子マップ（ワイヤ形式）で返す */
export function transitionsAsWireMap(machineName: string): Record<string, Record<string, string>> {
  const sm = scl.state_machines[machineName]
  if (!sm) throw new Error(`state machine ${machineName} not found`)
  const result: Record<string, Record<string, string>> = {}
  for (const s of statesOf(machineName)) result[toWire(s)] = {}
  for (const t of sm.transitions) {
    result[toWire(t.from)][toWire(t.event)] = toWire(t.to)
  }
  return result
}

/** state machine の終端状態をワイヤ形式で返す */
export function terminalWireStates(machineName: string): string[] {
  const sm = scl.state_machines[machineName]
  if (!sm) throw new Error(`state machine ${machineName} not found`)
  return (sm.terminal ?? []).map(toWire)
}

/** 全 vocabulary エントリの「コード」（最初の snake_case alias または名前そのもの）の集合 */
export function vocabularyCodes(): Set<string> {
  const codes = new Set<string>()
  for (const name of Object.keys(scl.vocabulary)) {
    codes.add(toWire(name))
  }
  return codes
}

/** モデルから JSON Schema 風オブジェクトを派生（gen 用の薄い変換） */
export function modelToJsonSchema(modelName: string): Record<string, unknown> {
  const m = scl.models[modelName]
  if (!m) throw new Error(`model ${modelName} not found`)
  switch (m.kind) {
    case 'entity':
    case 'value_object': {
      const props: Record<string, unknown> = {}
      const required: string[] = []
      for (const [fname, fdef] of Object.entries(m.fields)) {
        props[fname] = typeToJsonSchema(fdef.type)
        if (!fdef.optional) required.push(fname)
      }
      return {
        $id: `urn:idp:${modelName}`,
        type: 'object',
        additionalProperties: false,
        properties: props,
        required,
        ...(m.description ? { description: m.description } : {}),
      }
    }
    case 'enum':
      return { type: 'string', enum: toWireAll(m.values) }
    case 'event':
    case 'error': {
      const props: Record<string, unknown> = { type: { const: modelName } }
      const required = ['type']
      if (m.payload) {
        for (const [fname, fdef] of Object.entries(m.payload)) {
          props[fname] = typeToJsonSchema(fdef.type)
          if (!fdef.optional) required.push(fname)
        }
      }
      return { type: 'object', properties: props, required }
    }
  }
}

function typeToJsonSchema(t: string): Record<string, unknown> {
  // パラメトリック: List<X>, Set<X>, Map<K, V>
  const listM = t.match(/^List<(.+)>$/) ?? t.match(/^Set<(.+)>$/)
  if (listM) return { type: 'array', items: typeToJsonSchema(listM[1]) }
  const mapM = t.match(/^Map<\s*([^,]+)\s*,\s*(.+)\s*>$/)
  if (mapM) return { type: 'object', additionalProperties: typeToJsonSchema(mapM[2]) }
  if (t.startsWith('OneOf<')) return { description: t }
  switch (t) {
    case 'String':
      return { type: 'string' }
    case 'Integer':
      return { type: 'integer' }
    case 'Float':
      return { type: 'number' }
    case 'Boolean':
      return { type: 'boolean' }
    case 'UUID':
      return { type: 'string', format: 'uuid' }
    case 'Date':
      return { type: 'string', format: 'date' }
    case 'Timestamp':
      return { type: 'string', format: 'date-time' }
    case 'Duration':
      return { type: 'string' }
    case 'JSON':
      return {}
    case 'Bytes':
      return { type: 'string', contentEncoding: 'base64' }
    case 'Uri':
      return { type: 'string', format: 'uri' }
    case 'Audience':
      return { oneOf: [{ type: 'string' }, { type: 'array', items: { type: 'string' } }] }
  }
  // user-defined model reference
  const m = scl.models[t]
  if (m?.kind === 'enum') return { type: 'string', enum: toWireAll(m.values) }
  return { $ref: `${t}.json` }
}

// ===============================================================
// 便宜上の名前付きエクスポート（よく使う state_machines）
// ===============================================================

export const AUTH_CODE_FLOW = 'AuthorizationCodeFlow'
export const DEVICE_CODE_FLOW = 'DeviceCodeFlow'

/**
 * Types for every artifact the tool renders: SCL document, ADRs (incl.
 * CONCEPTION), and work items with optional completion records.
 *
 * SCL types follow SPECIFICATION_CORE_LANGUAGE.md §2–§3. Change types
 * mirror the JSON Schemas under tools/yaml-check/schemas/.
 */

// ─── SCL ───────────────────────────────────────────────────────────

export const SECTION_KINDS = [
  'standards',
  'context_map',
  'glossary',
  'models',
  'interfaces',
  'states',
  'invariants',
  'scenarios',
  'permissions',
  'objectives',
  'user_experience',
] as const

export type SectionKind = (typeof SECTION_KINDS)[number]

export interface SclDocument {
  system: string
  spec_version: string
  context?: string
  annotations?: Record<string, unknown>
  standards?: Record<string, Standard>
  context_map?: Record<string, ContextMapEntry>
  glossary?: Record<string, GlossaryEntry>
  models?: Record<string, Model>
  interfaces?: Record<string, Interface>
  states?: Record<string, StateMachine>
  invariants?: Record<string, Invariant>
  scenarios?: Record<string, Scenario>
  permissions?: Record<string, Permission>
  objectives?: Record<string, Objective>
  user_experience?: UserExperience
}

export interface ContextMapEntry {
  path?: string
  description?: string
  publishes?: string[]
  depends_on?: Record<string, { uses?: string[]; via?: string; reason?: string }>
  annotations?: Record<string, unknown>
}

export interface SclContextDocument {
  name: string
  path: string
  document: SclDocument
}

export interface SclBundle {
  root: SclDocument
  contexts: SclContextDocument[]
}

export interface Standard {
  title?: string
  version?: string
  url?: string
  roles?: string[]
  scope?: string
  requirements?: StandardRequirement[]
}

export interface StandardRequirement {
  id?: string
  section?: string
  strength?: string
  adoption?: 'required' | 'optional' | 'excluded'
  statement?: string
  reason?: string
  relates_to?: Record<string, string[]>
}

export interface UserExperience {
  accessibility?: { standard?: string; level?: string }
  locales?: string[]
  screens?: Record<
    string,
    { route?: string; purpose?: string; interfaces?: string[]; states?: string[] }
  >
  transitions?: Array<{
    from?: string
    to?: string
    trigger?: string
    interface?: string
    external?: boolean
  }>
  requirements?: Array<{
    id?: string
    category?: string
    adoption?: 'required' | 'optional' | 'excluded'
    statement?: string
    reason?: string
    screens?: string[]
    interfaces?: string[]
    standards?: string[]
    scenarios?: string[]
    invariants?: string[]
  }>
}

export interface GlossaryEntry {
  definition?: string
  description?: string
  aliases?: string[]
  context?: string
  not_to_confuse_with?: Array<{ term?: string; reason?: string }>
  annotations?: Record<string, unknown>
}

export interface Field {
  type?: unknown
  fields?: Record<string, Field>
  optional?: boolean
  default?: unknown
  constraints?: unknown[]
  description?: string
  annotations?: Record<string, unknown>
}

export interface Model {
  kind?: string
  description?: string
  identity?: string | string[]
  annotations?: Record<string, unknown>
  values?: string[]
  fields?: Record<string, Field>
  payload?: Record<string, Field>
}

export interface Interface {
  description?: string
  steps?: string[]
  input?: Record<string, Field>
  output?: Record<string, Field>
  errors?: string[]
  emits?: string[]
  idempotent?: boolean
  read_only?: boolean
  bindings?: Binding[]
  annotations?: Record<string, unknown>
}

export type Binding = { kind: string; description?: string } & Record<string, unknown>

export interface StateMachine {
  description?: string
  annotations?: Record<string, unknown>
  target?: string
  initial?: string
  terminal?: string[]
  transitions?: Array<{
    from?: string
    to?: string
    event?: string
    on?: string
    guard?: unknown
    effect?: string[]
  }>
}

export interface Invariant {
  description?: string
  annotations?: Record<string, unknown>
  target?: string
  assuming?: unknown
  always?: unknown
  never?: unknown
  eventually?: unknown
  within?: string
  severity?: string
}

export interface Scenario {
  description?: string
  annotations?: Record<string, unknown>
  tags?: string[]
  goal?: string
  primary_actor?: string
  scope?: string
  level?: string
  preconditions?: string[]
  success_guarantees?: string[]
  main_success?: string[]
  extensions?: Array<{ at?: string | number; condition?: string; steps?: string[] }>
  steps?: string[]
  where?: Array<Record<string, unknown>>
}

export interface Permission {
  description?: string
  annotations?: Record<string, unknown>
  actor?: string
  protects?: string[]
  operation?: string
  resource?: string
  allow_when?: unknown
  deny_when?: unknown
}

export interface Objective {
  kind?: string
  description?: string
  annotations?: Record<string, unknown>
  [k: string]: unknown
}

// ─── Decisions (CONCEPTION + ADR) ──────────────────────────────────

export interface DecisionDoc {
  /** Stable slug used as the in-page anchor (e.g. "adr-001-...", "conception"). */
  id: string
  /** Display title parsed from the first markdown heading. */
  title: string
  /** Document kind drives the navigation grouping. */
  kind: 'conception' | 'adr'
  /** Source filename, kept for "view source" links. */
  filename: string
  /** Raw markdown body (heading line dropped). */
  body: string
  /** Best-effort ADR number, parsed from the filename when applicable. */
  number?: number
}

// ─── Work items with optional completion records ────────────────────

export interface WorkItem {
  id: string
  title?: string
  status?: 'pending' | 'in_progress' | 'completed' | 'cancelled'
  created_at?: string
  authors?: string[]
  risk?: 'low' | 'medium' | 'high' | 'critical'
  motivation?: string
  scope?: unknown
  out_of_scope?: unknown
  affected_guarantees?: unknown
  verification?: unknown
  risk_notes?: string
  completion?: Completion
  [k: string]: unknown
}

export interface Completion {
  completed_at?: string
  summary?: string
  verification?: unknown
  evidence?: unknown
  affected_guarantees_state?: unknown
  remaining_guarantees_state?: unknown
  residual_risk?: unknown
  semantic_diff?: unknown
  traceability?: unknown
  human_decisions?: unknown
  approver_note?: string
  [k: string]: unknown
}

export interface ChangeEntry {
  /** File stem under `work-items/` (also the in-page anchor). */
  id: string
  work_item: WorkItem
}

// ─── Top-level page input ──────────────────────────────────────────

export interface SiteInput {
  scl: SclDocument | SclBundle
  decisions: DecisionDoc[]
  work_items: ChangeEntry[]
  /** Optional override for the document <title> and page header. */
  title?: string
}

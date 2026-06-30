/**
 * Pure transform: SCL `models` -> JSON Schema 2020-12.
 *
 * This is the smallest end-to-end demonstration of the Regenerative
 * Architecture thesis: a downstream artifact (here, JSON Schema) derived
 * mechanically from the single upstream source of truth (the SCL spec).
 * The schema is regenerated, never hand-maintained.
 *
 * Every model across the context bundle becomes one entry under `$defs`, so
 * cross-context references resolve as `#/$defs/<ModelName>`. No IO here.
 */

import type {
  Field,
  Interface,
  Model,
  SclBundle,
  SclDocument,
} from '../../scl-to-html/src/types.ts'

export type JsonSchema = Record<string, unknown>

/** SCL scalar type name -> JSON Schema fragment. */
const PRIMITIVES: Record<string, JsonSchema> = {
  String: { type: 'string' },
  Text: { type: 'string' },
  Integer: { type: 'integer' },
  Number: { type: 'number' },
  Float: { type: 'number' },
  Decimal: { type: 'number' },
  Boolean: { type: 'boolean' },
  Timestamp: { type: 'string', format: 'date-time' },
  DateTime: { type: 'string', format: 'date-time' },
  Date: { type: 'string', format: 'date' },
  Duration: { type: 'string' },
  UUID: { type: 'string', format: 'uuid' },
  Email: { type: 'string', format: 'email' },
  URL: { type: 'string', format: 'uri' },
  URI: { type: 'string', format: 'uri' },
  Bytes: { type: 'string' },
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v)
}

/** Resolve a field `type` (scalar, model ref, or `Name[]`) to a schema. */
function typeToSchema(type: unknown, modelNames: ReadonlySet<string>): JsonSchema {
  if (typeof type !== 'string') return {}
  const name = type.trim()
  if (name.endsWith('[]')) {
    return { type: 'array', items: typeToSchema(name.slice(0, -2).trim(), modelNames) }
  }
  if (name in PRIMITIVES) return { ...PRIMITIVES[name] }
  if (modelNames.has(name)) return { $ref: `#/$defs/${name}` }
  // Unknown type (e.g. a Map<…> or an external type): stay permissive rather
  // than emit an invalid schema. The dangling-ref test guards real models.
  return {}
}

function applyConstraints(schema: JsonSchema, constraints: unknown[]): void {
  for (const c of constraints) {
    if (c === 'non_empty') {
      if (schema.type === 'array') schema.minItems = 1
      else schema.minLength = 1
      continue
    }
    if (!isRecord(c)) continue
    if (typeof c.max_length === 'number') schema.maxLength = c.max_length
    if (typeof c.min_length === 'number') schema.minLength = c.min_length
    if (typeof c.pattern === 'string') schema.pattern = c.pattern
    if (typeof c.min === 'number') schema.minimum = c.min
    if (typeof c.max === 'number') schema.maximum = c.max
  }
}

export function fieldToSchema(field: Field, modelNames: ReadonlySet<string>): JsonSchema {
  // An inline object field declares nested `fields` instead of a named type.
  const schema: JsonSchema = field.fields
    ? fieldsToSchema(field.fields, modelNames)
    : typeToSchema(field.type, modelNames)
  if (Array.isArray(field.constraints)) applyConstraints(schema, field.constraints)
  if (field.default !== undefined) schema.default = field.default
  if (field.description) schema.description = field.description
  return schema
}

/**
 * Build an object schema from a field map (a model's fields, or an interface
 * input/output). Refs use the `#/$defs/` base; callers targeting another
 * document (e.g. OpenAPI components) rewrite the base with `rewriteRefs`.
 */
export function fieldsToSchema(
  fields: Record<string, Field>,
  modelNames: ReadonlySet<string>,
): JsonSchema {
  const properties: Record<string, JsonSchema> = {}
  const required: string[] = []
  for (const [name, field] of Object.entries(fields)) {
    properties[name] = fieldToSchema(field, modelNames)
    if (!field.optional) required.push(name)
  }
  const schema: JsonSchema = { type: 'object', properties, additionalProperties: false }
  if (required.length > 0) schema.required = required
  return schema
}

export function modelToSchema(model: Model, modelNames: ReadonlySet<string>): JsonSchema {
  let schema: JsonSchema
  switch (model.kind) {
    case 'enum':
      schema = { type: 'string', enum: model.values ?? [] }
      break
    case 'event':
      schema = fieldsToSchema(model.payload ?? {}, modelNames)
      break
    case 'error':
      schema = fieldsToSchema(model.fields ?? {}, modelNames)
      break
    default:
      // entity / value_object
      schema = fieldsToSchema(model.fields ?? {}, modelNames)
  }
  if (model.description) schema.description = model.description
  return schema
}

function docsOf(bundle: SclBundle | SclDocument): SclDocument[] {
  return 'contexts' in bundle ? [bundle.root, ...bundle.contexts.map((c) => c.document)] : [bundle]
}

/** Collect every model in the bundle (root + each context). */
export function collectModels(bundle: SclBundle | SclDocument): Record<string, Model> {
  const models: Record<string, Model> = {}
  for (const doc of docsOf(bundle)) {
    for (const [name, model] of Object.entries(doc.models ?? {})) models[name] = model
  }
  return models
}

/** Collect every interface in the bundle (root + each context). */
export function collectInterfaces(bundle: SclBundle | SclDocument): Record<string, Interface> {
  const interfaces: Record<string, Interface> = {}
  for (const doc of docsOf(bundle)) {
    for (const [name, iface] of Object.entries(doc.interfaces ?? {})) interfaces[name] = iface
  }
  return interfaces
}

export function generateModelSchemas(bundle: SclBundle | SclDocument): JsonSchema {
  const system = 'contexts' in bundle ? bundle.root.system : bundle.system
  const models = collectModels(bundle)
  const modelNames = new Set(Object.keys(models))

  const defs: Record<string, JsonSchema> = {}
  for (const name of [...modelNames].sort()) {
    const model = models[name]
    if (model) defs[name] = modelToSchema(model, modelNames)
  }

  return {
    $schema: 'https://json-schema.org/draft/2020-12/schema',
    $id: `https://regenerative-architecture/generated/${system}.models.json`,
    title: `${system} — generated model schemas`,
    description:
      'Generated from SCL models by scl-to-jsonschema. Do not edit by hand; regenerate from the spec.',
    $defs: defs,
  }
}

/** Collect the names referenced by `$ref`s under the given local prefix. */
export function collectRefNames(root: unknown, prefix = '#/$defs/'): string[] {
  const names = new Set<string>()
  const walk = (node: unknown): void => {
    if (Array.isArray(node)) {
      for (const item of node) walk(item)
      return
    }
    if (!isRecord(node)) return
    for (const [key, value] of Object.entries(node)) {
      if (key === '$ref' && typeof value === 'string' && value.startsWith(prefix)) {
        names.add(value.slice(prefix.length))
      } else {
        walk(value)
      }
    }
  }
  walk(root)
  return [...names].sort()
}

/** Names referenced under `prefix` for which `known` has no entry. */
export function missingRefs(
  root: unknown,
  known: ReadonlySet<string>,
  prefix = '#/$defs/',
): string[] {
  return collectRefNames(root, prefix).filter((n) => !known.has(n))
}

/** Return every `#/$defs/<name>` referenced in the schema that has no def. */
export function danglingRefs(schema: JsonSchema): string[] {
  const defs = isRecord(schema.$defs) ? schema.$defs : {}
  return missingRefs(schema, new Set(Object.keys(defs)))
}

/** Rewrite every local `$ref` from one base prefix to another, in place. */
export function rewriteRefs(node: unknown, fromPrefix: string, toPrefix: string): void {
  if (Array.isArray(node)) {
    for (const item of node) rewriteRefs(item, fromPrefix, toPrefix)
    return
  }
  if (!isRecord(node)) return
  for (const [key, value] of Object.entries(node)) {
    if (key === '$ref' && typeof value === 'string' && value.startsWith(fromPrefix)) {
      node.$ref = toPrefix + value.slice(fromPrefix.length)
    } else {
      rewriteRefs(value, fromPrefix, toPrefix)
    }
  }
}

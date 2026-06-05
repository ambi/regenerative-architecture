/**
 * Generator: spec/scl.yaml → gen/*.json + gen/openapi.yaml
 *
 * SCL の models / interfaces から JSON Schema / OpenAPI を派生する。
 *
 * 実行: bun run gen:scl
 */

import { mkdir, writeFile, readdir, unlink } from 'fs/promises'
import { join } from 'path'

import sclDoc from '../../spec/scl.yaml'

type Field = {
  type: string
  optional?: boolean
  default?: unknown
  constraints?: Array<string | Record<string, unknown>>
  description?: string
  annotations?: Record<string, unknown>
}

type Model =
  | {
      kind: 'entity'
      identity?: string | string[]
      fields: Record<string, Field>
      description?: string
    }
  | { kind: 'value_object'; fields: Record<string, Field>; description?: string }
  | { kind: 'enum'; values: string[]; description?: string }
  | { kind: 'event'; payload?: Record<string, Field>; description?: string }
  | { kind: 'error'; payload?: Record<string, Field>; description?: string }

type Interface = {
  description?: string
  bindings?: Array<{
    kind: string
    method?: string
    path?: string
    successful_status_codes?: string[]
    headers?: Record<string, Field>
  }>
  input?: Record<string, Field>
  output?: Record<string, Field>
  errors?: string[]
  emits?: string[]
}

type SclLike = {
  system: string
  spec_version: string
  vocabulary: Record<string, { aliases?: string[] }>
  models: Record<string, Model>
  interfaces: Record<string, Interface>
}

const scl = sclDoc as unknown as SclLike

function toWire(name: string): string {
  const v = scl.vocabulary[name]
  if (v?.aliases) for (const a of v.aliases) if (/^[a-z][a-z0-9_:.-]*$/.test(a)) return a
  return name
}

function httpBinding(iface: Interface) {
  return iface.bindings?.find((binding) => binding.kind === 'http')
}

function applyConstraints(schema: any, fdef: Field): any {
  if (!fdef.constraints) return schema
  const out = { ...schema }
  for (const c of fdef.constraints) {
    if (c === 'non_empty') {
      if (out.type === 'string') out.minLength = Math.max(out.minLength ?? 0, 1)
      if (out.type === 'array') out.minItems = Math.max(out.minItems ?? 0, 1)
    } else if (c === 'unique') {
      if (out.type === 'array') out.uniqueItems = true
    } else if (typeof c === 'object' && c !== null) {
      if ('max_length' in c) out.maxLength = c.max_length
      if ('min_length' in c) out.minLength = c.min_length
      if ('max' in c) out.maximum = c.max
      if ('min' in c) out.minimum = c.min
      if ('pattern' in c) out.pattern = c.pattern
      if ('format' in c) out.format = c.format
    }
  }
  return out
}

function typeToSchema(t: string): any {
  const listM = t.match(/^List<(.+)>$/) ?? t.match(/^Set<(.+)>$/)
  if (listM) return { type: 'array', items: typeToSchema(listM[1].trim()) }
  const mapM = t.match(/^Map<\s*([^,]+)\s*,\s*(.+)\s*>$/)
  if (mapM) return { type: 'object', additionalProperties: typeToSchema(mapM[2].trim()) }
  if (t.startsWith('OneOf<')) return {}
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
  const m = scl.models[t]
  if (!m) return { $ref: `#/components/schemas/${t}` }
  if (m.kind === 'enum') return { type: 'string', enum: m.values.map(toWire) }
  return { $ref: `#/components/schemas/${t}` }
}

function fieldToSchema(f: Field): any {
  return applyConstraints(typeToSchema(f.type), f)
}

function modelToSchema(name: string, m: Model): any {
  switch (m.kind) {
    case 'enum':
      return { type: 'string', enum: m.values.map(toWire) }
    case 'entity':
    case 'value_object': {
      const properties: Record<string, any> = {}
      const required: string[] = []
      for (const [fn, fd] of Object.entries(m.fields)) {
        properties[fn] = fieldToSchema(fd)
        if (!fd.optional) required.push(fn)
      }
      return {
        type: 'object',
        additionalProperties: false,
        ...(m.description ? { description: m.description } : {}),
        properties,
        ...(required.length ? { required } : {}),
      }
    }
    case 'event':
    case 'error': {
      const properties: Record<string, any> = { type: { const: name, type: 'string' } }
      const required = ['type']
      if (m.payload) {
        for (const [fn, fd] of Object.entries(m.payload)) {
          properties[fn] = fieldToSchema(fd)
          if (!fd.optional) required.push(fn)
        }
      }
      return { type: 'object', properties, required }
    }
  }
}

// ===============================================================
// JSON Schemas to gen/*.json
// ===============================================================

async function emitJsonSchemas(outDir: string) {
  await mkdir(outDir, { recursive: true })
  // 旧 .json は一掃する（drift 防止）
  for (const f of await readdir(outDir).catch(() => [] as string[])) {
    if (f.endsWith('.json')) await unlink(join(outDir, f)).catch(() => {})
  }

  const written: string[] = []
  for (const [name, m] of Object.entries(scl.models)) {
    if (m.kind === 'enum') continue
    const schema = {
      $id: `urn:idp:${name}`,
      ...modelToSchema(name, m),
    }
    const path = join(outDir, `${name}.json`)
    await writeFile(path, JSON.stringify(schema, null, 2) + '\n')
    written.push(name)
  }

  // DomainEvent: oneOf of all event schemas
  const events = Object.entries(scl.models)
    .filter(([, m]) => m.kind === 'event')
    .map(([n]) => n)
  const domainEvent = {
    $id: 'urn:idp:domain-events',
    oneOf: events.map((n) => ({ $ref: `${n}.json` })),
  }
  await writeFile(join(outDir, 'DomainEvent.json'), JSON.stringify(domainEvent, null, 2) + '\n')
  written.push('DomainEvent')

  return written
}

// ===============================================================
// OpenAPI to gen/openapi.yaml
// ===============================================================

function yamlSerialize(obj: any, indent = 0): string {
  const pad = '  '.repeat(indent)
  if (obj === null || obj === undefined) return 'null'
  if (typeof obj === 'string') {
    if (
      obj === '' ||
      /[:#\-{}[\],&*!|>'"%@`?]|^[\s]|[\s]$|^(true|false|null|yes|no)$|^-?\d/.test(obj)
    ) {
      return JSON.stringify(obj)
    }
    return obj
  }
  if (typeof obj === 'number' || typeof obj === 'boolean') return String(obj)
  if (Array.isArray(obj)) {
    if (obj.length === 0) return '[]'
    return (
      '\n' +
      obj
        .map((v) => {
          if (v === null || typeof v !== 'object' || Array.isArray(v)) {
            return `${pad}- ${yamlSerialize(v, indent + 1)}`
          }
          const inner = yamlSerialize(v, indent + 1).trimStart()
          return `${pad}- ${inner}`
        })
        .join('\n')
    )
  }
  // object
  const keys = Object.keys(obj)
  if (keys.length === 0) return '{}'
  return (
    '\n' +
    keys
      .map((k) => {
        const v = obj[k]
        if (v === null || typeof v !== 'object')
          return `${pad}${k}: ${yamlSerialize(v, indent + 1)}`
        if (Array.isArray(v) && v.length === 0) return `${pad}${k}: []`
        if (!Array.isArray(v) && Object.keys(v).length === 0) return `${pad}${k}: {}`
        return `${pad}${k}:${yamlSerialize(v, indent + 1)}`
      })
      .join('\n')
  )
}

function inputBodyType(iface: Interface): string | null {
  if (!iface.input) return null
  const keys = Object.keys(iface.input)
  if (keys.length === 1) return iface.input[keys[0]].type
  return null
}

function outputBodyType(iface: Interface): string | null {
  if (!iface.output) return null
  const keys = Object.keys(iface.output)
  if (keys.length === 1) return iface.output[keys[0]].type
  return null
}

async function emitOpenApi(outPath: string) {
  const paths: Record<string, any> = {}
  for (const [name, iface] of Object.entries(scl.interfaces)) {
    const http = httpBinding(iface)
    if (!http?.method || !http.path) continue
    const method = http.method.toLowerCase()
    const path = http.path
    paths[path] ??= {}
    const op: any = {
      operationId: name[0].toLowerCase() + name.slice(1),
      summary: iface.description ?? '',
      responses: {},
    }
    const inputT = inputBodyType(iface)
    if (inputT) {
      op.requestBody = {
        required: true,
        content: { 'application/json': { schema: typeToSchema(inputT) } },
      }
    }
    const respCode = http.successful_status_codes?.[0] ?? '200'
    const outputT = outputBodyType(iface)
    op.responses[respCode] = {
      description: iface.description ?? 'OK',
      ...(outputT ? { content: { 'application/json': { schema: typeToSchema(outputT) } } } : {}),
    }
    if (iface.errors?.length) {
      op.responses['400'] = {
        description: 'Error',
        content: { 'application/json': { schema: { $ref: '#/components/schemas/OAuthError' } } },
      }
    }
    paths[path][method] = op
  }

  const schemas: Record<string, any> = {}
  for (const [name, m] of Object.entries(scl.models)) {
    schemas[name] = modelToSchema(name, m)
  }

  const openapi = {
    openapi: '3.0.3',
    info: {
      title: 'OAuth2 / OIDC IdP',
      version: scl.spec_version,
    },
    paths,
    components: { schemas },
  }

  // 簡易 YAML（OpenAPI 既知の構造を出すための最低限の serializer）
  const yaml = `openapi: ${openapi.openapi}\ninfo:\n  title: ${JSON.stringify(openapi.info.title)}\n  version: "${openapi.info.version}"\npaths:${yamlSerialize(openapi.paths, 1)}\ncomponents:\n  schemas:${yamlSerialize(openapi.components.schemas, 2)}\n`
  await writeFile(outPath, yaml)
}

// ===============================================================
// Discovery to gen/discovery.json
// ===============================================================

async function emitDiscoveryTemplate(outPath: string) {
  const tpl = (scl as any).annotations?.discovery_template ?? {}
  const doc: Record<string, unknown> = {
    issuer: '{{ISSUER}}',
  }
  // collect endpoint paths
  for (const [field, ifaceName] of Object.entries({
    authorization_endpoint: 'Authorize',
    token_endpoint: 'Token',
    userinfo_endpoint: 'UserInfo',
    jwks_uri: 'GetJwks',
    introspection_endpoint: 'Introspect',
    revocation_endpoint: 'Revoke',
    pushed_authorization_request_endpoint: 'PushAuthorizationRequest',
    device_authorization_endpoint: 'DeviceAuthorization',
    registration_endpoint: 'RegisterClient',
  })) {
    const iface = scl.interfaces[ifaceName]
    const p = iface ? httpBinding(iface)?.path : undefined
    if (p) doc[field] = `{{ISSUER}}${p}`
  }
  doc.end_session_endpoint = `{{ISSUER}}/end_session`
  doc.scopes_supported = tpl.scopes_supported ?? []
  doc.response_types_supported = (scl.models.ResponseType as any).values.map(toWire)
  doc.response_modes_supported = (scl.models.ResponseMode as any).values.map(toWire)
  doc.grant_types_supported = (scl.models.GrantType as any).values.map(toWire)
  doc.subject_types_supported = tpl.subject_types_supported ?? ['public']
  doc.id_token_signing_alg_values_supported = (scl.models.SignatureAlgorithm as any).values.map(
    toWire,
  )
  doc.token_endpoint_auth_methods_supported = (scl.models.TokenEndpointAuthMethod as any).values
    .map(toWire)
    .filter((m: string) => m !== 'none')
  doc.code_challenge_methods_supported = (scl.models.CodeChallengeMethod as any).values.map(toWire)
  doc.require_pushed_authorization_requests = false
  doc.require_pkce = true
  doc.dpop_signing_alg_values_supported = (scl.models.SignatureAlgorithm as any).values.map(toWire)
  doc.tls_client_certificate_bound_access_tokens = true
  doc.claims_supported = tpl.claims_supported ?? []
  doc.ui_locales_supported = tpl.ui_locales_supported ?? ['en', 'ja']

  await writeFile(outPath, JSON.stringify(doc, null, 2) + '\n')
}

// ===============================================================
// Main
// ===============================================================

async function main() {
  const genDir = join(import.meta.dir, '../../gen')
  await mkdir(genDir, { recursive: true })

  const written = await emitJsonSchemas(genDir)
  // eslint-disable-next-line no-console
  console.log(`Wrote ${written.length} JSON Schemas → gen/`)

  await emitOpenApi(join(genDir, 'openapi.yaml'))
  // eslint-disable-next-line no-console
  console.log(`Wrote gen/openapi.yaml`)

  await emitDiscoveryTemplate(join(genDir, 'discovery.json'))
  // eslint-disable-next-line no-console
  console.log(`Wrote gen/discovery.json`)
}

main().catch((e) => {
  console.error(e)
  process.exit(1)
})

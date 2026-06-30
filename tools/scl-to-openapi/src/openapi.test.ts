import { resolve } from 'node:path'
import Ajv2020 from 'ajv/dist/2020.js'
import addFormats from 'ajv-formats'
import { describe, expect, it } from 'bun:test'
import { loadSclBundle } from '../../scl-to-html/src/load.ts'
import type { SclDocument } from '../../scl-to-html/src/types.ts'
import {
  collectRefNames,
  type JsonSchema,
  missingRefs,
} from '../../scl-to-jsonschema/src/generate.ts'
import { generateOpenApi } from './openapi.ts'

const newAjv = () => {
  const ajv = new Ajv2020({ allErrors: true, strict: false })
  addFormats.default(ajv)
  return ajv
}

const op = (doc: JsonSchema, path: string, method: string): Record<string, unknown> => {
  const paths = doc.paths as Record<string, Record<string, unknown>>
  const item = paths[path]
  if (!item) throw new Error(`no path ${path}`)
  const o = item[method]
  if (!o || typeof o !== 'object') throw new Error(`no ${method} ${path}`)
  return o as Record<string, unknown>
}

const doc = (
  models: SclDocument['models'],
  interfaces: SclDocument['interfaces'],
): SclDocument => ({ system: 'demo', spec_version: '2.0', models, interfaces })

describe('generateOpenApi — unit', () => {
  it('turns an http interface into an operation with a json request body', () => {
    const out = generateOpenApi(
      doc(
        { Req: { kind: 'value_object', fields: { a: { type: 'String' } } } },
        {
          DoThing: {
            description: 'Do a thing.\nmore detail',
            input: { request: { type: 'Req' } },
            bindings: [{ kind: 'http', method: 'POST', path: '/things', request_form: 'body' }],
          },
        },
      ),
    )
    const o = op(out, '/things', 'post')
    expect(o.operationId).toBe('DoThing')
    expect(o.summary).toBe('Do a thing.')
    const body = o.requestBody as { content: Record<string, { schema: Record<string, unknown> }> }
    const schema = body.content['application/json']?.schema as Record<string, unknown>
    // request schema references the model under components/schemas
    const props = schema.properties as Record<string, unknown>
    expect(props.request).toEqual({ $ref: '#/components/schemas/Req' })
  })

  it('maps query request_form to query parameters and path tokens to path params', () => {
    const out = generateOpenApi(
      doc(undefined, {
        Get: {
          input: { q: { type: 'String' }, opt: { type: 'String', optional: true } },
          bindings: [{ kind: 'http', method: 'GET', path: '/t/{id}', request_form: 'query' }],
        },
      }),
    )
    const o = op(out, '/t/{id}', 'get')
    const params = o.parameters as Array<Record<string, unknown>>
    expect(params).toContainEqual({
      name: 'id',
      in: 'path',
      required: true,
      schema: { type: 'string' },
    })
    expect(params).toContainEqual({
      name: 'q',
      in: 'query',
      required: true,
      schema: { type: 'string' },
    })
    expect(params).toContainEqual({
      name: 'opt',
      in: 'query',
      required: false,
      schema: { type: 'string' },
    })
  })

  it('emits success status codes and an error default response', () => {
    const out = generateOpenApi(
      doc(
        { E: { kind: 'error' }, Resp: { kind: 'value_object', fields: { x: { type: 'String' } } } },
        {
          Op: {
            output: { response: { type: 'Resp' } },
            errors: ['E'],
            bindings: [
              {
                kind: 'http',
                method: 'POST',
                path: '/op',
                successful_status_codes: ['201'],
              },
            ],
          },
        },
      ),
    )
    const o = op(out, '/op', 'post')
    const responses = o.responses as Record<string, Record<string, unknown>>
    expect(responses['201']).toBeDefined()
    expect(responses.default?.description).toContain('E')
  })
})

describe('generateOpenApi — ra-idp-go conformance', () => {
  it('produces a 3.1 document whose refs all resolve to components/schemas', async () => {
    const sclPath = resolve(import.meta.dir, '../../../ra-idp-go/spec/scl.yaml')
    const bundle = await loadSclBundle(sclPath)
    const out = generateOpenApi(bundle)

    expect(out.openapi).toBe('3.1.0')
    const paths = out.paths as Record<string, unknown>
    expect(Object.keys(paths).length).toBeGreaterThan(0)

    const components = out.components as { schemas: Record<string, unknown> }
    const known = new Set(Object.keys(components.schemas))
    // Every $ref in the whole document resolves, and none still point at $defs.
    expect(missingRefs(out, known, '#/components/schemas/')).toEqual([])
    expect(collectRefNames(out, '#/$defs/')).toEqual([])

    // Each component schema is a valid JSON Schema 2020-12 (OpenAPI 3.1 uses it).
    const ajv = newAjv()
    expect(() => ajv.compile({ $defs: components.schemas })).not.toThrow()
  })
})

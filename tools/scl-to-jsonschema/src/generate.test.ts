import { resolve } from 'node:path'
import Ajv2020 from 'ajv/dist/2020.js'
import addFormats from 'ajv-formats'
import { describe, expect, it } from 'bun:test'
import { loadSclBundle } from '../../scl-to-html/src/load.ts'
import type { SclDocument } from '../../scl-to-html/src/types.ts'
import { danglingRefs, generateModelSchemas, type JsonSchema } from './generate.ts'

const newAjv = () => {
  const ajv = new Ajv2020({ allErrors: true, strict: false })
  addFormats.default(ajv)
  return ajv
}

const defOf = (schema: JsonSchema, name: string): Record<string, unknown> => {
  const defs = schema.$defs as Record<string, unknown>
  const d = defs[name]
  if (!d || typeof d !== 'object') throw new Error(`missing $def: ${name}`)
  return d as Record<string, unknown>
}

const doc = (models: SclDocument['models']): SclDocument => ({
  system: 'demo',
  spec_version: '2.0',
  models,
})

describe('generateModelSchemas — unit', () => {
  it('maps an enum to a string enum', () => {
    const out = generateModelSchemas(doc({ Color: { kind: 'enum', values: ['Red', 'Blue'] } }))
    expect(defOf(out, 'Color')).toEqual({ type: 'string', enum: ['Red', 'Blue'] })
  })

  it('maps an entity, marking non-optional fields required and resolving refs', () => {
    const out = generateModelSchemas(
      doc({
        Color: { kind: 'enum', values: ['Red'] },
        Thing: {
          kind: 'entity',
          identity: 'id',
          fields: {
            id: { type: 'String', constraints: ['non_empty', { max_length: 10 }] },
            color: { type: 'Color' },
            note: { type: 'String', optional: true },
          },
        },
      }),
    )
    const thing = defOf(out, 'Thing')
    expect(thing.type).toBe('object')
    expect(thing.additionalProperties).toBe(false)
    expect(thing.required).toEqual(['id', 'color'])
    const props = thing.properties as Record<string, Record<string, unknown>>
    expect(props.id).toMatchObject({ type: 'string', minLength: 1, maxLength: 10 })
    expect(props.color).toEqual({ $ref: '#/$defs/Color' })
  })

  it('maps Name[] to an array of items', () => {
    const out = generateModelSchemas(
      doc({
        Tag: { kind: 'value_object', fields: { v: { type: 'String' } } },
        Bag: { kind: 'value_object', fields: { tags: { type: 'Tag[]' } } },
      }),
    )
    const bag = defOf(out, 'Bag')
    const props = bag.properties as Record<string, unknown>
    expect(props.tags).toEqual({ type: 'array', items: { $ref: '#/$defs/Tag' } })
  })

  it('produces a schema with no dangling $defs references', () => {
    const out = generateModelSchemas(
      doc({
        A: { kind: 'value_object', fields: { b: { type: 'B' } } },
        B: { kind: 'value_object', fields: { x: { type: 'String' } } },
      }),
    )
    expect(danglingRefs(out)).toEqual([])
  })

  it('generated enum schema actually constrains values', () => {
    const out = generateModelSchemas(doc({ Color: { kind: 'enum', values: ['Red', 'Blue'] } }))
    const validate = newAjv().compile({ ...out, $ref: '#/$defs/Color' })
    expect(validate('Red')).toBe(true)
    expect(validate('Green')).toBe(false)
  })

  it('generated entity schema enforces required fields', () => {
    const out = generateModelSchemas(
      doc({
        Thing: { kind: 'entity', identity: 'id', fields: { id: { type: 'String' } } },
      }),
    )
    const validate = newAjv().compile({ ...out, $ref: '#/$defs/Thing' })
    expect(validate({ id: 'x' })).toBe(true)
    expect(validate({})).toBe(false)
  })
})

describe('generateModelSchemas — ra-idp-go conformance', () => {
  it('compiles as valid JSON Schema 2020-12 with no dangling refs', async () => {
    const sclPath = resolve(import.meta.dir, '../../../ra-idp-go/spec/scl.yaml')
    const bundle = await loadSclBundle(sclPath)
    const schema = generateModelSchemas(bundle)

    const defs = schema.$defs as Record<string, unknown>
    expect(Object.keys(defs).length).toBeGreaterThan(0)
    expect(danglingRefs(schema)).toEqual([])

    // Ajv compiling the whole document proves it is structurally a valid schema.
    expect(() => newAjv().compile(schema)).not.toThrow()
  })
})

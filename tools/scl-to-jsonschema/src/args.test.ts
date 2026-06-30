import { describe, expect, it } from 'bun:test'
import { parseCliArgs } from './args.ts'

describe('parseCliArgs', () => {
  it('parses --scl and --out (space form)', () => {
    const r = parseCliArgs(['--scl', 'a.yaml', '--out', 'a.json'])
    expect(r).toEqual({ kind: 'ok', opts: { scl: 'a.yaml', out: 'a.json' } })
  })

  it('parses --flag=value form', () => {
    const r = parseCliArgs(['--scl=a.yaml'])
    expect(r).toMatchObject({ kind: 'ok', opts: { scl: 'a.yaml', out: null } })
  })

  it('returns help for --help / -h', () => {
    expect(parseCliArgs(['--help']).kind).toBe('help')
    expect(parseCliArgs(['-h']).kind).toBe('help')
  })

  it('errors when --scl is missing', () => {
    const r = parseCliArgs([])
    expect(r.kind).toBe('error')
    if (r.kind === 'error') expect(r.code).toBe(2)
  })

  it('errors when a value flag has no value', () => {
    const r = parseCliArgs(['--scl'])
    expect(r).toMatchObject({ kind: 'error', code: 2 })
  })

  it('errors on an unknown flag', () => {
    const r = parseCliArgs(['--scl', 'a.yaml', '--nope'])
    expect(r.kind).toBe('error')
  })
})

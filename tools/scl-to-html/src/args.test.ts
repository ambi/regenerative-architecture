import { describe, expect, it } from 'bun:test'
import { parseCliArgs } from './args.ts'

describe('parseCliArgs', () => {
  it('requires --scl', () => {
    const r = parseCliArgs([])
    expect(r.kind).toBe('error')
    if (r.kind === 'error') expect(r.code).toBe(2)
  })

  it('accepts --scl <path> in space form', () => {
    const r = parseCliArgs(['--scl', 'spec.yaml'])
    expect(r.kind).toBe('ok')
    if (r.kind === 'ok') expect(r.opts.scl).toBe('spec.yaml')
  })

  it('accepts --scl=path in equals form', () => {
    const r = parseCliArgs(['--scl=spec.yaml'])
    expect(r.kind).toBe('ok')
    if (r.kind === 'ok') expect(r.opts.scl).toBe('spec.yaml')
  })

  it('accepts all optional flags together', () => {
    const r = parseCliArgs([
      '--scl=a.yaml',
      '--decisions=adr/',
      '--work-items=work-items/',
      '--title=demo',
      '--out=out.html',
    ])
    expect(r.kind).toBe('ok')
    if (r.kind === 'ok') {
      expect(r.opts).toEqual({
        scl: 'a.yaml',
        decisions: 'adr/',
        workItems: 'work-items/',
        title: 'demo',
        out: 'out.html',
      })
    }
  })

  it('accepts --changes as a compatibility alias', () => {
    const r = parseCliArgs(['--scl=a.yaml', '--changes=old/'])
    expect(r.kind).toBe('ok')
    if (r.kind === 'ok') expect(r.opts.workItems).toBe('old/')
  })

  it('returns help when --help is given', () => {
    expect(parseCliArgs(['--help']).kind).toBe('help')
    expect(parseCliArgs(['-h']).kind).toBe('help')
  })

  it('errors when a value flag is the final argument', () => {
    const r = parseCliArgs(['--scl', 'a.yaml', '--out'])
    expect(r.kind).toBe('error')
    if (r.kind === 'error') expect(r.message).toContain('--out requires a value')
  })

  it('errors on unknown flag', () => {
    const r = parseCliArgs(['--scl=a.yaml', '--what'])
    expect(r.kind).toBe('error')
    if (r.kind === 'error') expect(r.message).toContain('unknown flag')
  })

  it('errors on positional arguments', () => {
    const r = parseCliArgs(['file.yaml'])
    expect(r.kind).toBe('error')
    if (r.kind === 'error') expect(r.message).toContain('unexpected positional')
  })
})

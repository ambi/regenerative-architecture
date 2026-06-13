/**
 * Layer 4 — Adapter Layer (HIBP BreachedPasswordChecker テスト)
 *
 * k-anonymity プロトコル契約: prefix 5 文字のみを送信する / suffix 35 文字を
 * 大文字 hex で比較する / count > 0 のみを breached と扱う / 障害時は
 * fail-open する。詳細は ADR-028。
 */

import { describe, expect, it } from 'bun:test'
import { createHash } from 'crypto'

import { HibpBreachedPasswordChecker } from './hibp-breached-password-checker'

function sha1Hex(plain: string): string {
  return createHash('sha1').update(plain, 'utf8').digest('hex').toUpperCase()
}

function fakeFetch(handler: (url: string, init?: RequestInit) => Response): typeof fetch {
  return (async (input: Request | string | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input.toString()
    return handler(url, init)
  }) as typeof fetch
}

describe('HibpBreachedPasswordChecker', () => {
  it('SHA-1 prefix 5 文字だけを URL に乗せ、Add-Padding ヘッダを付与する', async () => {
    const calls: { url: string; headers: Headers }[] = []
    const plain = 'P@ssw0rd-correct-horse'
    const expectedPrefix = sha1Hex(plain).slice(0, 5)

    const checker = new HibpBreachedPasswordChecker({
      endpoint: 'https://api.test/range',
      fetchImpl: fakeFetch((url, init) => {
        calls.push({ url, headers: new Headers(init?.headers) })
        return new Response('AAAAA:1\r\n', { status: 200 })
      }),
    })
    await checker.isBreached(plain)

    expect(calls).toHaveLength(1)
    expect(calls[0].url).toBe(`https://api.test/range/${expectedPrefix}`)
    expect(calls[0].headers.get('Add-Padding')).toBe('true')
    // 生パスワード / 完全な SHA-1 は URL に乗ってはならない。
    expect(calls[0].url).not.toContain(plain)
    expect(calls[0].url).not.toContain(sha1Hex(plain))
  })

  it('suffix が count > 0 で一致する行があれば breached', async () => {
    const plain = 'leaked-secret-1234'
    const sha1 = sha1Hex(plain)
    const suffix = sha1.slice(5)
    const body = ['00000000000000000000000000000000000:9', `${suffix}:42`, 'FFFFFFFFFFF:0'].join(
      '\r\n',
    )

    const checker = new HibpBreachedPasswordChecker({
      endpoint: 'https://api.test/range',
      fetchImpl: fakeFetch(() => new Response(body, { status: 200 })),
    })
    expect(await checker.isBreached(plain)).toBe(true)
  })

  it('レスポンスが小文字 suffix でも大文字に正規化して比較する', async () => {
    const plain = 'mixed-case-suffix-7777'
    const suffix = sha1Hex(plain).slice(5)
    const body = `${suffix.toLowerCase()}:3\r\n`
    const checker = new HibpBreachedPasswordChecker({
      endpoint: 'https://api.test/range',
      fetchImpl: fakeFetch(() => new Response(body, { status: 200 })),
    })
    expect(await checker.isBreached(plain)).toBe(true)
  })

  it('suffix 一致が count=0 のときは breached ではない (padding 行を弾く)', async () => {
    const plain = 'fresh-unique-passphrase'
    const suffix = sha1Hex(plain).slice(5)
    const body = `${suffix}:0\r\n`
    const checker = new HibpBreachedPasswordChecker({
      endpoint: 'https://api.test/range',
      fetchImpl: fakeFetch(() => new Response(body, { status: 200 })),
    })
    expect(await checker.isBreached(plain)).toBe(false)
  })

  it('suffix が無ければ breached ではない', async () => {
    const checker = new HibpBreachedPasswordChecker({
      endpoint: 'https://api.test/range',
      fetchImpl: fakeFetch(() => new Response('AAAAA:1\r\nBBBBB:2\r\n', { status: 200 })),
    })
    expect(await checker.isBreached('truly-unique-passphrase')).toBe(false)
  })

  it('HTTP error は fail-open (false)', async () => {
    const checker = new HibpBreachedPasswordChecker({
      endpoint: 'https://api.test/range',
      fetchImpl: fakeFetch(() => new Response('boom', { status: 500 })),
    })
    expect(await checker.isBreached('any-password-please')).toBe(false)
  })

  it('fetch 例外も fail-open (false)', async () => {
    const checker = new HibpBreachedPasswordChecker({
      endpoint: 'https://api.test/range',
      fetchImpl: fakeFetch(() => {
        throw new Error('network down')
      }),
    })
    expect(await checker.isBreached('any-password-please')).toBe(false)
  })
})

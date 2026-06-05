/**
 * Layer 3 — Application Logic（ドメイン単体テスト）
 *
 * リフレッシュトークンのローテーション・ファミリー継承の不変条件テスト。
 * ADR-004 で定義した「ローテーション越境不可」「ファミリー一括失効」を検証する。
 */

import { describe, it, expect } from 'bun:test'
import { generateInitial, rotate, hashToken, isAbsoluteExpired, isReplay } from './refresh-token'

describe('generateInitial', () => {
  it('新しいトークンは未失効・未ローテーションで発行される', () => {
    const { token, record } = generateInitial({
      client_id: 'c',
      sub: 'u',
      scopes: ['openid'],
    })
    expect(token.length).toBeGreaterThan(40)
    expect(record.revoked).toBe(false)
    expect(record.rotated).toBe(false)
    expect(record.parent_id).toBeUndefined()
    expect(record.hash).toBe(hashToken(token))
  })

  it('absolute_expires_at は expires_at 以上である', () => {
    const { record } = generateInitial({ client_id: 'c', sub: 'u', scopes: [] })
    expect(Date.parse(record.absolute_expires_at)).toBeGreaterThanOrEqual(
      Date.parse(record.expires_at),
    )
  })
})

describe('rotate', () => {
  it('子トークンは親と同じ family_id を持ち、parent_id でリンクされる', () => {
    const parent = generateInitial({ client_id: 'c', sub: 'u', scopes: ['openid'] })
    const child = rotate(parent.record)
    expect(child.record.family_id).toBe(parent.record.family_id)
    expect(child.record.parent_id).toBe(parent.record.id)
  })

  it('absolute_expires_at は親から継承される（ローテーションで延長されない）', () => {
    const parent = generateInitial({ client_id: 'c', sub: 'u', scopes: [] })
    // 親を遠い未来に擬装したくないので、親そのものから継承を確認
    const child = rotate(parent.record)
    expect(child.record.absolute_expires_at).toBe(parent.record.absolute_expires_at)
  })

  it('子の expires_at は親の absolute_expires_at を超えない', () => {
    const parent = generateInitial({ client_id: 'c', sub: 'u', scopes: [] })
    // 親の absolute_expires_at の手前にローテーション時刻を設定
    const nearAbsoluteExpiry = new Date(Date.parse(parent.record.absolute_expires_at) - 1000)
    const child = rotate(parent.record, nearAbsoluteExpiry)
    expect(Date.parse(child.record.expires_at)).toBeLessThanOrEqual(
      Date.parse(parent.record.absolute_expires_at),
    )
  })

  it('sender_constraint は親から継承される', () => {
    const parent = generateInitial({
      client_id: 'c',
      sub: 'u',
      scopes: [],
      sender_constraint: { type: 'dpop', jkt: 'abc' },
    })
    const child = rotate(parent.record)
    expect(child.record.sender_constraint).toEqual({ type: 'dpop', jkt: 'abc' })
  })

  it('子は親と異なるトークン文字列を発行する', () => {
    const parent = generateInitial({ client_id: 'c', sub: 'u', scopes: [] })
    const child = rotate(parent.record)
    expect(child.token).not.toBe(parent.token)
    expect(child.record.hash).not.toBe(parent.record.hash)
  })
})

describe('isReplay / isAbsoluteExpired', () => {
  it('rotated フラグが立った record はリプレイとみなす', () => {
    const parent = generateInitial({ client_id: 'c', sub: 'u', scopes: [] })
    const rotatedParent = { ...parent.record, rotated: true }
    expect(isReplay(rotatedParent)).toBe(true)
  })

  it('revoked フラグが立った record はリプレイとみなす', () => {
    const parent = generateInitial({ client_id: 'c', sub: 'u', scopes: [] })
    const revoked = { ...parent.record, revoked: true }
    expect(isReplay(revoked)).toBe(true)
  })

  it('absolute_expires_at を過ぎたら期限切れ', () => {
    const r = generateInitial({ client_id: 'c', sub: 'u', scopes: [] })
    const futureNow = new Date(Date.parse(r.record.absolute_expires_at) + 1000)
    expect(isAbsoluteExpired(r.record, futureNow)).toBe(true)
  })

  it('absolute_expires_at の手前ではまだ有効', () => {
    const r = generateInitial({ client_id: 'c', sub: 'u', scopes: [] })
    const beforeExpiry = new Date(Date.parse(r.record.absolute_expires_at) - 1000)
    expect(isAbsoluteExpired(r.record, beforeExpiry)).toBe(false)
  })
})

describe('hashToken', () => {
  it('同じトークンは同じハッシュを返す（決定論的）', () => {
    expect(hashToken('xyz')).toBe(hashToken('xyz'))
  })

  it('異なるトークンは異なるハッシュを返す', () => {
    expect(hashToken('abc')).not.toBe(hashToken('def'))
  })
})

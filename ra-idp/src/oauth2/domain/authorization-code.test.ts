/**
 * Layer 3 — Application Logic（ドメイン単体テスト）
 *
 * 認可コードの不変条件:
 * - TTL は仕様核（SLO）で定めた 60 秒以内
 * - 単一使用（一度 redeemed にしたら再度 redeem 不可）
 */

import { describe, it, expect } from 'bun:test'
import {
  generateAuthorizationCode,
  isExpired,
  isRedeemed,
  markRedeemed,
} from './authorization-code'
import { OAuthError } from '../protocol/oauth-error'

const baseInput = {
  authorization_request_id: '00000000-0000-0000-0000-000000000001',
  client_id: 'cli',
  sub: 'user_alice',
  scopes: ['openid'],
  redirect_uri: 'https://app.example.com/cb',
  code_challenge: 'abc123',
  code_challenge_method: 'S256' as const,
  auth_time: 1700000000,
}

describe('generateAuthorizationCode', () => {
  it('デフォルトの TTL は 60 秒以下である（RFC 9700 §4.10 推奨上限）', () => {
    const now = new Date()
    const code = generateAuthorizationCode({ ...baseInput, now })
    const ttl = Date.parse(code.expires_at) - now.getTime()
    expect(ttl).toBeLessThanOrEqual(60_000)
  })

  it('発行直後は未使用である', () => {
    const code = generateAuthorizationCode(baseInput)
    expect(isRedeemed(code)).toBe(false)
  })

  it('TTL を超えた時刻では isExpired が true', () => {
    const now = new Date()
    const code = generateAuthorizationCode({ ...baseInput, ttl_seconds: 60, now })
    expect(isExpired(code, new Date(now.getTime() + 61_000))).toBe(true)
  })
})

describe('markRedeemed', () => {
  it('redeemed_at をセットして返す', () => {
    const code = generateAuthorizationCode(baseInput)
    const redeemed = markRedeemed(code)
    expect(redeemed.redeemed_at).toBeDefined()
    expect(isRedeemed(redeemed)).toBe(true)
  })

  it('すでに使用済みのコードを再度 redeem しようとすると invalid_grant', () => {
    const code = generateAuthorizationCode(baseInput)
    const once = markRedeemed(code)
    expect(() => markRedeemed(once)).toThrow(OAuthError)
  })
})

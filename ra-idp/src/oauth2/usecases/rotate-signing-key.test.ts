/**
 * 署名鍵ローテーション usecase (ADR-009) の単体テスト。
 *
 * 永続化非依存なので InMemoryKeyStore で検証する (PostgresKeyStore は
 * persistence-contract.test.ts の KeyStore 契約で同じ不変条件を検証する)。
 *
 * ADR-009 の要点:
 *   - 回転後、新鍵が active になる
 *   - 旧鍵は JWKS / findByKid に残る (検証オーバーラップ — 旧鍵署名トークンを検証可能)
 *   - SigningKeyRotated 監査イベントが newKid / previousKid 付きで発行される
 */

import { describe, test, expect } from 'bun:test'
import { InMemoryKeyStore } from '../../../adapters/crypto/in-memory-key-store'
import { rotateSigningKeyUseCase } from './rotate-signing-key'
import type { DomainEvent } from '../../spec-bindings/schemas'

describe('rotateSigningKeyUseCase', () => {
  test('回転で新 active 鍵を作り、旧鍵を検証用に残す', async () => {
    const keyStore = await InMemoryKeyStore.create('PS256')
    const before = await keyStore.getActiveKey()

    const events: DomainEvent[] = []
    const result = await rotateSigningKeyUseCase({ keyStore }, (e) => events.push(e))

    // 新 active 鍵は旧鍵と別物
    const after = await keyStore.getActiveKey()
    expect(after.kid).toBe(result.newKid)
    expect(after.kid).not.toBe(before.kid)
    expect(result.previousKid).toBe(before.kid)

    // 旧鍵は findByKid / getAllKeys に残る (オーバーラップ)
    const oldStill = await keyStore.findByKid(before.kid)
    expect(oldStill).not.toBeNull()
    expect(oldStill?.active).toBe(false)
    const all = await keyStore.getAllKeys()
    expect(all.length).toBeGreaterThanOrEqual(2)
  })

  test('SigningKeyRotated イベントを newKid / previousKid 付きで発行する', async () => {
    const keyStore = await InMemoryKeyStore.create('PS256')
    const before = await keyStore.getActiveKey()

    const events: DomainEvent[] = []
    await rotateSigningKeyUseCase({ keyStore }, (e) => events.push(e))

    const rotated = events.find((e) => e.type === 'SigningKeyRotated')
    expect(rotated).toBeDefined()
    if (rotated && rotated.type === 'SigningKeyRotated') {
      expect(rotated.previousKid).toBe(before.kid)
      expect(typeof rotated.newKid).toBe('string')
      expect(rotated.newKid).not.toBe(before.kid)
    }
  })

  test('全鍵が PS256 / ES256 のみ (ADR-003 と整合)', async () => {
    const keyStore = await InMemoryKeyStore.create('PS256')
    await rotateSigningKeyUseCase({ keyStore }, () => {})
    const all = await keyStore.getAllKeys()
    for (const k of all) {
      expect(['PS256', 'ES256']).toContain(k.alg)
    }
  })
})

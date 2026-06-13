/**
 * Layer 3 — Application Logic (LoginSessionManager.resolve disabled-user ガード)
 *
 * ADR-031: userRepo を注入した resolve は、user の disabled_at をチェックして
 * 既存 session を失効させる。
 */

import { describe, expect, it } from 'bun:test'

import { InMemorySessionStore } from '../../../adapters/persistence/memory/session-store'
import { InMemoryUserRepository } from '../../../adapters/persistence/memory/user-repo'
import { UserSchema } from '../../spec-bindings/schemas'

import { LoginSessionManager, SESSION_COOKIE } from './session-manager'

async function seedActiveAlice(userRepo: InMemoryUserRepository): Promise<void> {
  await userRepo.save(
    UserSchema.parse({
      sub: 'user-alice',
      preferred_username: 'alice',
      password_hash: 'argon-placeholder',
      email_verified: true,
      mfa_enrolled: false,
      created_at: '2024-01-01T00:00:00.000Z',
      updated_at: '2024-01-01T00:00:00.000Z',
    }),
  )
}

function cookieHeaders(sessionId: string): Headers {
  return new Headers({ cookie: `${SESSION_COOKIE}=${sessionId}` })
}

describe('LoginSessionManager.resolve (ADR-031)', () => {
  it('userRepo 未注入なら従来どおり session のみ検証する', async () => {
    const sessionStore = new InMemorySessionStore()
    const sm = new LoginSessionManager(sessionStore)
    const ctx = await sm.create('user-alice', ['pwd'])
    const resolved = await sm.resolve(cookieHeaders(ctx.session_id!))
    expect(resolved?.sub).toBe('user-alice')
  })

  it('userRepo 注入時、disabled user の session は失効させ null を返す', async () => {
    const sessionStore = new InMemorySessionStore()
    const userRepo = new InMemoryUserRepository()
    await seedActiveAlice(userRepo)
    const sm = new LoginSessionManager(sessionStore, userRepo)

    const ctx = await sm.create('user-alice', ['pwd'])

    // セッション後に管理者が無効化したケースを模倣
    const alice = await userRepo.findBySub('user-alice')
    if (!alice) throw new Error('seed missing')
    await userRepo.save({ ...alice, disabled_at: '2024-02-01T00:00:00.000Z' })

    const resolved = await sm.resolve(cookieHeaders(ctx.session_id!))
    expect(resolved).toBeNull()
    // 副作用: session も削除されている
    const sessionStillThere = await sessionStore.find(ctx.session_id!)
    expect(sessionStillThere).toBeNull()
  })

  it('user が存在しない sub の session は失効させ null を返す', async () => {
    const sessionStore = new InMemorySessionStore()
    const userRepo = new InMemoryUserRepository()
    const sm = new LoginSessionManager(sessionStore, userRepo)

    const ghostCtx = await sm.create('user-ghost', ['pwd'])
    const resolved = await sm.resolve(cookieHeaders(ghostCtx.session_id!))
    expect(resolved).toBeNull()
    expect(await sessionStore.find(ghostCtx.session_id!)).toBeNull()
  })
})

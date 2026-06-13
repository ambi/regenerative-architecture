/**
 * Layer 3 — Application Logic (administration use cases)
 *
 * 認可境界 (RBAC / CSRF / session) を持たない純粋な use case 層を確認する。
 * HTTP 統合は adapters/http/admin-user-routes.test.ts で別途見る。
 */

import { describe, expect, it } from 'bun:test'

import { Argon2idPasswordHasher } from '../../../adapters/crypto/argon2id-password-hasher'
import { InMemoryPasswordHistoryRepository } from '../../../adapters/persistence/memory/password-history-repo'
import { InMemoryUserRepository } from '../../../adapters/persistence/memory/user-repo'
import { type DomainEvent, UserSchema } from '../../spec-bindings/schemas'

import {
  type AdminUserDeps,
  UsernameConflictError,
  UserNotFoundError,
  createAdminUser,
  setAdminUserDisabled,
  updateAdminUser,
} from './users'

const HASHER = new Argon2idPasswordHasher(8, 1)

async function setup(): Promise<{
  deps: AdminUserDeps
  userRepo: InMemoryUserRepository
  events: DomainEvent[]
}> {
  const userRepo = new InMemoryUserRepository()
  const passwordHistoryRepo = new InMemoryPasswordHistoryRepository()
  const events: DomainEvent[] = []
  await userRepo.save(
    UserSchema.parse({
      sub: 'user-bob',
      tenant_id: 'default',
      preferred_username: 'bob',
      password_hash: await HASHER.hash('bob-password-12345'),
      email: 'bob@example.com',
      email_verified: false,
      mfa_enrolled: false,
      roles: [],
      created_at: '2024-01-01T00:00:00.000Z',
      updated_at: '2024-01-01T00:00:00.000Z',
    }),
  )
  const deps: AdminUserDeps = {
    userRepo,
    passwordHasher: HASHER,
    passwordHistoryRepo,
    emit: (e) => events.push(e),
  }
  return { deps, userRepo, events }
}

describe('createAdminUser', () => {
  it('roles を normalize して保存し UserCreated を emit', async () => {
    const { deps, userRepo, events } = await setup()
    const user = await createAdminUser(deps, {
      actorSub: 'user-admin',
      tenant_id: 'default',
      preferred_username: 'carol',
      password: 'fresh-password-12345',
      email: 'carol@example.com',
      roles: ['support', 'admin', 'support'],
    })
    expect(user.roles).toEqual(['admin', 'support'])
    expect((await userRepo.findByUsername('default', 'carol'))?.sub).toBe(user.sub)
    expect(events.some((e) => e.type === 'UserCreated')).toBe(true)
  })

  it('username 衝突は UsernameConflictError', async () => {
    const { deps } = await setup()
    await expect(
      createAdminUser(deps, {
        actorSub: 'user-admin',
        tenant_id: 'default',
        preferred_username: 'bob',
        password: 'fresh-password-12345',
      }),
    ).rejects.toBeInstanceOf(UsernameConflictError)
  })
})

describe('updateAdminUser', () => {
  it('変更なしなら何も emit しない', async () => {
    const { deps, events } = await setup()
    await updateAdminUser(deps, { actorSub: 'user-admin', sub: 'user-bob' })
    expect(events.filter((e) => e.type === 'UserUpdated')).toHaveLength(0)
  })

  it('email_verified を true に変更すると changedFields に乗る', async () => {
    const { deps, events, userRepo } = await setup()
    await updateAdminUser(deps, {
      actorSub: 'user-admin',
      sub: 'user-bob',
      email_verified: true,
    })
    expect((await userRepo.findBySub('user-bob'))?.email_verified).toBe(true)
    const event = events.find((e) => e.type === 'UserUpdated') as
      | { changedFields: string[] }
      | undefined
    expect(event?.changedFields).toEqual(['email_verified'])
  })

  it('存在しない sub は UserNotFoundError', async () => {
    const { deps } = await setup()
    await expect(
      updateAdminUser(deps, { actorSub: 'user-admin', sub: 'nope', name: 'X' }),
    ).rejects.toBeInstanceOf(UserNotFoundError)
  })
})

describe('setAdminUserDisabled', () => {
  it('disable→enable で UserDisabled / UserEnabled を順に emit', async () => {
    const { deps, events, userRepo } = await setup()
    await setAdminUserDisabled(deps, { actorSub: 'user-admin', sub: 'user-bob', disabled: true })
    expect((await userRepo.findBySub('user-bob'))?.disabled_at).toBeDefined()
    await setAdminUserDisabled(deps, { actorSub: 'user-admin', sub: 'user-bob', disabled: false })
    expect((await userRepo.findBySub('user-bob'))?.disabled_at).toBeUndefined()
    expect(events.map((e) => e.type)).toContain('UserDisabled')
    expect(events.map((e) => e.type)).toContain('UserEnabled')
  })

  it('既に disabled の user を再び disable しても idempotent', async () => {
    const { deps, events, userRepo } = await setup()
    await setAdminUserDisabled(deps, { actorSub: 'user-admin', sub: 'user-bob', disabled: true })
    const before = (await userRepo.findBySub('user-bob'))?.disabled_at
    const eventsBefore = events.length
    await setAdminUserDisabled(deps, { actorSub: 'user-admin', sub: 'user-bob', disabled: true })
    const after = (await userRepo.findBySub('user-bob'))?.disabled_at
    expect(after).toBe(before!)
    expect(events.length).toBe(eventsBefore)
  })
})

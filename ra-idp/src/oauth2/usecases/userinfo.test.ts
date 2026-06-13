/**
 * Layer 3 — Application Logic (UserInfo use case 単体テスト)
 *
 * AuthZEN ポリシー + UserRepository を直接駆動。HTTP / DPoP 統合は
 * adapters/http/userinfo-routes.test.ts 側に閉じる。
 *
 * ADR-031: 無効化された user の UserInfo は invalid_token で拒否する。
 */

import { describe, expect, it } from 'bun:test'

import { InMemoryUserRepository } from '../../../adapters/persistence/memory/user-repo'
import { UserSchema } from '../../spec-bindings/schemas'

import { userInfoUseCase } from './userinfo'

const INPUT = {
  scopes: ['openid', 'profile'],
  sub: 'user_alice',
  active: true,
  client_id: 'web-app',
}

async function seed(repo: InMemoryUserRepository, overrides: Record<string, unknown> = {}) {
  await repo.save(
    UserSchema.parse({
      sub: 'user_alice',
      preferred_username: 'alice',
      password_hash: 'x',
      name: 'Alice',
      email: 'alice@example.com',
      email_verified: true,
      created_at: '2024-01-01T00:00:00.000Z',
      updated_at: '2024-01-01T00:00:00.000Z',
      ...overrides,
    }),
  )
}

describe('userInfoUseCase', () => {
  it('有効な user は profile claims を返す', async () => {
    const userRepo = new InMemoryUserRepository()
    await seed(userRepo)
    const res = await userInfoUseCase({ userRepo }, INPUT)
    expect(res.sub).toBe('user_alice')
    expect(res.name).toBe('Alice')
    expect(res.preferred_username).toBe('alice')
  })

  it('disabled user は invalid_token で拒否される (ADR-031)', async () => {
    const userRepo = new InMemoryUserRepository()
    await seed(userRepo, { disabled_at: '2024-02-01T00:00:00.000Z' })
    await expect(userInfoUseCase({ userRepo }, INPUT)).rejects.toMatchObject({
      code: 'invalid_token',
    })
  })
})

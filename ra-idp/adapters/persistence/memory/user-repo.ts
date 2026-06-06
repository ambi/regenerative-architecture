/**
 * Layer 4 — Adapter Layer (in-memory UserRepository)
 */

import type { User } from '../../../src/spec-bindings/schemas'
import type { UserRepository } from '../../../src/authentication/ports/user-repository'

export class InMemoryUserRepository implements UserRepository {
  private readonly bySub = new Map<string, User>()
  private readonly byUsername = new Map<string, string>()

  async findBySub(sub: string): Promise<User | null> {
    const u = this.bySub.get(sub)
    return u ? { ...u } : null
  }

  async findByUsername(username: string): Promise<User | null> {
    const sub = this.byUsername.get(username)
    return sub ? this.findBySub(sub) : null
  }

  async save(user: User): Promise<void> {
    this.bySub.set(user.sub, { ...user })
    this.byUsername.set(user.preferred_username, user.sub)
  }
}

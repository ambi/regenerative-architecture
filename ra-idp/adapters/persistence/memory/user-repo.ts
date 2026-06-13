/**
 * Layer 4 — Adapter Layer (in-memory UserRepository)
 */

import type { User } from '../../../src/spec-bindings/schemas'
import type { UserRepository } from '../../../src/authentication/ports/user-repository'

export class InMemoryUserRepository implements UserRepository {
  private readonly bySub = new Map<string, User>()
  private readonly byUsername = new Map<string, string>()
  private readonly byEmail = new Map<string, string>()

  async findBySub(sub: string): Promise<User | null> {
    const u = this.bySub.get(sub)
    return u ? { ...u } : null
  }

  async findByUsername(tenant_id: string, username: string): Promise<User | null> {
    const sub = this.byUsername.get(usernameKey(tenant_id, username))
    return sub ? this.findBySub(sub) : null
  }

  async findByEmail(tenant_id: string, email: string): Promise<User | null> {
    const sub = this.byEmail.get(emailKey(tenant_id, email.toLowerCase()))
    return sub ? this.findBySub(sub) : null
  }

  async findAll(tenant_id: string): Promise<User[]> {
    return [...this.bySub.values()]
      .filter((u) => u.tenant_id === tenant_id && !u.deleted_at)
      .map((u) => ({ ...u }))
      .sort((a, b) => a.created_at.localeCompare(b.created_at))
  }

  async save(user: User): Promise<void> {
    this.bySub.set(user.sub, { ...user })
    this.byUsername.set(usernameKey(user.tenant_id, user.preferred_username), user.sub)
    if (user.email) this.byEmail.set(emailKey(user.tenant_id, user.email.toLowerCase()), user.sub)
  }
}

function usernameKey(tenant_id: string, username: string): string {
  return `${tenant_id} ${username}`
}

function emailKey(tenant_id: string, email: string): string {
  return `${tenant_id} ${email}`
}

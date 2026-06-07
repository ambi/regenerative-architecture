/**
 * Layer 4 — Adapter Layer (in-memory MfaFactorRepository)
 */

import type { MfaFactor, MfaFactorType } from '../../../src/spec-bindings/schemas'
import type { MfaFactorRepository } from '../../../src/authentication/ports/mfa-factor-repository'

export class InMemoryMfaFactorRepository implements MfaFactorRepository {
  private readonly store = new Map<string, MfaFactor>()

  private key(sub: string, type: MfaFactorType): string {
    return `${sub}::${type}`
  }

  async listBySub(sub: string): Promise<MfaFactor[]> {
    const prefix = `${sub}::`
    return [...this.store.entries()]
      .filter(([k]) => k.startsWith(prefix))
      .map(([, v]) => ({ ...v }))
  }

  async find(sub: string, type: MfaFactorType): Promise<MfaFactor | null> {
    const f = this.store.get(this.key(sub, type))
    return f ? { ...f } : null
  }

  async save(factor: MfaFactor): Promise<void> {
    this.store.set(this.key(factor.sub, factor.type), { ...factor })
  }

  async delete(sub: string, type: MfaFactorType): Promise<void> {
    this.store.delete(this.key(sub, type))
  }
}

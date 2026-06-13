import type { Tenant } from '../../../src/spec-bindings/schemas'
import type { TenantRepository } from '../../../src/tenancy/ports/tenant-repository'

export class InMemoryTenantRepository implements TenantRepository {
  private readonly byId = new Map<string, Tenant>()

  async findById(id: string): Promise<Tenant | null> {
    return this.byId.get(id) ?? null
  }

  async findAll(): Promise<Tenant[]> {
    return [...this.byId.values()].sort((a, b) => (a.id < b.id ? -1 : a.id > b.id ? 1 : 0))
  }

  async save(tenant: Tenant): Promise<void> {
    this.byId.set(tenant.id, tenant)
  }
}

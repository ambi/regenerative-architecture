import type { Tenant } from '../../spec-bindings/schemas'

export interface TenantRepository {
  findById(id: string): Promise<Tenant | null>
  findAll(): Promise<Tenant[]>
  save(tenant: Tenant): Promise<void>
}

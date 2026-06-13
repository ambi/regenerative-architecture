/**
 * テナント lifecycle ユースケース (ADR-032)。
 *
 * Create / Update / SetDisabled / EnsureDefault。
 * default テナントは起動時に idempotent に upsert され、disable は不可。
 */

import { DEFAULT_TENANT_ID, TENANT_ID_PATTERN, TenantSchema, type Tenant } from '../../spec-bindings/schemas'
import type { TenantRepository } from '../ports/tenant-repository'

export class TenantNotFoundError extends Error {
  constructor() {
    super('tenant not found')
    this.name = 'TenantNotFoundError'
  }
}

export class TenantConflictError extends Error {
  constructor() {
    super('tenant already exists')
    this.name = 'TenantConflictError'
  }
}

export class InvalidTenantIdError extends Error {
  constructor() {
    super('invalid tenant id')
    this.name = 'InvalidTenantIdError'
  }
}

export class DisplayNameRequiredError extends Error {
  constructor() {
    super('display name is required')
    this.name = 'DisplayNameRequiredError'
  }
}

export class DefaultTenantImmutableError extends Error {
  constructor() {
    super('default tenant cannot be disabled')
    this.name = 'DefaultTenantImmutableError'
  }
}

export async function ensureDefaultTenant(
  repo: TenantRepository,
  now: Date = new Date(),
): Promise<void> {
  const existing = await repo.findById(DEFAULT_TENANT_ID)
  if (existing) return
  await repo.save(
    TenantSchema.parse({
      id: DEFAULT_TENANT_ID,
      display_name: 'Default',
      status: 'active',
      created_at: now.toISOString(),
    }),
  )
}

export async function createTenant(
  repo: TenantRepository,
  input: { id: string; display_name: string },
  now: Date = new Date(),
): Promise<Tenant> {
  const id = input.id.trim()
  const displayName = input.display_name.trim()
  if (!displayName) throw new DisplayNameRequiredError()
  if (!TENANT_ID_PATTERN.test(id) || id === 'admin') throw new InvalidTenantIdError()

  const existing = await repo.findById(id)
  if (existing) throw new TenantConflictError()

  const tenant = TenantSchema.parse({
    id,
    display_name: displayName,
    status: 'active',
    created_at: now.toISOString(),
  })
  await repo.save(tenant)
  return tenant
}

export async function updateTenant(
  repo: TenantRepository,
  id: string,
  input: { display_name: string },
  now: Date = new Date(),
): Promise<Tenant> {
  const displayName = input.display_name.trim()
  if (!displayName) throw new DisplayNameRequiredError()
  const tenant = await repo.findById(id)
  if (!tenant) throw new TenantNotFoundError()
  const updated: Tenant = {
    ...tenant,
    display_name: displayName,
    updated_at: now.toISOString(),
  }
  await repo.save(updated)
  return updated
}

export async function setTenantDisabled(
  repo: TenantRepository,
  id: string,
  disabled: boolean,
  now: Date = new Date(),
): Promise<Tenant> {
  if (id === DEFAULT_TENANT_ID && disabled) throw new DefaultTenantImmutableError()
  const tenant = await repo.findById(id)
  if (!tenant) throw new TenantNotFoundError()
  const updated: Tenant = {
    ...tenant,
    status: disabled ? 'disabled' : 'active',
    disabled_at: disabled ? now.toISOString() : undefined,
    updated_at: now.toISOString(),
  }
  await repo.save(updated)
  return updated
}

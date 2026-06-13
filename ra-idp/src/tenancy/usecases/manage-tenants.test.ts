import { describe, expect, it } from 'bun:test'

import { InMemoryTenantRepository } from '../../../adapters/persistence/memory/tenant-repository'
import {
  DefaultTenantImmutableError,
  InvalidTenantIdError,
  TenantConflictError,
  TenantNotFoundError,
  createTenant,
  ensureDefaultTenant,
  setTenantDisabled,
  updateTenant,
} from './manage-tenants'

describe('ensureDefaultTenant', () => {
  it('default テナントが無ければ idempotent に作成する', async () => {
    const repo = new InMemoryTenantRepository()
    await ensureDefaultTenant(repo)
    await ensureDefaultTenant(repo) // 2 回目でも throw しない
    const tenant = await repo.findById('default')
    expect(tenant?.status).toBe('active')
  })
})

describe('createTenant', () => {
  it('正常系: active で保存される', async () => {
    const repo = new InMemoryTenantRepository()
    const tenant = await createTenant(repo, { id: 'acme', display_name: 'Acme' })
    expect(tenant.id).toBe('acme')
    expect(tenant.status).toBe('active')
  })

  it('予約語 admin は拒否', async () => {
    const repo = new InMemoryTenantRepository()
    await expect(createTenant(repo, { id: 'admin', display_name: 'X' })).rejects.toBeInstanceOf(
      InvalidTenantIdError,
    )
  })

  it('既存 ID は conflict', async () => {
    const repo = new InMemoryTenantRepository()
    await createTenant(repo, { id: 'acme', display_name: 'Acme' })
    await expect(createTenant(repo, { id: 'acme', display_name: 'X' })).rejects.toBeInstanceOf(
      TenantConflictError,
    )
  })
})

describe('updateTenant / setTenantDisabled', () => {
  it('updateTenant は不在テナントで not found', async () => {
    const repo = new InMemoryTenantRepository()
    await expect(updateTenant(repo, 'ghost', { display_name: 'X' })).rejects.toBeInstanceOf(
      TenantNotFoundError,
    )
  })

  it('default tenant は disable 不可', async () => {
    const repo = new InMemoryTenantRepository()
    await ensureDefaultTenant(repo)
    await expect(setTenantDisabled(repo, 'default', true)).rejects.toBeInstanceOf(
      DefaultTenantImmutableError,
    )
  })

  it('disable→enable で status と disabled_at が往復する', async () => {
    const repo = new InMemoryTenantRepository()
    await createTenant(repo, { id: 'acme', display_name: 'Acme' })
    const disabled = await setTenantDisabled(repo, 'acme', true)
    expect(disabled.status).toBe('disabled')
    expect(disabled.disabled_at).toBeDefined()
    const enabled = await setTenantDisabled(repo, 'acme', false)
    expect(enabled.status).toBe('active')
    expect(enabled.disabled_at).toBeUndefined()
  })
})

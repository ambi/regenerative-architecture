/**
 * Layer 3 — Application Logic (admin: Client lifecycle)
 *
 * Mirrors ra-idp-go/internal/administration/usecases/clients.go.
 * Admin (system_admin in default tenant 不要・各テナント admin role) からの
 * クライアント CRUD を、tenant_id 境界に閉じて行う。認可境界 (admin role 検査・
 * CSRF) は HTTP adapter 側で行う。
 */

import { OAuthError } from '../../oauth2/protocol/oauth-error'
import type { ClientRepository } from '../../oauth2/ports/client-repository'
import {
  type RegisterClientInput,
  type RegisterClientResult,
  registerClientUseCase,
} from '../../oauth2/usecases/register-client'
import { ClientSchema, type Client, type DomainEvent } from '../../spec-bindings/schemas'

export class ClientNotFoundError extends Error {
  constructor(public readonly client_id: string) {
    super(`client not found: ${client_id}`)
    this.name = 'ClientNotFoundError'
  }
}

export interface AdminClientDeps {
  clientRepo: ClientRepository
  emit: (event: DomainEvent) => void
}

export interface CreateAdminClientInput {
  actorSub: string
  registration: RegisterClientInput
  now?: Date
}

export async function createAdminClient(
  deps: AdminClientDeps,
  input: CreateAdminClientInput,
): Promise<RegisterClientResult> {
  const now = input.now ?? new Date()
  const result = await registerClientUseCase(
    { clientRepo: deps.clientRepo },
    input.registration,
    now,
  )
  deps.emit({
    type: 'AdminClientCreated',
    occurredAt: now.toISOString(),
    actorSub: input.actorSub,
    clientId: result.client.client_id,
  })
  return result
}

export interface UpdateAdminClientInput {
  actorSub: string
  tenant_id: string
  client_id: string
  client_name?: string | null
  redirect_uris?: string[]
  grant_types?: Client['grant_types']
  response_types?: Client['response_types']
  scope?: string
  require_pushed_authorization_requests?: boolean
  dpop_bound_access_tokens?: boolean
  now?: Date
}

export async function updateAdminClient(
  deps: AdminClientDeps,
  input: UpdateAdminClientInput,
): Promise<Client> {
  const existing = await deps.clientRepo.findById(input.tenant_id, input.client_id)
  if (!existing) throw new ClientNotFoundError(input.client_id)

  const updated: Client = { ...existing }
  const changed: string[] = []

  if (input.client_name !== undefined && input.client_name !== (existing.client_name ?? null)) {
    updated.client_name = input.client_name ?? undefined
    changed.push('client_name')
  }
  if (
    input.redirect_uris !== undefined &&
    !arrayEqual(existing.redirect_uris, input.redirect_uris)
  ) {
    updated.redirect_uris = [...input.redirect_uris]
    changed.push('redirect_uris')
  }
  if (input.grant_types !== undefined && !arrayEqual(existing.grant_types, input.grant_types)) {
    updated.grant_types = [...input.grant_types]
    changed.push('grant_types')
  }
  if (
    input.response_types !== undefined &&
    !arrayEqual(existing.response_types, input.response_types)
  ) {
    updated.response_types = [...input.response_types]
    changed.push('response_types')
  }
  if (input.scope !== undefined && input.scope !== existing.scope) {
    updated.scope = input.scope
    changed.push('scope')
  }
  if (
    input.require_pushed_authorization_requests !== undefined &&
    input.require_pushed_authorization_requests !==
      (existing.require_pushed_authorization_requests ?? false)
  ) {
    updated.require_pushed_authorization_requests = input.require_pushed_authorization_requests
    changed.push('require_pushed_authorization_requests')
  }
  if (
    input.dpop_bound_access_tokens !== undefined &&
    input.dpop_bound_access_tokens !== (existing.dpop_bound_access_tokens ?? false)
  ) {
    updated.dpop_bound_access_tokens = input.dpop_bound_access_tokens
    changed.push('dpop_bound_access_tokens')
  }

  if (changed.length === 0) return existing

  const validated = ClientSchema.parse(updated)
  await deps.clientRepo.save(validated)

  const now = input.now ?? new Date()
  deps.emit({
    type: 'AdminClientUpdated',
    occurredAt: now.toISOString(),
    actorSub: input.actorSub,
    clientId: validated.client_id,
    changedFields: changed,
  })
  return validated
}

export interface DeleteAdminClientInput {
  actorSub: string
  tenant_id: string
  client_id: string
  now?: Date
}

export async function deleteAdminClient(
  deps: AdminClientDeps,
  input: DeleteAdminClientInput,
): Promise<void> {
  const existing = await deps.clientRepo.findById(input.tenant_id, input.client_id)
  if (!existing) throw new ClientNotFoundError(input.client_id)

  await deps.clientRepo.delete(input.tenant_id, input.client_id)

  const now = input.now ?? new Date()
  deps.emit({
    type: 'AdminClientDeleted',
    occurredAt: now.toISOString(),
    actorSub: input.actorSub,
    clientId: input.client_id,
  })
}

function arrayEqual<T>(a: readonly T[], b: readonly T[]): boolean {
  if (a.length !== b.length) return false
  for (let i = 0; i < a.length; i++) if (a[i] !== b[i]) return false
  return true
}

// Re-export OAuthError to keep adapter imports stable
export { OAuthError }

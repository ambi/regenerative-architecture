import type {
  AdminAgent,
  AdminAuditEvent,
  AdminClient,
  AdminConsent,
  AdminGroup,
  AdminGroupMember,
  AdminKey,
  AdminSettings,
  AdminTenant,
  AdminUser,
  AdminUserGroups,
  AuthorizationDetailType,
  TenantUserAttributeSchema,
  UserAttributeDef,
} from '../types'
import { adminRequest, request, tenantURL } from './core'

type AdminUserListResponse = { users: AdminUser[] }
type AdminClientListResponse = { clients: AdminClient[] }
type AdminConsentListResponse = { consents: AdminConsent[] }
type AdminAuditEventListResponse = { events: AdminAuditEvent[] }
type AdminKeyListResponse = { keys: AdminKey[] }
export type AdminRotateKeyResponse = { next: AdminKey; previous?: AdminKey }
type AdminTenantListResponse = { tenants: AdminTenant[] }

export type CreateAdminUserInput = {
  preferred_username: string
  password: string
  name?: string
  email?: string
  email_verified: boolean
  roles: string[]
}

export async function listAdminUsers(): Promise<AdminUser[]> {
  return (await request<AdminUserListResponse>('/api/admin/users')).users
}

export async function getAdminUser(sub: string): Promise<AdminUser> {
  return request<AdminUser>(`/api/admin/users/${encodeURIComponent(sub)}`)
}

export async function createAdminUser(
  csrfToken: string,
  input: CreateAdminUserInput,
): Promise<AdminUser> {
  return request('/api/admin/users', adminRequest(csrfToken, 'POST', input))
}

export type UpdateAdminUserInput = {
  preferred_username?: string
  name?: string
  given_name?: string
  family_name?: string
  email?: string
  email_verified?: boolean
  roles?: string[]
  attributes?: AdminUser['attributes']
}

export async function updateAdminUser(
  csrfToken: string,
  sub: string,
  input: UpdateAdminUserInput,
): Promise<AdminUser> {
  return request(
    `/api/admin/users/${encodeURIComponent(sub)}`,
    adminRequest(csrfToken, 'PATCH', input),
  )
}

export async function setAdminUserRequiredAction(
  csrfToken: string,
  sub: string,
  action: string,
): Promise<AdminUser> {
  return request(
    `/api/admin/users/${encodeURIComponent(sub)}/required_actions`,
    adminRequest(csrfToken, 'POST', { action }),
  )
}

export async function clearAdminUserRequiredAction(
  csrfToken: string,
  sub: string,
  action: string,
): Promise<AdminUser> {
  return request(
    `/api/admin/users/${encodeURIComponent(sub)}/required_actions/${encodeURIComponent(action)}`,
    adminRequest(csrfToken, 'DELETE'),
  )
}

export async function setAdminUserDisabled(
  csrfToken: string,
  sub: string,
  disabled: boolean,
): Promise<void> {
  await request(
    `/api/admin/users/${encodeURIComponent(sub)}/${disabled ? 'disable' : 'enable'}`,
    adminRequest(csrfToken, 'POST'),
  )
}

export async function deleteAdminUser(
  csrfToken: string,
  sub: string,
  reason?: string,
): Promise<void> {
  const body = reason?.trim() ? { reason: reason.trim() } : undefined
  await request(
    `/api/admin/users/${encodeURIComponent(sub)}`,
    adminRequest(csrfToken, 'DELETE', body),
  )
}

export type CreateAdminClientInput = {
  client_name?: string
  client_type: 'public' | 'confidential'
  redirect_uris: string[]
  grant_types: string[]
  response_types: string[]
  token_endpoint_auth_method: AdminClient['token_endpoint_auth_method']
  scope: string
  jwks_uri?: string
  tls_client_auth_subject_dn?: string
  require_pushed_authorization_requests: boolean
  dpop_bound_access_tokens: boolean
}

export type UpdateAdminClientInput = {
  client_name?: string
  redirect_uris: string[]
  grant_types: string[]
  response_types: string[]
  scope: string
  require_pushed_authorization_requests: boolean
  dpop_bound_access_tokens: boolean
}

export async function listAdminClients(): Promise<AdminClient[]> {
  return (await request<AdminClientListResponse>('/api/admin/clients')).clients
}

export async function getAdminClient(clientID: string): Promise<AdminClient> {
  return request<AdminClient>(`/api/admin/clients/${encodeURIComponent(clientID)}`)
}

export async function createAdminClient(
  csrfToken: string,
  input: CreateAdminClientInput,
): Promise<{ client: AdminClient; client_secret?: string }> {
  return request('/api/admin/clients', adminRequest(csrfToken, 'POST', input))
}

export async function updateAdminClient(
  csrfToken: string,
  clientID: string,
  input: UpdateAdminClientInput,
): Promise<AdminClient> {
  return request(
    `/api/admin/clients/${encodeURIComponent(clientID)}`,
    adminRequest(csrfToken, 'PATCH', input),
  )
}

export async function deleteAdminClient(csrfToken: string, clientID: string): Promise<void> {
  await request(
    `/api/admin/clients/${encodeURIComponent(clientID)}`,
    adminRequest(csrfToken, 'DELETE'),
  )
}

// authorization_details type (RFC 9396 / ADR-050) の管理 API クライアント。
export type AuthorizationDetailTypeInput = {
  type?: string
  description?: string
  display_template: string
  state?: AuthorizationDetailType['state']
  schema: AuthorizationDetailType['schema']
}

export async function listAuthorizationDetailTypes(): Promise<AuthorizationDetailType[]> {
  return (
    await request<{ types: AuthorizationDetailType[] }>('/api/admin/authorization-detail-types')
  ).types
}

export async function createAuthorizationDetailType(
  csrfToken: string,
  input: AuthorizationDetailTypeInput,
): Promise<AuthorizationDetailType> {
  return request('/api/admin/authorization-detail-types', adminRequest(csrfToken, 'POST', input))
}

export async function updateAuthorizationDetailType(
  csrfToken: string,
  detailType: string,
  input: AuthorizationDetailTypeInput,
): Promise<AuthorizationDetailType> {
  return request(
    `/api/admin/authorization-detail-types/${encodeURIComponent(detailType)}`,
    adminRequest(csrfToken, 'PATCH', input),
  )
}

export async function deleteAuthorizationDetailType(
  csrfToken: string,
  detailType: string,
): Promise<void> {
  await request(
    `/api/admin/authorization-detail-types/${encodeURIComponent(detailType)}`,
    adminRequest(csrfToken, 'DELETE'),
  )
}

export async function listAdminConsents(): Promise<AdminConsent[]> {
  return (await request<AdminConsentListResponse>('/api/admin/consents')).consents
}

export async function revokeAdminConsent(
  csrfToken: string,
  sub: string,
  clientID: string,
): Promise<void> {
  await request(
    `/api/admin/consents/${encodeURIComponent(sub)}/${encodeURIComponent(clientID)}`,
    adminRequest(csrfToken, 'DELETE'),
  )
}

// イベントカテゴリ (wi-44 統合)。認証サブ分類 + 管理操作カテゴリ。
export type AdminAuditEventCategory =
  | 'authentication'
  | 'success'
  | 'fail'
  | 'aggregated'
  | 'user'
  | 'group'
  | 'client'
  | 'consent'
  | 'token'
  | 'tenant'
  | 'key'

export type AdminAuditEventQuery = {
  // type 完全一致 (機械向け低レベルフィルタ)。UI には出さない。
  type?: string
  category?: AdminAuditEventCategory
  sub?: string
  after?: string
  before?: string
  limit?: number
  allTenants?: boolean
}

function auditEventParams(query: AdminAuditEventQuery): URLSearchParams {
  const params = new URLSearchParams()
  if (query.type) params.set('type', query.type)
  if (query.category) params.set('category', query.category)
  if (query.sub) params.set('sub', query.sub)
  if (query.after) params.set('after', query.after)
  if (query.before) params.set('before', query.before)
  if (query.limit !== undefined) params.set('limit', String(query.limit))
  if (query.allTenants) params.set('all_tenants', 'true')
  return params
}

export async function listAdminAuditEvents(
  query: AdminAuditEventQuery,
): Promise<AdminAuditEvent[]> {
  const params = auditEventParams(query)
  const url =
    params.size > 0 ? `/api/admin/audit_events?${params.toString()}` : '/api/admin/audit_events'
  return (await request<AdminAuditEventListResponse>(url)).events
}

// 監査イベントのエクスポート URL (認証イベント含む)。新規タブで開いてダウンロードする。
export function adminAuditEventsExportURL(query: AdminAuditEventQuery): string {
  const params = auditEventParams(query)
  return tenantURL(`/api/admin/audit_events/export?${params.toString()}`)
}

export async function listAdminKeys(): Promise<AdminKey[]> {
  return (await request<AdminKeyListResponse>('/api/admin/keys')).keys
}

export async function rotateAdminKey(csrfToken: string): Promise<AdminRotateKeyResponse> {
  return request<AdminRotateKeyResponse>('/api/admin/keys/rotate', adminRequest(csrfToken, 'POST'))
}

export type UpdateAdminSettingsInput = {
  display_name?: string
  password_policy_override?: AdminSettings['password_policy_override']
}

export async function getAdminSettings(): Promise<AdminSettings> {
  return request<AdminSettings>('/api/admin/settings')
}

export async function updateAdminSettings(
  csrfToken: string,
  input: UpdateAdminSettingsInput,
): Promise<AdminSettings> {
  return request('/api/admin/settings', adminRequest(csrfToken, 'PATCH', input))
}
export async function getTenantUserAttributeSchema(): Promise<TenantUserAttributeSchema> {
  return request<TenantUserAttributeSchema>('/api/admin/tenant/user_attribute_schema')
}

export async function updateTenantUserAttributeSchema(
  csrfToken: string,
  attributes: UserAttributeDef[],
): Promise<TenantUserAttributeSchema> {
  return request(
    '/api/admin/tenant/user_attribute_schema',
    adminRequest(csrfToken, 'PUT', { attributes }),
  )
}

export async function listAdminTenants(): Promise<AdminTenant[]> {
  return (await request<AdminTenantListResponse>('/admin/tenants')).tenants
}

export type CreateAdminTenantInput = {
  id: string
  display_name: string
}

export type UpdateAdminTenantInput = {
  display_name?: string
  password_policy_override?: AdminTenant['password_policy_override']
}

export async function createAdminTenant(
  csrfToken: string,
  input: CreateAdminTenantInput,
): Promise<AdminTenant> {
  return request('/admin/tenants', adminRequest(csrfToken, 'POST', input))
}

export async function updateAdminTenant(
  csrfToken: string,
  tenantID: string,
  input: UpdateAdminTenantInput,
): Promise<AdminTenant> {
  return request(
    `/admin/tenants/${encodeURIComponent(tenantID)}`,
    adminRequest(csrfToken, 'PATCH', input),
  )
}

export async function setAdminTenantDisabled(
  csrfToken: string,
  tenantID: string,
  disabled: boolean,
): Promise<void> {
  await request(
    `/admin/tenants/${encodeURIComponent(tenantID)}/${disabled ? 'disable' : 'enable'}`,
    adminRequest(csrfToken, 'POST'),
  )
}

export async function listAdminGroups(): Promise<AdminGroup[]> {
  return (await request<{ groups: AdminGroup[] }>('/api/admin/groups')).groups
}

export async function getAdminGroup(
  id: string,
): Promise<{ group: AdminGroup; members: AdminGroupMember[] }> {
  return request(`/api/admin/groups/${encodeURIComponent(id)}`)
}

export type CreateAdminGroupInput = {
  name: string
  description?: string
  roles?: string[]
}

export type UpdateAdminGroupInput = {
  name?: string
  description?: string
  roles?: string[]
}

export async function createAdminGroup(
  csrfToken: string,
  input: CreateAdminGroupInput,
): Promise<AdminGroup> {
  return request('/api/admin/groups', adminRequest(csrfToken, 'POST', input))
}

export async function updateAdminGroup(
  csrfToken: string,
  id: string,
  input: UpdateAdminGroupInput,
): Promise<AdminGroup> {
  return request(
    `/api/admin/groups/${encodeURIComponent(id)}`,
    adminRequest(csrfToken, 'PATCH', input),
  )
}

export async function deleteAdminGroup(csrfToken: string, id: string): Promise<void> {
  await request(`/api/admin/groups/${encodeURIComponent(id)}`, adminRequest(csrfToken, 'DELETE'))
}

export async function addAdminGroupMember(
  csrfToken: string,
  groupID: string,
  userSub: string,
): Promise<void> {
  await request(
    `/api/admin/groups/${encodeURIComponent(groupID)}/members/${encodeURIComponent(userSub)}`,
    adminRequest(csrfToken, 'POST'),
  )
}

export async function removeAdminGroupMember(
  csrfToken: string,
  groupID: string,
  userSub: string,
): Promise<void> {
  await request(
    `/api/admin/groups/${encodeURIComponent(groupID)}/members/${encodeURIComponent(userSub)}`,
    adminRequest(csrfToken, 'DELETE'),
  )
}

export async function getAdminUserGroups(sub: string): Promise<AdminUserGroups> {
  return request(`/api/admin/users/${encodeURIComponent(sub)}/groups`)
}

export async function listAdminAgents(): Promise<AdminAgent[]> {
  return (await request<{ agents: AdminAgent[] }>('/api/admin/agents')).agents
}

export async function getAdminAgent(id: string): Promise<AdminAgent> {
  return request<AdminAgent>(`/api/admin/agents/${encodeURIComponent(id)}`)
}

export type RegisterAdminAgentInput = {
  name: string
  description?: string
  kind?: AdminAgent['kind']
  owner_sub?: string
  roles?: string[]
}

export type UpdateAdminAgentInput = {
  name?: string
  description?: string
  kind?: AdminAgent['kind']
  owner_sub?: string
  roles?: string[]
}

export async function registerAdminAgent(
  csrfToken: string,
  input: RegisterAdminAgentInput,
): Promise<AdminAgent> {
  return request('/api/admin/agents', adminRequest(csrfToken, 'POST', input))
}

export async function updateAdminAgent(
  csrfToken: string,
  id: string,
  input: UpdateAdminAgentInput,
): Promise<AdminAgent> {
  return request(
    `/api/admin/agents/${encodeURIComponent(id)}`,
    adminRequest(csrfToken, 'PATCH', input),
  )
}

export async function disableAdminAgent(csrfToken: string, id: string): Promise<void> {
  await request(
    `/api/admin/agents/${encodeURIComponent(id)}/disable`,
    adminRequest(csrfToken, 'POST'),
  )
}

export async function enableAdminAgent(csrfToken: string, id: string): Promise<void> {
  await request(
    `/api/admin/agents/${encodeURIComponent(id)}/enable`,
    adminRequest(csrfToken, 'POST'),
  )
}

export async function killAdminAgent(csrfToken: string, id: string): Promise<void> {
  await request(`/api/admin/agents/${encodeURIComponent(id)}/kill`, adminRequest(csrfToken, 'POST'))
}

export async function deleteAdminAgent(csrfToken: string, id: string): Promise<void> {
  await request(`/api/admin/agents/${encodeURIComponent(id)}`, adminRequest(csrfToken, 'DELETE'))
}

export async function bindAdminAgentCredential(
  csrfToken: string,
  agentID: string,
  clientID: string,
): Promise<void> {
  await request(
    `/api/admin/agents/${encodeURIComponent(agentID)}/credentials`,
    adminRequest(csrfToken, 'POST', { client_id: clientID }),
  )
}

export async function unbindAdminAgentCredential(
  csrfToken: string,
  agentID: string,
  clientID: string,
): Promise<void> {
  await request(
    `/api/admin/agents/${encodeURIComponent(agentID)}/credentials/${encodeURIComponent(clientID)}`,
    adminRequest(csrfToken, 'DELETE'),
  )
}

import type {
  AdminAgent,
  AdminApplication,
  AdminApplicationDetail,
  AdminAuditEvent,
  AdminConsent,
  AdminGroup,
  AdminGroupMember,
  AdminKey,
  AdminSettings,
  AdminTenant,
  AdminUser,
  AdminUserGroups,
  ApplicationAssignment,
  ApplicationStatus,
  ProtocolBinding,
  ProtocolBindingType,
  AuthorizationDetailType,
  TenantUserAttributeSchema,
  UserAttributeDef,
  EntraFederationProfile,
  WsFedClaimMappingRule,
  WsFedRelyingParty,
  WsFedTokenType,
} from '../types'
import { adminRequest, request, tenantURL } from './core'

type AdminUserListResponse = { users: AdminUser[] }
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

type WsFedRelyingPartyListResponse = { relying_parties: WsFedRelyingParty[] | null }

export async function listWsFedRelyingParties(): Promise<WsFedRelyingParty[]> {
  const response = await request<WsFedRelyingPartyListResponse>('/api/admin/wsfed/relying-parties')
  return response.relying_parties ?? []
}

export async function deleteWsFedRelyingParty(csrfToken: string, wtrealm: string): Promise<void> {
  await request(
    `/api/admin/wsfed/relying-parties?wtrealm=${encodeURIComponent(wtrealm)}`,
    adminRequest(csrfToken, 'DELETE'),
  )
}

export type ConfigureEntraFederationInput = {
  domain: string
  issuer_uri?: string
  source_anchor_attribute: string
  reply_url?: string
}

export type ConfigureEntraFederationResponse = {
  profile: EntraFederationProfile
  relying_party: WsFedRelyingParty
  powershell: Record<string, string>
  known_limitations: string[]
}

export async function configureEntraFederation(
  csrfToken: string,
  input: ConfigureEntraFederationInput,
): Promise<ConfigureEntraFederationResponse> {
  return request('/api/admin/wsfed/entra-federation', adminRequest(csrfToken, 'POST', input))
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

// Application カタログ (wi-69)。種別を選びプロトコル設定もまとめて入力する一括作成 API。
// backend が OAuth2 client / WS-Fed RP を作成し、Application と binding を一括で作る。
export type CreateAdminApplicationInput = {
  name: string
  type: 'oidc' | 'wsfed' | 'weblink' | 'service'
  icon_url?: string
  launch_url?: string
  // OIDC
  redirect_uris?: string[]
  // OIDC / service の生成 client 設定。認証方式は作成時に確定し以後不変。
  scope?: string
  client_type?: 'public' | 'confidential'
  token_endpoint_auth_method?: string
  jwks_uri?: string
  tls_client_auth_subject_dn?: string
  // WS-Federation
  wtrealm?: string
  reply_urls?: string[]
  name_id_format?: string
  name_id_source?: string
}

// OIDC を一括作成すると client_secret が一度だけ返る (再表示不可)。
export type CreateAdminApplicationResult = {
  application: AdminApplication
  client_id?: string
  client_secret?: string
}

export type UpdateAdminApplicationInput = {
  name?: string
  status?: ApplicationStatus
  icon_url?: string
  launch_url?: string
}

export type UpdateApplicationOidcInput = {
  redirect_uris?: string[]
  grant_types?: string[]
  response_types?: string[]
  scope?: string
  require_pushed_authorization_requests?: boolean
  dpop_bound_access_tokens?: boolean
}

export type UpdateApplicationWsFedInput = {
  reply_urls?: string[]
  audience?: string
  token_type?: WsFedTokenType
  name_id_format?: string
  name_id_source?: string
  rules?: WsFedClaimMappingRule[]
}

export async function listAdminApplications(): Promise<AdminApplication[]> {
  return (await request<{ applications: AdminApplication[] }>('/api/admin/applications'))
    .applications
}

export async function getAdminApplication(id: string): Promise<AdminApplicationDetail> {
  return request<AdminApplicationDetail>(`/api/admin/applications/${encodeURIComponent(id)}`)
}

export async function createAdminApplication(
  csrfToken: string,
  input: CreateAdminApplicationInput,
): Promise<CreateAdminApplicationResult> {
  return request('/api/admin/applications', adminRequest(csrfToken, 'POST', input))
}

export async function updateApplicationOidcConfig(
  csrfToken: string,
  id: string,
  input: UpdateApplicationOidcInput,
): Promise<void> {
  await request(
    `/api/admin/applications/${encodeURIComponent(id)}/oidc`,
    adminRequest(csrfToken, 'PATCH', input),
  )
}

export async function updateApplicationWsFedConfig(
  csrfToken: string,
  id: string,
  input: UpdateApplicationWsFedInput,
): Promise<void> {
  await request(
    `/api/admin/applications/${encodeURIComponent(id)}/wsfed`,
    adminRequest(csrfToken, 'PATCH', input),
  )
}

export async function updateAdminApplication(
  csrfToken: string,
  id: string,
  input: UpdateAdminApplicationInput,
): Promise<AdminApplication> {
  return request(
    `/api/admin/applications/${encodeURIComponent(id)}`,
    adminRequest(csrfToken, 'PATCH', input),
  )
}

export async function deleteAdminApplication(csrfToken: string, id: string): Promise<void> {
  await request(
    `/api/admin/applications/${encodeURIComponent(id)}`,
    adminRequest(csrfToken, 'DELETE'),
  )
}

export async function attachProtocolBinding(
  csrfToken: string,
  id: string,
  binding: ProtocolBinding,
): Promise<AdminApplication> {
  return request(
    `/api/admin/applications/${encodeURIComponent(id)}/bindings`,
    adminRequest(csrfToken, 'POST', binding),
  )
}

export async function detachProtocolBinding(
  csrfToken: string,
  id: string,
  bindingType: ProtocolBindingType,
): Promise<void> {
  await request(
    `/api/admin/applications/${encodeURIComponent(id)}/bindings/${encodeURIComponent(bindingType)}`,
    adminRequest(csrfToken, 'DELETE'),
  )
}

export async function listApplicationAssignments(id: string): Promise<ApplicationAssignment[]> {
  return (
    await request<{ assignments: ApplicationAssignment[] }>(
      `/api/admin/applications/${encodeURIComponent(id)}/assignments`,
    )
  ).assignments
}

export type AssignApplicationInput = {
  subject_type: 'user' | 'group'
  subject_id: string
  visibility?: 'visible' | 'hidden'
}

export async function assignApplication(
  csrfToken: string,
  id: string,
  input: AssignApplicationInput,
): Promise<ApplicationAssignment> {
  return request(
    `/api/admin/applications/${encodeURIComponent(id)}/assignments`,
    adminRequest(csrfToken, 'POST', input),
  )
}

export async function unassignApplication(
  csrfToken: string,
  id: string,
  subjectType: 'user' | 'group',
  subjectID: string,
): Promise<void> {
  await request(
    `/api/admin/applications/${encodeURIComponent(id)}/assignments/${encodeURIComponent(subjectType)}/${encodeURIComponent(subjectID)}`,
    adminRequest(csrfToken, 'DELETE'),
  )
}

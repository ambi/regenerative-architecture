import type {
  BrowserFlowResponse,
  AccountProfile,
  AccountProfilePage,
  AdminAuditEvent,
  AdminAuditEventsPage,
  AdminTenantAttributesPage,
  AdminClient,
  AdminClientsPage,
  AdminConsent,
  AdminConsentsPage,
  AdminDashboardPage,
  AdminGroup,
  AdminGroupMember,
  AdminGroupsPage,
  AdminUserGroups,
  AdminKey,
  AdminKeysPage,
  AdminRole,
  AdminRolesPage,
  AdminSettings,
  AdminSettingsPage,
  AdminTenant,
  AdminTenantsPage,
  AdminUser,
  AdminUsersPage,
  ChangePasswordPage,
  ConsentPage,
  CallbackPage,
  DevicePage,
  ForgotPasswordPage,
  HomePage,
  LoginPage,
  PageData,
  StatusPage,
  TenantUserAttributeSchema,
  TotpPage,
  UserAttributeDef,
  ResetPasswordPage,
} from './types'

type TransactionResponse = {
  kind: 'login' | 'totp' | 'consent'
  csrf_token: string
  client_name?: string
  scopes?: string[]
}

type DeviceResponse = {
  kind: 'device'
  csrf_token: string
  user_code: string
}

type AccountContextResponse = {
  csrf_token: string
  sub: string
  preferred_username?: string
  tenant_id?: string
  roles?: string[]
}

type PasswordResetContextResponse = {
  csrf_token: string
}

type APIError = {
  error?: string
  message?: string
  error_description?: string
}

type AdminUserListResponse = {
  users: AdminUser[]
}

type AdminClientListResponse = {
  clients: AdminClient[]
}

type AdminConsentListResponse = {
  consents: AdminConsent[]
}

type AdminAuditEventListResponse = {
  events: AdminAuditEvent[]
}

type AdminKeyListResponse = {
  keys: AdminKey[]
}

type AdminRotateKeyResponse = {
  next: AdminKey
  previous?: AdminKey
}

type AdminTenantListResponse = {
  tenants: AdminTenant[]
}

type AdminRoleListResponse = {
  roles: AdminRole[]
}

export class AuthenticationAPIError extends Error {
  code?: string

  constructor(message: string, code?: string) {
    super(message)
    this.name = 'AuthenticationAPIError'
    this.code = code
  }
}

export class UnauthenticatedError extends AuthenticationAPIError {
  constructor(message: string, code?: string) {
    super(message, code)
    this.name = 'UnauthenticatedError'
  }
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(tenantURL(url), {
    credentials: 'same-origin',
    cache: 'no-store',
    ...init,
  })
  const body = (await response.json().catch(() => ({}))) as T & APIError
  if (!response.ok) {
    const message = body.message ?? body.error_description ?? '認証サービスに接続できませんでした。'
    if (response.status === 401) {
      throw new UnauthenticatedError(message, body.error)
    }
    throw new AuthenticationAPIError(message, body.error)
  }
  return body
}

export async function loadPageData(): Promise<PageData> {
  const path = tenantLocalPath()
  let adminAccount: AccountContextResponse | undefined
  if (path === '/admin' || path.startsWith('/admin/')) {
    try {
      adminAccount = await request<AccountContextResponse>('/api/auth/account')
    } catch (error) {
      if (!(error instanceof UnauthenticatedError)) throw error
      const returnTo = `${window.location.pathname}${window.location.search}`
      window.location.assign(`${tenantURL('/login')}?return_to=${encodeURIComponent(returnTo)}`)
      return new Promise<PageData>(() => {})
    }
  }
  if (path === '/') {
    return { kind: 'home', demoEnabled: import.meta.env.DEV } satisfies HomePage
  }
  if (path === '/status') {
    const state = new URLSearchParams(window.location.search).get('state')
    const supported = ['approved', 'denied', 'signed-out', 'authentication-required'] as const
    const status = supported.find((value) => value === state) ?? 'authentication-required'
    return { kind: 'status', status } satisfies StatusPage
  }
  if (path === '/callback') {
    const parameters = new URLSearchParams(window.location.search)
    return {
      kind: 'callback',
      code: parameters.get('code') ?? undefined,
      error: parameters.get('error') ?? undefined,
      errorDescription: parameters.get('error_description') ?? undefined,
    } satisfies CallbackPage
  }
  if (path === '/device') {
    const userCode = new URLSearchParams(window.location.search).get('user_code') ?? ''
    const data = await request<DeviceResponse>(
      `/api/auth/device?user_code=${encodeURIComponent(userCode)}`,
    )
    return {
      kind: 'device',
      csrfToken: data.csrf_token,
      userCode: data.user_code,
    } satisfies DevicePage
  }
  if (path === '/account/password') {
    const data = await request<AccountContextResponse>('/api/auth/account')
    return {
      kind: 'change-password',
      csrfToken: data.csrf_token,
      sub: data.sub,
      preferredUsername: data.preferred_username,
    } satisfies ChangePasswordPage
  }
  if (path === '/account/profile') {
    const account = await request<AccountContextResponse>('/api/auth/account')
    const profile = await request<AccountProfile>('/api/account/profile')
    return {
      kind: 'account-profile',
      csrfToken: account.csrf_token,
      profile,
    } satisfies AccountProfilePage
  }
  if (path === '/admin') {
    const since = new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString()
    const [users, clients, consents, recentEvents] = await Promise.all([
      request<AdminUserListResponse>('/api/admin/users'),
      request<AdminClientListResponse>('/api/admin/clients'),
      request<AdminConsentListResponse>('/api/admin/consents'),
      listAdminAuditEvents({ after: since, limit: 100 }),
    ])
    const account = adminAccount!
    const activeUserCount = users.users.filter((u) => !u.disabled_at).length
    return {
      kind: 'admin-dashboard',
      csrfToken: account.csrf_token,
      actorUsername: account.preferred_username,
      actorRoles: account.roles ?? [],
      actorTenantID: account.tenant_id ?? '',
      userCount: users.users.length,
      activeUserCount,
      disabledUserCount: users.users.length - activeUserCount,
      clientCount: clients.clients.length,
      grantedConsentCount: consents.consents.filter((c) => c.state === 'granted').length,
      auditEventCount24h: recentEvents.length,
      recentEvents: recentEvents.slice(0, 5),
    } satisfies AdminDashboardPage
  }
  if (path === '/admin/users') {
    const [users, schema] = await Promise.all([
      request<AdminUserListResponse>('/api/admin/users'),
      request<TenantUserAttributeSchema>('/api/admin/tenant/user_attribute_schema'),
    ])
    return {
      kind: 'admin-users',
      csrfToken: adminAccount!.csrf_token,
      actorUsername: adminAccount!.preferred_username,
      users: users.users,
      attributeDefs: [...schema.builtin, ...schema.attributes],
    } satisfies AdminUsersPage
  }
  if (path === '/admin/roles') {
    const [roles, users] = await Promise.all([
      request<AdminRoleListResponse>('/api/admin/policy/roles'),
      request<AdminUserListResponse>('/api/admin/users'),
    ])
    return {
      kind: 'admin-roles',
      actorUsername: adminAccount!.preferred_username,
      roles: roles.roles,
      users: users.users,
    } satisfies AdminRolesPage
  }
  if (path === '/admin/clients') {
    const clients = await request<AdminClientListResponse>('/api/admin/clients')
    return {
      kind: 'admin-clients',
      csrfToken: adminAccount!.csrf_token,
      actorUsername: adminAccount!.preferred_username,
      clients: clients.clients,
    } satisfies AdminClientsPage
  }
  if (path === '/admin/consents') {
    const consents = await request<AdminConsentListResponse>('/api/admin/consents')
    return {
      kind: 'admin-consents',
      csrfToken: adminAccount!.csrf_token,
      actorUsername: adminAccount!.preferred_username,
      consents: consents.consents,
    } satisfies AdminConsentsPage
  }
  if (path === '/admin/audit_events') {
    const events = await request<AdminAuditEventListResponse>('/api/admin/audit_events')
    return {
      kind: 'admin-audit-events',
      csrfToken: adminAccount!.csrf_token,
      actorUsername: adminAccount!.preferred_username,
      actorRoles: adminAccount!.roles ?? [],
      actorTenantID: adminAccount!.tenant_id ?? '',
      events: events.events,
    } satisfies AdminAuditEventsPage
  }
  if (path === '/admin/keys') {
    const keys = await request<AdminKeyListResponse>('/api/admin/keys')
    return {
      kind: 'admin-keys',
      csrfToken: adminAccount!.csrf_token,
      actorUsername: adminAccount!.preferred_username,
      actorRoles: adminAccount!.roles ?? [],
      actorTenantID: adminAccount!.tenant_id ?? '',
      keys: keys.keys,
    } satisfies AdminKeysPage
  }
  if (path === '/admin/settings') {
    const settings = await request<AdminSettings>('/api/admin/settings')
    return {
      kind: 'admin-settings',
      csrfToken: adminAccount!.csrf_token,
      actorUsername: adminAccount!.preferred_username,
      actorRoles: adminAccount!.roles ?? [],
      actorTenantID: adminAccount!.tenant_id ?? '',
      settings,
    } satisfies AdminSettingsPage
  }
  if (path === '/admin/tenant/attributes') {
    const schema = await request<TenantUserAttributeSchema>(
      '/api/admin/tenant/user_attribute_schema',
    )
    return {
      kind: 'admin-tenant-attributes',
      csrfToken: adminAccount!.csrf_token,
      actorUsername: adminAccount!.preferred_username,
      schema,
    } satisfies AdminTenantAttributesPage
  }
  if (path === '/admin/tenants') {
    const tenants = await request<AdminTenantListResponse>('/admin/tenants')
    return {
      kind: 'admin-tenants',
      csrfToken: adminAccount!.csrf_token,
      actorUsername: adminAccount!.preferred_username,
      tenants: tenants.tenants,
    } satisfies AdminTenantsPage
  }
  if (path === '/admin/groups') {
    const groups = await request<{ groups: AdminGroup[] }>('/api/admin/groups')
    return {
      kind: 'admin-groups',
      csrfToken: adminAccount!.csrf_token,
      actorUsername: adminAccount!.preferred_username,
      groups: groups.groups,
    } satisfies AdminGroupsPage
  }
  if (path === '/forgot_password' || path === '/reset_password') {
    const data = await request<PasswordResetContextResponse>('/api/auth/password_reset_context')
    if (path === '/forgot_password') {
      return { kind: 'forgot-password', csrfToken: data.csrf_token } satisfies ForgotPasswordPage
    }
    return {
      kind: 'reset-password',
      csrfToken: data.csrf_token,
      token: new URLSearchParams(window.location.search).get('token') ?? '',
    } satisfies ResetPasswordPage
  }

  const requestedReturnTo = new URLSearchParams(window.location.search).get('return_to') ?? ''
  const returnTo = requestedReturnTo
    ? validAdminReturnTo(requestedReturnTo)
      ? requestedReturnTo
      : tenantURL('/admin')
    : undefined
  const transactionURL = returnTo
    ? `/api/auth/transaction?return_to=${encodeURIComponent(returnTo)}`
    : '/api/auth/transaction'
  const data = await request<TransactionResponse>(transactionURL)
  if (data.kind === 'consent') {
    if (path !== '/consent') {
      window.history.replaceState(null, '', tenantURL('/consent'))
    }
    return {
      kind: 'consent',
      csrfToken: data.csrf_token,
      clientName: data.client_name ?? '',
      scopes: data.scopes ?? [],
    } satisfies ConsentPage
  }
  if (data.kind === 'totp') {
    if (path !== '/totp') {
      window.history.replaceState(null, '', tenantURL('/totp'))
    }
    return { kind: 'totp', csrfToken: data.csrf_token, returnTo } satisfies TotpPage
  }
  if (path !== '/login') {
    window.history.replaceState(null, '', tenantURL('/login'))
  }
  return { kind: 'login', csrfToken: data.csrf_token, returnTo } satisfies LoginPage
}

export async function login(
  csrfToken: string,
  username: string,
  password: string,
  returnTo?: string,
): Promise<BrowserFlowResponse> {
  return request('/api/auth/login', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': csrfToken,
    },
    body: JSON.stringify({ username, password, return_to: returnTo }),
  })
}

export async function submitConsent(
  csrfToken: string,
  action: 'allow' | 'deny',
): Promise<BrowserFlowResponse> {
  return request('/api/auth/consent', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': csrfToken,
    },
    body: JSON.stringify({ action }),
  })
}

export async function submitTOTP(
  csrfToken: string,
  code: string,
  returnTo?: string,
): Promise<BrowserFlowResponse> {
  return request('/api/auth/totp', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': csrfToken,
    },
    body: JSON.stringify({ code, return_to: returnTo }),
  })
}

export class PasswordPolicyError extends AuthenticationAPIError {
  violations: string[]

  constructor(message: string, violations: string[]) {
    super(message, 'password_policy')
    this.name = 'PasswordPolicyError'
    this.violations = violations
  }
}

export async function changePassword(
  csrfToken: string,
  currentPassword: string,
  newPassword: string,
): Promise<void> {
  const response = await fetch(tenantURL('/api/auth/change_password'), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': csrfToken,
    },
    body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
    credentials: 'same-origin',
    cache: 'no-store',
  })
  if (response.status === 204) return
  const body = (await response.json().catch(() => ({}))) as {
    error?: string
    message?: string
    violations?: string[]
  }
  if (body.error === 'password_policy') {
    throw new PasswordPolicyError(
      body.message ?? 'パスワードがセキュリティ要件を満たしていません。',
      body.violations ?? [],
    )
  }
  throw new AuthenticationAPIError(
    body.message ?? '認証サービスに接続できませんでした。',
    body.error,
  )
}

export async function requestPasswordReset(csrfToken: string, email: string): Promise<void> {
  const response = await fetch(tenantURL('/api/auth/forgot_password'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
    body: JSON.stringify({ email }),
    credentials: 'same-origin',
    cache: 'no-store',
  })
  if (response.status === 204) return
  const body = (await response.json().catch(() => ({}))) as APIError
  throw new AuthenticationAPIError(
    body.message ?? 'リセット要求を送信できませんでした。',
    body.error,
  )
}

export async function resetPassword(
  csrfToken: string,
  token: string,
  newPassword: string,
): Promise<void> {
  const response = await fetch(tenantURL('/api/auth/reset_password'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
    body: JSON.stringify({ token, new_password: newPassword }),
    credentials: 'same-origin',
    cache: 'no-store',
  })
  if (response.ok) return
  const body = (await response.json().catch(() => ({}))) as APIError & { violations?: string[] }
  if (body.error === 'password_policy') {
    throw new PasswordPolicyError(
      body.message ?? 'パスワードがセキュリティ要件を満たしていません。',
      body.violations ?? [],
    )
  }
  throw new AuthenticationAPIError(
    body.message ?? 'パスワードをリセットできませんでした。',
    body.error,
  )
}

export async function submitDevice(
  csrfToken: string,
  userCode: string,
  action: 'allow' | 'deny',
): Promise<BrowserFlowResponse> {
  return request('/api/auth/device', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': csrfToken,
    },
    body: JSON.stringify({ user_code: userCode, action }),
  })
}

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

export type AdminAuditEventQuery = {
  type?: string
  sub?: string
  after?: string
  before?: string
  limit?: number
  allTenants?: boolean
}

export async function listAdminAuditEvents(
  query: AdminAuditEventQuery,
): Promise<AdminAuditEvent[]> {
  const params = new URLSearchParams()
  if (query.type) params.set('type', query.type)
  if (query.sub) params.set('sub', query.sub)
  if (query.after) params.set('after', query.after)
  if (query.before) params.set('before', query.before)
  if (query.limit !== undefined) params.set('limit', String(query.limit))
  if (query.allTenants) params.set('all_tenants', 'true')
  const url =
    params.size > 0 ? `/api/admin/audit_events?${params.toString()}` : '/api/admin/audit_events'
  return (await request<AdminAuditEventListResponse>(url)).events
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

export type UpdateAccountProfileInput = {
  name?: string
  given_name?: string
  family_name?: string
  attributes?: AccountProfile['attributes']
}

export async function getAccountProfile(): Promise<AccountProfile> {
  return request<AccountProfile>('/api/account/profile')
}

export async function updateAccountProfile(
  csrfToken: string,
  input: UpdateAccountProfileInput,
): Promise<AccountProfile> {
  return request('/api/account/profile', adminRequest(csrfToken, 'PATCH', input))
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

function adminRequest(csrfToken: string, method: string, body?: unknown): RequestInit {
  return {
    method,
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': csrfToken,
    },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  }
}

export function continueBrowserFlow(result: BrowserFlowResponse) {
  const destination = result.redirect_to ?? result.next
  if (!destination) {
    throw new AuthenticationAPIError('認証フローの遷移先がありません。')
  }
  window.location.assign(destination)
}

export async function startDemoAuthorization() {
  const verifierBytes = crypto.getRandomValues(new Uint8Array(32))
  const verifier = base64URL(verifierBytes)
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier))
  const state = base64URL(crypto.getRandomValues(new Uint8Array(16)))
  const nonce = base64URL(crypto.getRandomValues(new Uint8Array(16)))

  sessionStorage.setItem('ra-idp-demo-code-verifier', verifier)
  const parameters = new URLSearchParams({
    response_type: 'code',
    client_id: 'demo-client',
    redirect_uri: `${window.location.origin}${tenantURL('/callback')}`,
    scope: 'openid profile email offline_access',
    state,
    nonce,
    code_challenge: base64URL(new Uint8Array(digest)),
    code_challenge_method: 'S256',
  })
  window.location.assign(`${tenantURL('/authorize')}?${parameters.toString()}`)
}

export function tenantBasePath(path = window.location.pathname): string {
  const match = path.match(/^\/realms\/([a-z0-9][a-z0-9-]{0,62})(?:\/|$)/)
  return match ? `/realms/${match[1]}` : ''
}

function tenantLocalPath(): string {
  const base = tenantBasePath()
  const path = window.location.pathname.slice(base.length)
  return path === '' ? '/' : path
}

export function tenantURL(path: string): string {
  return `${tenantBasePath()}${path}`
}

export function validAdminReturnTo(returnTo: string): boolean {
  if (!returnTo.startsWith('/') || returnTo.includes('\\')) return false
  const parsed = new URL(returnTo, window.location.origin)
  const adminRoot = tenantURL('/admin')
  return (
    parsed.origin === window.location.origin &&
    (parsed.pathname === adminRoot || parsed.pathname.startsWith(`${adminRoot}/`))
  )
}

function base64URL(value: Uint8Array) {
  let binary = ''
  for (const byte of value) {
    binary += String.fromCharCode(byte)
  }
  return btoa(binary).replaceAll('+', '-').replaceAll('/', '_').replaceAll('=', '')
}

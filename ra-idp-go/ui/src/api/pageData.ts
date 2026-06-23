import type {
  AccountActivityPage,
  AccountApplicationsPage,
  AccountDataPage,
  AccountEmailsPage,
  AccountHomePage,
  AccountProfile,
  AccountProfilePage,
  AccountSecurityPage,
  AdminAgent,
  AdminAgentDetailPage,
  AdminAgentsPage,
  AdminAuditEvent,
  AdminAuditEventsPage,
  AdminClient,
  AdminClientDetailPage,
  AdminClientsPage,
  AdminConsent,
  AdminConsentsPage,
  AdminDashboardPage,
  AdminGroup,
  AdminGroupDetailPage,
  AdminGroupsPage,
  AdminKey,
  AdminKeysPage,
  AdminRole,
  AdminRoleDetailPage,
  AdminRolesPage,
  AdminSettings,
  AdminSettingsPage,
  AdminTenant,
  AdminTenantAttributesPage,
  AdminTenantsPage,
  AdminUser,
  AdminUserDetailPage,
  AdminUsersPage,
  CallbackPage,
  ChangePasswordPage,
  ConsentDetailView,
  ConsentPage,
  DevicePage,
  EmailVerifyPage,
  ForgotPasswordPage,
  HomePage,
  LoginPage,
  PageData,
  ResetPasswordPage,
  StatusPage,
  TenantUserAttributeSchema,
  TotpPage,
} from '../types'
import {
  getAccountSecurity,
  getAccountSummary,
  getSignInActivity,
  listAccountConsents,
  listAccountSessions,
} from './account'
import {
  getAdminAgent,
  getAdminClient,
  getAdminGroup,
  getAdminUser,
  listAdminAuditEvents,
} from './admin'
import {
  AuthenticationAPIError,
  request,
  tenantLocalPath,
  tenantURL,
  UnauthenticatedError,
  validAdminReturnTo,
} from './core'

type TransactionResponse = {
  kind: 'login' | 'totp' | 'consent'
  csrf_token: string
  client_name?: string
  scopes?: string[]
  authorization_details?: ConsentDetailView[]
}
type DeviceResponse = { kind: 'device'; csrf_token: string; user_code: string }
type AccountContextResponse = {
  csrf_token: string
  sub: string
  preferred_username?: string
  tenant_id?: string
  roles?: string[]
}
type PasswordResetContextResponse = { csrf_token: string }
type AdminUserListResponse = { users: AdminUser[] }
type AdminClientListResponse = { clients: AdminClient[] }
type AdminConsentListResponse = { consents: AdminConsent[] }
type AdminAuditEventListResponse = { events: AdminAuditEvent[] }
type AdminKeyListResponse = { keys: AdminKey[] }
type AdminTenantListResponse = { tenants: AdminTenant[] }
type AdminRoleListResponse = { roles: AdminRole[] }

function hasAdminRole(roles: string[] | undefined): boolean {
  return (roles ?? []).some((role) => role === 'admin' || role === 'system_admin')
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
  // end-user account portal も認証必須。未認証なら login へ誘導し戻り先を保持する
  // (wi-18 と同じ pattern)。一度取得したコンテキストは各 /account/* 分岐で再利用する。
  // ただしメール変更の検証ページはメールのリンクから (未ログインで) 開かれうるため除外する。
  let accountContext: AccountContextResponse | undefined
  if (path !== '/account/email/verify' && (path === '/account' || path.startsWith('/account/'))) {
    try {
      accountContext = await request<AccountContextResponse>('/api/auth/account')
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
  if (path === '/account') {
    const account = accountContext!
    const summary = await getAccountSummary()
    return {
      kind: 'account-home',
      summary,
      isAdmin: hasAdminRole(account.roles),
    } satisfies AccountHomePage
  }
  if (path === '/account/emails') {
    const account = accountContext!
    const summary = await getAccountSummary()
    return {
      kind: 'account-emails',
      csrfToken: account.csrf_token,
      email: summary.email,
      emailVerified: summary.email_verified,
      isAdmin: hasAdminRole(account.roles),
    } satisfies AccountEmailsPage
  }
  if (path === '/account/email/verify') {
    const ctx = await request<{ csrf_token: string }>('/api/account/email/verify_context')
    const token = new URLSearchParams(window.location.search).get('token') ?? ''
    return { kind: 'email-verify', csrfToken: ctx.csrf_token, token } satisfies EmailVerifyPage
  }
  if (path === '/account/applications') {
    const account = accountContext!
    const consents = await listAccountConsents()
    return {
      kind: 'account-applications',
      csrfToken: account.csrf_token,
      username: account.preferred_username ?? 'account',
      consents,
      isAdmin: hasAdminRole(account.roles),
    } satisfies AccountApplicationsPage
  }
  if (path === '/account/data') {
    const account = accountContext!
    return {
      kind: 'account-data',
      username: account.preferred_username ?? 'account',
      isAdmin: hasAdminRole(account.roles),
    } satisfies AccountDataPage
  }
  if (path === '/account/security') {
    const account = accountContext!
    const security = await getAccountSecurity()
    return {
      kind: 'account-security',
      csrfToken: account.csrf_token,
      username: account.preferred_username ?? 'account',
      isAdmin: hasAdminRole(account.roles),
      security,
    } satisfies AccountSecurityPage
  }
  if (path === '/account/activity') {
    const account = accountContext!
    const [activities, sessions] = await Promise.all([getSignInActivity(), listAccountSessions()])
    return {
      kind: 'account-activity',
      csrfToken: account.csrf_token,
      username: account.preferred_username ?? 'account',
      isAdmin: hasAdminRole(account.roles),
      activities,
      sessions,
    } satisfies AccountActivityPage
  }
  if (path === '/account/password') {
    const data = accountContext!
    return {
      kind: 'change-password',
      csrfToken: data.csrf_token,
      sub: data.sub,
      preferredUsername: data.preferred_username,
      isAdmin: hasAdminRole(data.roles),
    } satisfies ChangePasswordPage
  }
  if (path === '/account/profile') {
    const account = accountContext!
    const profile = await request<AccountProfile>('/api/account/profile')
    return {
      kind: 'account-profile',
      csrfToken: account.csrf_token,
      profile,
      isAdmin: hasAdminRole(account.roles),
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
  const userDetailMatch = path.match(/^\/admin\/users\/([^/]+)$/)
  if (userDetailMatch) {
    const sub = decodeURIComponent(userDetailMatch[1])
    const [user, schema] = await Promise.all([
      getAdminUser(sub),
      request<TenantUserAttributeSchema>('/api/admin/tenant/user_attribute_schema'),
    ])
    return {
      kind: 'admin-user-detail',
      csrfToken: adminAccount!.csrf_token,
      actorUsername: adminAccount!.preferred_username,
      user,
      schema,
    } satisfies AdminUserDetailPage
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
  const roleDetailMatch = path.match(/^\/admin\/roles\/([^/]+)$/)
  if (roleDetailMatch) {
    const name = decodeURIComponent(roleDetailMatch[1])
    const [roles, users] = await Promise.all([
      request<AdminRoleListResponse>('/api/admin/policy/roles'),
      request<AdminUserListResponse>('/api/admin/users'),
    ])
    const role = roles.roles.find((r) => r.name === name)
    if (!role) throw new AuthenticationAPIError('ロールが見つかりません', 'not_found')
    const usernames = users.users
      .filter((u) => u.roles.includes(name))
      .map((u) => u.preferred_username)
    return {
      kind: 'admin-role-detail',
      actorUsername: adminAccount!.preferred_username,
      role,
      count: usernames.length,
      usernames,
    } satisfies AdminRoleDetailPage
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
  const clientDetailMatch = path.match(/^\/admin\/clients\/([^/]+)$/)
  if (clientDetailMatch) {
    const clientID = decodeURIComponent(clientDetailMatch[1])
    const client = await getAdminClient(clientID)
    return {
      kind: 'admin-client-detail',
      csrfToken: adminAccount!.csrf_token,
      actorUsername: adminAccount!.preferred_username,
      client,
    } satisfies AdminClientDetailPage
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
  const groupDetailMatch = path.match(/^\/admin\/groups\/([^/]+)$/)
  if (groupDetailMatch) {
    const id = decodeURIComponent(groupDetailMatch[1])
    const { group } = await getAdminGroup(id)
    return {
      kind: 'admin-group-detail',
      csrfToken: adminAccount!.csrf_token,
      actorUsername: adminAccount!.preferred_username,
      group,
    } satisfies AdminGroupDetailPage
  }
  if (path === '/admin/agents') {
    const agents = await request<{ agents: AdminAgent[] }>('/api/admin/agents')
    return {
      kind: 'admin-agents',
      csrfToken: adminAccount!.csrf_token,
      actorUsername: adminAccount!.preferred_username,
      agents: agents.agents,
    } satisfies AdminAgentsPage
  }
  const agentDetailMatch = path.match(/^\/admin\/agents\/([^/]+)$/)
  if (agentDetailMatch) {
    const id = decodeURIComponent(agentDetailMatch[1])
    const agent = await getAdminAgent(id)
    return {
      kind: 'admin-agent-detail',
      csrfToken: adminAccount!.csrf_token,
      actorUsername: adminAccount!.preferred_username,
      agent,
    } satisfies AdminAgentDetailPage
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
      authorizationDetails: data.authorization_details ?? [],
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

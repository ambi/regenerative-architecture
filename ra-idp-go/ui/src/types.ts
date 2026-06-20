export type HomePage = {
  kind: 'home'
  demoEnabled: boolean
}

export type LoginPage = {
  kind: 'login'
  csrfToken: string
  returnTo?: string
}

export type TotpPage = {
  kind: 'totp'
  csrfToken: string
  returnTo?: string
}

export type ConsentPage = {
  kind: 'consent'
  csrfToken: string
  clientName: string
  scopes: string[]
}

export type DevicePage = {
  kind: 'device'
  csrfToken: string
  userCode: string
}

export type StatusPage = {
  kind: 'status'
  status: 'approved' | 'denied' | 'signed-out' | 'authentication-required'
}

export type CallbackPage = {
  kind: 'callback'
  code?: string
  error?: string
  errorDescription?: string
}

export type ChangePasswordPage = {
  kind: 'change-password'
  csrfToken: string
  sub: string
  preferredUsername?: string
  isAdmin: boolean
}

export type ForgotPasswordPage = {
  kind: 'forgot-password'
  csrfToken: string
}

export type ResetPasswordPage = {
  kind: 'reset-password'
  csrfToken: string
  token: string
}

export type AdminUser = {
  sub: string
  preferred_username: string
  name?: string
  given_name?: string
  family_name?: string
  email?: string
  email_verified: boolean
  mfa_enrolled: boolean
  roles: string[]
  status?: string
  attributes?: Record<string, AttributeValue>
  required_actions?: string[]
  last_login_at?: string
  password_changed_at?: string
  disabled_at?: string
  created_at: string
  updated_at: string
}

export const REQUIRED_ACTIONS = [
  'update_password',
  'verify_email',
  'configure_totp',
  'update_profile',
  'terms_and_conditions',
] as const

export type RequiredActionValue = (typeof REQUIRED_ACTIONS)[number]

export const REQUIRED_ACTION_LABELS: Record<string, string> = {
  update_password: 'パスワードの変更',
  verify_email: 'メールアドレスの確認',
  configure_totp: '二要素認証の設定',
  update_profile: 'プロフィールの更新',
  terms_and_conditions: '利用規約への同意',
}

// requiredActionLabel は内部値を利用者向けの日本語表示名へ変換する。未知の値でも
// 内部表現をそのまま見せず、一般的な文言にフォールバックする。
export function requiredActionLabel(action: string): string {
  return REQUIRED_ACTION_LABELS[action] ?? 'その他の必須対応'
}

export type AdminUsersPage = {
  kind: 'admin-users'
  csrfToken: string
  actorUsername?: string
  users: AdminUser[]
  attributeDefs: UserAttributeDef[]
}

export type AdminUserDetailPage = {
  kind: 'admin-user-detail'
  csrfToken: string
  actorUsername?: string
  user: AdminUser
  schema: TenantUserAttributeSchema
}

export type AdminDashboardPage = {
  kind: 'admin-dashboard'
  csrfToken: string
  actorUsername?: string
  actorRoles: string[]
  actorTenantID: string
  userCount: number
  activeUserCount: number
  disabledUserCount: number
  clientCount: number
  grantedConsentCount: number
  auditEventCount24h: number
  recentEvents: AdminAuditEvent[]
}

export type AdminClient = {
  tenant_id: string
  client_id: string
  client_name?: string
  client_type: 'public' | 'confidential'
  redirect_uris: string[]
  grant_types: string[]
  response_types: string[]
  token_endpoint_auth_method:
    | 'client_secret_basic'
    | 'client_secret_post'
    | 'private_key_jwt'
    | 'tls_client_auth'
    | 'none'
  scope: string
  jwks_uri?: string
  jwks?: Record<string, unknown>
  tls_client_auth_subject_dn?: string
  id_token_signed_response_alg: string
  require_pushed_authorization_requests: boolean
  dpop_bound_access_tokens: boolean
  fapi_profile: string
  created_at: string
}

export type AdminClientsPage = {
  kind: 'admin-clients'
  csrfToken: string
  actorUsername?: string
  clients: AdminClient[]
}

export type AdminConsent = {
  tenant_id: string
  sub: string
  client_id: string
  scopes: string[]
  state: 'granted' | 'revoked' | 'expired'
  granted_at: string
  expires_at: string
  revoked_at?: string
}

export type AdminConsentsPage = {
  kind: 'admin-consents'
  csrfToken: string
  actorUsername?: string
  consents: AdminConsent[]
}

export type AdminAuditEvent = {
  id: string
  tenant_id: string
  type: string
  occurred_at: string
  payload: Record<string, unknown>
}

export type AdminAuditEventsPage = {
  kind: 'admin-audit-events'
  csrfToken: string
  actorUsername?: string
  actorRoles: string[]
  actorTenantID: string
  events: AdminAuditEvent[]
}

export type AdminKey = {
  kid: string
  alg: string
  active: boolean
  created_at: string
  public_jwk: Record<string, unknown>
}

export type AdminKeysPage = {
  kind: 'admin-keys'
  csrfToken: string
  actorUsername?: string
  actorRoles: string[]
  actorTenantID: string
  keys: AdminKey[]
}

export type AdminGroup = {
  id: string
  tenant_id: string
  name: string
  description?: string
  roles: string[]
  member_count: number
  created_at: string
  updated_at?: string
}

export type AdminGroupMember = {
  user_sub: string
  preferred_username: string
  added_at: string
}

export type AdminUserGroups = {
  groups: AdminGroup[]
  direct_roles: string[]
  group_roles: string[]
  effective_roles: string[]
}

export type AdminGroupsPage = {
  kind: 'admin-groups'
  csrfToken: string
  actorUsername?: string
  groups: AdminGroup[]
}

export type AdminTenant = {
  id: string
  display_name: string
  status: 'active' | 'disabled'
  password_policy_override?: {
    min_length?: number
    max_length?: number
    history_depth?: number
  }
  created_at: string
  updated_at?: string
  disabled_at?: string
}

export type AdminTenantsPage = {
  kind: 'admin-tenants'
  csrfToken: string
  actorUsername?: string
  tenants: AdminTenant[]
}

export type AdminRoleInterface = {
  name: string
  method: string
  path: string
}

export type AdminRolePermission = {
  name: string
  action: string
  description: string
  interfaces: AdminRoleInterface[]
}

export type AdminRole = {
  name: string
  description: string
  aliases: string[]
  permissions: AdminRolePermission[]
}

export type AdminRolesPage = {
  kind: 'admin-roles'
  actorUsername?: string
  roles: AdminRole[]
  users: AdminUser[]
}

export type AdminRoleDetailPage = {
  kind: 'admin-role-detail'
  actorUsername?: string
  role: AdminRole
  count: number
  usernames: string[]
}

export type AdminClientDetailPage = {
  kind: 'admin-client-detail'
  csrfToken: string
  actorUsername?: string
  client: AdminClient
}

export type AdminGroupDetailPage = {
  kind: 'admin-group-detail'
  csrfToken: string
  actorUsername?: string
  group: AdminGroup
}

export type AdminSettings = {
  tenant_id: string
  display_name: string
  password_policy_override?: {
    min_length?: number
    max_length?: number
    history_depth?: number
  }
  password_policy_defaults: {
    min_length: number
    max_length: number
    history_depth: number
  }
}

export type AdminSettingsPage = {
  kind: 'admin-settings'
  csrfToken: string
  actorUsername?: string
  actorRoles: string[]
  actorTenantID: string
  settings: AdminSettings
}

export type AttributeType = 'string' | 'number' | 'boolean' | 'date' | 'string_array'

export type AttrVisibility = 'private' | 'self_readable' | 'admin_readable' | 'claim_exposed'

export type UserAttributeDef = {
  key: string
  label?: string
  type: AttributeType
  multi_valued: boolean
  required: boolean
  editable_by_user: boolean
  claim_name?: string
  oidc_scope?: string
  visibility: AttrVisibility
  pii: boolean
}

export type AttributeValue = {
  type: AttributeType
  string?: string
  number?: number
  boolean?: boolean
  date?: string
  string_array?: string[]
}

export type TenantUserAttributeSchema = {
  tenant_id: string
  attributes: UserAttributeDef[]
  builtin: UserAttributeDef[]
  updated_at: string
}

export type AccountProfile = {
  sub: string
  preferred_username: string
  name?: string
  given_name?: string
  family_name?: string
  email?: string
  email_verified: boolean
  mfa_enrolled: boolean
  status: string
  attributes: Record<string, AttributeValue>
  editable_attributes: UserAttributeDef[]
}

export type AccountProfilePage = {
  kind: 'account-profile'
  csrfToken: string
  profile: AccountProfile
  isAdmin: boolean
}

export type AccountSummary = {
  sub: string
  preferred_username: string
  name?: string
  email?: string
  email_verified: boolean
  mfa_enrolled: boolean
  status: string
  last_login_at?: string
  password_changed_at?: string
  required_actions: string[]
}

export type AccountHomePage = {
  kind: 'account-home'
  summary: AccountSummary
  isAdmin: boolean
}

export type AccountEmailsPage = {
  kind: 'account-emails'
  csrfToken: string
  email?: string
  emailVerified: boolean
  isAdmin: boolean
}

export type EmailVerifyPage = {
  kind: 'email-verify'
  csrfToken: string
  token: string
}

export type AccountConsent = {
  client_id: string
  scopes: string[]
  state: string
  granted_at: string
  expires_at: string
}

export type AccountApplicationsPage = {
  kind: 'account-applications'
  csrfToken: string
  username: string
  consents: AccountConsent[]
  isAdmin: boolean
}

export type AdminTenantAttributesPage = {
  kind: 'admin-tenant-attributes'
  csrfToken: string
  actorUsername?: string
  schema: TenantUserAttributeSchema
}

export type PageData =
  | HomePage
  | LoginPage
  | TotpPage
  | ConsentPage
  | DevicePage
  | StatusPage
  | CallbackPage
  | ChangePasswordPage
  | ForgotPasswordPage
  | ResetPasswordPage
  | AdminDashboardPage
  | AdminUsersPage
  | AdminUserDetailPage
  | AdminClientsPage
  | AdminConsentsPage
  | AdminAuditEventsPage
  | AdminKeysPage
  | AdminTenantsPage
  | AdminGroupsPage
  | AdminGroupDetailPage
  | AdminRolesPage
  | AdminRoleDetailPage
  | AdminClientDetailPage
  | AdminSettingsPage
  | AdminTenantAttributesPage
  | AccountProfilePage
  | AccountHomePage
  | AccountEmailsPage
  | EmailVerifyPage
  | AccountApplicationsPage

export type BrowserFlowResponse = {
  next?: string
  redirect_to?: string
}

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
  email?: string
  email_verified: boolean
  mfa_enrolled: boolean
  roles: string[]
  disabled_at?: string
  created_at: string
  updated_at: string
}

export type AdminUsersPage = {
  kind: 'admin-users'
  csrfToken: string
  actorUsername?: string
  users: AdminUser[]
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
  requirements: string[]
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
  | AdminClientsPage
  | AdminConsentsPage
  | AdminAuditEventsPage
  | AdminKeysPage
  | AdminTenantsPage
  | AdminGroupsPage
  | AdminRolesPage
  | AdminSettingsPage
  | AdminTenantAttributesPage
  | AccountProfilePage

export type BrowserFlowResponse = {
  next?: string
  redirect_to?: string
}

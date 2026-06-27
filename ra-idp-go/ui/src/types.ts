export type ConsentDetailView = {
  type: string
  description?: string
  summary: string
  lines?: string[]
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

export type ApplicationKind = 'federated' | 'weblink' | 'service'
export type ApplicationStatus = 'active' | 'disabled'
export type ProtocolBindingType = 'oidc' | 'saml' | 'wsfed'

export type ProtocolBinding = {
  type: ProtocolBindingType
  client_id?: string
  wtrealm?: string
}

export type AdminApplication = {
  application_id: string
  name: string
  kind: ApplicationKind
  status: ApplicationStatus
  icon_url?: string
  launch_url?: string
  bindings: ProtocolBinding[]
  created_at: string
  updated_at: string
}

export type ApplicationAssignment = {
  subject_type: 'user' | 'group'
  subject_id: string
  visibility: 'visible' | 'hidden'
  created_at: string
}

// プロトコル設定はアプリ詳細で解決される。OAuth2 client / WS-Fed RP の実設定を映す。
export type ApplicationOidcConfig = {
  client_id: string
  redirect_uris: string[]
  scope: string
}

export type ApplicationWsFedConfig = {
  wtrealm: string
  reply_urls: string[]
  name_id_format: string
  name_id_source: string
}

export type AdminApplicationDetail = {
  application: AdminApplication
  oidc?: ApplicationOidcConfig | null
  wsfed?: ApplicationWsFedConfig | null
}

export type AuthorizationDetailFieldRule = {
  name: string
  semantics: 'set' | 'at_most' | 'enum' | 'exact'
  required?: boolean
  allowed?: string[]
}

export type AuthorizationDetailType = {
  tenant_id: string
  type: string
  description?: string
  schema: { rules: AuthorizationDetailFieldRule[] }
  display_template: string
  state: 'Enabled' | 'Disabled'
  created_at: string
  updated_at: string
}

export type WsFedClaimMappingRule = {
  claim_type: string
  source: 'user_attribute' | 'fixed' | 'nameid'
  source_key?: string
  fixed_value?: string
  required?: boolean
}

export type WsFedNameIdConfiguration = {
  format: string
  source_attribute: string
}

export type WsFedClaimMappingPolicy = {
  name_id: WsFedNameIdConfiguration
  rules?: WsFedClaimMappingRule[]
}

export type EntraFederationProfile = {
  domain: string
  issuer_uri: string
  source_anchor_attribute: string
  immutable_id_attribute: string
  passive_logon_uri?: string
  active_logon_uri?: string
  metadata_exchange_uri?: string
}

export type WsFedTokenType =
  | 'urn:oasis:names:tc:SAML:1.0:assertion'
  | 'urn:oasis:names:tc:SAML:2.0:assertion'

export type WsFedRelyingParty = {
  tenant_id: string
  wtrealm: string
  display_name?: string
  reply_urls: string[]
  audience?: string
  token_type?: WsFedTokenType
  claim_policy: WsFedClaimMappingPolicy
  entra_profile?: EntraFederationProfile
  created_at: string
  updated_at?: string
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

export type AdminAuditEvent = {
  id: string
  tenant_id: string
  type: string
  occurred_at: string
  payload: Record<string, unknown>
}

export type AdminKey = {
  kid: string
  alg: string
  active: boolean
  created_at: string
  public_jwk: Record<string, unknown>
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

export type AdminAgent = {
  id: string
  tenant_id: string
  name: string
  description?: string
  kind: 'autonomous' | 'supervised'
  owner_sub: string
  status: 'active' | 'disabled' | 'killed'
  roles: string[]
  client_ids: string[]
  created_at: string
  updated_at?: string
  disabled_at?: string
  killed_at?: string
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
  readable_attributes: UserAttributeDef[]
  editable_attributes: UserAttributeDef[]
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

export type AccountConsent = {
  client_id: string
  scopes: string[]
  state: string
  granted_at: string
  expires_at: string
}

export type MyApplication = {
  application_id: string
  name: string
  kind: ApplicationKind
  icon_url?: string
  launch_url?: string
}

export type AccountMfaFactor = {
  type: string
  label?: string
  created_at: string
  last_used_at?: string
}

export type AccountSecurity = {
  password_changed_at?: string
  totp_enrolled: boolean
  factors: AccountMfaFactor[]
}

export type TotpEnrollmentStart = {
  secret: string
  otpauth_uri: string
  account_name: string
  issuer: string
}

export type AccountSignInActivity = {
  occurred_at: string
  amr: string[]
}

export type AccountSession = {
  id: string
  current: boolean
  amr: string[]
  acr: string
  started_at: string
  expires_at: string
}

export type BrowserFlowResponse = {
  next?: string
  redirect_to?: string
}

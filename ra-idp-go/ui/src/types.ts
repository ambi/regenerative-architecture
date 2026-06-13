export type HomePage = {
  kind: 'home'
  demoEnabled: boolean
}

export type LoginPage = {
  kind: 'login'
  csrfToken: string
}

export type TotpPage = {
  kind: 'totp'
  csrfToken: string
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
  | AdminUsersPage

export type BrowserFlowResponse = {
  next?: string
  redirect_to?: string
}

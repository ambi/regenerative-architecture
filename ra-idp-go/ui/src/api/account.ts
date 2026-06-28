import type {
  AccountConsent,
  AccountProfile,
  AccountSecurity,
  AccountSession,
  AccountSignInActivity,
  AccountSummary,
  MyApplication,
  PortalCategory,
  TotpEnrollmentStart,
} from '../types'
import { adminRequest, AuthenticationAPIError, request, tenantURL, type APIError } from './core'

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

export async function getAccountSummary(): Promise<AccountSummary> {
  return request<AccountSummary>('/api/account/summary')
}

export async function requestEmailChange(csrfToken: string, newEmail: string): Promise<void> {
  const response = await fetch(tenantURL('/api/account/email/change_request'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
    body: JSON.stringify({ new_email: newEmail }),
    credentials: 'same-origin',
    cache: 'no-store',
  })
  if (response.status === 204) return
  const body = (await response.json().catch(() => ({}))) as APIError
  throw new AuthenticationAPIError(
    body.message ?? 'メールアドレスの変更を要求できませんでした。',
    body.error,
  )
}

export async function exportAccountData(): Promise<unknown> {
  return request<unknown>('/api/account/data_export')
}

export async function listAccountConsents(): Promise<AccountConsent[]> {
  return (await request<{ consents: AccountConsent[] }>('/api/account/consents')).consents
}

export async function revokeAccountConsent(csrfToken: string, clientId: string): Promise<void> {
  const response = await fetch(
    tenantURL(`/api/account/consents/${encodeURIComponent(clientId)}/revoke`),
    {
      method: 'POST',
      headers: { 'X-CSRF-Token': csrfToken },
      credentials: 'same-origin',
      cache: 'no-store',
    },
  )
  if (response.status === 204) return
  const body = (await response.json().catch(() => ({}))) as APIError
  throw new AuthenticationAPIError(body.message ?? 'アクセスを取り消せませんでした。', body.error)
}

export async function getAccountSecurity(): Promise<AccountSecurity> {
  return request<AccountSecurity>('/api/account/security')
}

export async function getSignInActivity(): Promise<AccountSignInActivity[]> {
  return (await request<{ activities: AccountSignInActivity[] }>('/api/account/signin_activity'))
    .activities
}

export async function listAccountSessions(): Promise<AccountSession[]> {
  return (await request<{ sessions: AccountSession[] }>('/api/account/sessions')).sessions
}

export async function revokeAccountSession(csrfToken: string, id: string): Promise<void> {
  const response = await fetch(
    tenantURL(`/api/account/sessions/${encodeURIComponent(id)}/revoke`),
    {
      method: 'POST',
      headers: { 'X-CSRF-Token': csrfToken },
      credentials: 'same-origin',
      cache: 'no-store',
    },
  )
  if (response.status === 204) return
  const body = (await response.json().catch(() => ({}))) as APIError
  throw new AuthenticationAPIError(body.message ?? 'セッションを終了できませんでした。', body.error)
}

export async function revokeOtherAccountSessions(csrfToken: string): Promise<void> {
  const response = await fetch(tenantURL('/api/account/sessions/revoke_others'), {
    method: 'POST',
    headers: { 'X-CSRF-Token': csrfToken },
    credentials: 'same-origin',
    cache: 'no-store',
  })
  if (response.status === 204) return
  const body = (await response.json().catch(() => ({}))) as APIError
  throw new AuthenticationAPIError(
    body.message ?? '他のセッションを終了できませんでした。',
    body.error,
  )
}

// step-up 再認証 (ADR-043 / wi-43)。高 sensitivity 操作が 403 step_up_required を返したら、
// start で利用可能な factor を取得し、complete で password / TOTP を提示して再認証する。
export type StepUpMethod = 'password' | 'totp'

export function isStepUpRequired(cause: unknown): boolean {
  return cause instanceof AuthenticationAPIError && cause.code === 'step_up_required'
}

export async function startStepUp(csrfToken: string): Promise<StepUpMethod[]> {
  const response = await fetch(tenantURL('/api/account/step_up/start'), {
    method: 'POST',
    headers: { 'X-CSRF-Token': csrfToken },
    credentials: 'same-origin',
    cache: 'no-store',
  })
  if (response.ok) {
    return ((await response.json()) as { methods: StepUpMethod[] }).methods
  }
  const body = (await response.json().catch(() => ({}))) as APIError
  throw new AuthenticationAPIError(body.message ?? '再認証を開始できませんでした。', body.error)
}

export async function completeStepUp(
  csrfToken: string,
  method: StepUpMethod,
  credential: string,
): Promise<void> {
  const payload =
    method === 'password' ? { method, password: credential } : { method, code: credential }
  const response = await fetch(tenantURL('/api/account/step_up/complete'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
    body: JSON.stringify(payload),
    credentials: 'same-origin',
    cache: 'no-store',
  })
  if (response.status === 204) return
  const body = (await response.json().catch(() => ({}))) as APIError
  throw new AuthenticationAPIError(body.message ?? '再認証に失敗しました。', body.error)
}

export async function startTotpEnrollment(csrfToken: string): Promise<TotpEnrollmentStart> {
  const response = await fetch(tenantURL('/api/account/mfa/totp/enroll/start'), {
    method: 'POST',
    headers: { 'X-CSRF-Token': csrfToken },
    credentials: 'same-origin',
    cache: 'no-store',
  })
  if (response.ok) return (await response.json()) as TotpEnrollmentStart
  const body = (await response.json().catch(() => ({}))) as APIError
  throw new AuthenticationAPIError(
    body.message ?? '認証アプリの登録を開始できませんでした。',
    body.error,
  )
}

export async function confirmTotpEnrollment(
  csrfToken: string,
  secret: string,
  code: string,
): Promise<void> {
  const response = await fetch(tenantURL('/api/account/mfa/totp/enroll/confirm'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
    body: JSON.stringify({ secret, code }),
    credentials: 'same-origin',
    cache: 'no-store',
  })
  if (response.status === 204) return
  const body = (await response.json().catch(() => ({}))) as APIError
  throw new AuthenticationAPIError(body.message ?? '認証アプリを登録できませんでした。', body.error)
}

export async function removeTotpFactor(csrfToken: string, code: string): Promise<void> {
  const response = await fetch(tenantURL('/api/account/mfa/totp/remove'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
    body: JSON.stringify({ code }),
    credentials: 'same-origin',
    cache: 'no-store',
  })
  if (response.status === 204) return
  const body = (await response.json().catch(() => ({}))) as APIError
  throw new AuthenticationAPIError(body.message ?? '認証アプリを解除できませんでした。', body.error)
}

export async function confirmEmailChange(csrfToken: string, token: string): Promise<void> {
  const response = await fetch(tenantURL('/api/account/email/verify'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
    body: JSON.stringify({ token }),
    credentials: 'same-origin',
    cache: 'no-store',
  })
  if (response.ok) return
  const body = (await response.json().catch(() => ({}))) as APIError
  throw new AuthenticationAPIError(body.message ?? '確認に失敗しました。', body.error)
}

// 利用者ポータルの割当済みアプリ一覧とカテゴリ定義 (wi-69, wi-70)。visible 割当のみ返り、
// categories は管理者定義のセクション見出しを position 昇順で含む。
export type MyPortal = {
  applications: MyApplication[]
  categories: PortalCategory[]
}

export async function listMyApplications(): Promise<MyPortal> {
  const body = await request<{ applications: MyApplication[]; categories: PortalCategory[] }>(
    '/api/account/applications',
  )
  return { applications: body.applications, categories: body.categories ?? [] }
}

// 利用者ごとの手動並び順 (wi-70)。未保存なら空配列が返る。
export async function getMyApplicationOrder(): Promise<string[]> {
  return (await request<{ application_ids: string[] }>('/api/account/applications/order'))
    .application_ids
}

export async function reorderMyApplications(
  csrfToken: string,
  applicationIds: string[],
): Promise<string[]> {
  return (
    await request<{ application_ids: string[] }>(
      '/api/account/applications/order',
      adminRequest(csrfToken, 'PUT', { application_ids: applicationIds }),
    )
  ).application_ids
}

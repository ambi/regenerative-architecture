/**
 * バックエンドが HTML shell に埋め込む page context を読み取る。
 *
 * `<meta name="ra-idp:request-id" content="...">` のような形式で、各ページの
 * 初期 props (request_id / CSRF / consent の client_id・scope など) を伝える。
 * SPA は CSRF Cookie を直接読めない (HttpOnly) ため、サーバ側で埋め込む。
 */
export function readMeta(name: string): string | null {
  if (typeof document === 'undefined') return null
  const tag = document.querySelector<HTMLMetaElement>(`meta[name="${name}"]`)
  return tag?.content ?? null
}

export interface LoginContext {
  requestId: string
  csrf: string
}

export function readLoginContext(): LoginContext {
  return {
    requestId: readMeta('ra-idp:request-id') ?? '',
    csrf: readMeta('ra-idp:csrf') ?? '',
  }
}

export interface ConsentContext {
  requestId: string
  csrf: string
  clientId: string
  clientName: string
  scopes: string[]
}

export function readConsentContext(): ConsentContext {
  const scope = readMeta('ra-idp:scope') ?? ''
  return {
    requestId: readMeta('ra-idp:request-id') ?? '',
    csrf: readMeta('ra-idp:csrf') ?? '',
    clientId: readMeta('ra-idp:client-id') ?? '',
    clientName: readMeta('ra-idp:client-name') ?? '',
    scopes: scope.split(/\s+/).filter(Boolean),
  }
}

export interface TotpContext {
  requestId: string
  csrf: string
  /** no-JS/form fallback の POST /totp が無効コードで戻ってきた場合に true。 */
  invalidPrevious: boolean
}

export function readTotpContext(): TotpContext {
  return {
    requestId: readMeta('ra-idp:request-id') ?? '',
    csrf: readMeta('ra-idp:csrf') ?? '',
    invalidPrevious: readMeta('ra-idp:totp-invalid') === '1',
  }
}

export interface DeviceContext {
  prefillUserCode: string
  csrf: string
}

export function readDeviceContext(): DeviceContext {
  return {
    prefillUserCode: readMeta('ra-idp:user-code') ?? '',
    csrf: readMeta('ra-idp:csrf') ?? '',
  }
}

export interface ChangePasswordContext {
  csrf: string
}

export function readChangePasswordContext(): ChangePasswordContext {
  return {
    csrf: readMeta('ra-idp:csrf') ?? '',
  }
}

export interface ForgotPasswordContext {
  csrf: string
}

export function readForgotPasswordContext(): ForgotPasswordContext {
  return {
    csrf: readMeta('ra-idp:csrf') ?? '',
  }
}

export interface ResetPasswordContext {
  csrf: string
  token: string
}

export function readResetPasswordContext(): ResetPasswordContext {
  return {
    csrf: readMeta('ra-idp:csrf') ?? '',
    token: readMeta('ra-idp:reset-token') ?? '',
  }
}

export interface ErrorContext {
  /** 'logged_out' / 'access_denied' / 'invalid_request' / 'server_error' など。 */
  kind: string
  title: string
  description: string
  /** デバッグ用の補足 (request id 等)。表示は控えめに。 */
  detail?: string
}

export function readErrorContext(): ErrorContext {
  return {
    kind: readMeta('ra-idp:error-kind') ?? 'unknown',
    title: readMeta('ra-idp:error-title') ?? 'エラーが発生しました',
    description: readMeta('ra-idp:error-description') ?? '',
    detail: readMeta('ra-idp:error-detail') ?? undefined,
  }
}

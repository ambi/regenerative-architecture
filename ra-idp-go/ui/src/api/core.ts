type APIError = {
  error?: string
  message?: string
  error_description?: string
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

// bearerTokenProvider は OIDC RP モジュール (api/oidc) が登録する access token 取得関数。
// core → oidc の循環 import を避けるため、依存方向を逆 (oidc が core に登録) にする。
let bearerTokenProvider: () => string | null = () => null

export function setBearerTokenProvider(provider: () => string | null) {
  bearerTokenProvider = provider
}

export async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const token = bearerTokenProvider()
  const headers = token
    ? { ...(init?.headers ?? {}), Authorization: `Bearer ${token}` }
    : init?.headers
  const response = await fetch(tenantURL(url), {
    credentials: 'same-origin',
    cache: 'no-store',
    ...init,
    ...(headers ? { headers } : {}),
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

export function adminRequest(csrfToken: string, method: string, body?: unknown): RequestInit {
  return {
    method,
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': csrfToken,
    },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  }
}

export function tenantBasePath(path = window.location.pathname): string {
  const match = path.match(/^\/realms\/([a-z0-9][a-z0-9-]{0,62})(?:\/|$)/)
  return match ? `/realms/${match[1]}` : ''
}

export function tenantLocalPath(): string {
  const base = tenantBasePath()
  const path = window.location.pathname.slice(base.length)
  return path === '' ? '/' : path
}

export function tenantURL(path: string): string {
  return `${tenantBasePath()}${path}`
}

// validReturnTo は login 後に戻ってよい同一オリジンの内部パスかを判定する。
// 管理 UI (/admin 配下) と WS-Federation passive エンドポイント (/wsfed) を許可する (wi-61)。
export function validReturnTo(returnTo: string): boolean {
  if (!returnTo.startsWith('/') || returnTo.includes('\\')) return false
  const parsed = new URL(returnTo, window.location.origin)
  if (parsed.origin !== window.location.origin) return false
  const adminRoot = tenantURL('/admin')
  const wsfedPath = tenantURL('/wsfed')
  return (
    parsed.pathname === adminRoot ||
    parsed.pathname.startsWith(`${adminRoot}/`) ||
    parsed.pathname === wsfedPath
  )
}

export function base64URL(value: Uint8Array) {
  let binary = ''
  for (const byte of value) {
    binary += String.fromCharCode(byte)
  }
  return btoa(binary).replaceAll('+', '-').replaceAll('/', '_').replaceAll('=', '')
}

export type { APIError }

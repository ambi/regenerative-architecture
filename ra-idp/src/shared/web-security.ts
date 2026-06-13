import { randomUUID } from 'crypto'

export const CSRF_COOKIE = 'ra_idp_csrf'

export class WebSecurityError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'WebSecurityError'
  }
}

export function createCsrfToken(): string {
  return randomUUID()
}

export function assertCsrf(cookieHeader: string | undefined, submitted: string): void {
  const expected = parseCookies(cookieHeader)[CSRF_COOKIE]
  if (!expected || !submitted || expected !== submitted) {
    throw new WebSecurityError('CSRF トークンが不正です')
  }
}

export function csrfCookie(csrf: string, path = '/'): string {
  return `${CSRF_COOKIE}=${encodeURIComponent(csrf)}; Path=${path}; HttpOnly; SameSite=Lax; Max-Age=600`
}

export function sessionCookie(
  name: string,
  sessionId: string,
  ttlSeconds: number,
  path = '/',
): string {
  return `${name}=${encodeURIComponent(sessionId)}; Path=${path}; HttpOnly; SameSite=Lax; Max-Age=${ttlSeconds}`
}

export function validatedAdminReturnTo(
  value: string | undefined,
  adminBase: string,
): string | undefined {
  if (!value || value.includes('\\')) return undefined
  let decoded: string
  try {
    decoded = decodeURIComponent(value)
  } catch {
    return undefined
  }
  const target = new URL(decoded, 'http://ra-idp.local')
  if (target.origin !== 'http://ra-idp.local') return undefined
  if (target.pathname !== adminBase && !target.pathname.startsWith(`${adminBase}/`)) {
    return undefined
  }
  return `${target.pathname}${target.search}${target.hash}`
}

export function clearCookie(name: string): string {
  return `${name}=; Path=/; HttpOnly; SameSite=Lax; Max-Age=0`
}

function parseCookies(header: string | undefined): Record<string, string> {
  const cookies: Record<string, string> = {}
  if (!header) return cookies
  for (const part of header.split(';')) {
    const [name, ...rest] = part.trim().split('=')
    if (!name) continue
    cookies[name] = decodeURIComponent(rest.join('='))
  }
  return cookies
}

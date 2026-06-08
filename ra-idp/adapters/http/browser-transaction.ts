import type { AuthorizationRequestStore } from '../../src/oauth2/ports/authorization-store'

export const AUTHORIZATION_TRANSACTION_COOKIE = 'ra_idp_transaction'

export function transactionCookie(requestId: string): string {
  return `${AUTHORIZATION_TRANSACTION_COOKIE}=${encodeURIComponent(requestId)}; Path=/; HttpOnly; SameSite=Lax; Max-Age=600`
}

export function clearTransactionCookie(): string {
  return `${AUTHORIZATION_TRANSACTION_COOKIE}=; Path=/; HttpOnly; SameSite=Lax; Max-Age=0`
}

export function transactionIdFromCookie(cookieHeader: string | undefined): string {
  return parseCookies(cookieHeader)[AUTHORIZATION_TRANSACTION_COOKIE] ?? ''
}

export async function transactionRequest(
  store: AuthorizationRequestStore,
  cookieHeader: string | undefined,
) {
  const requestId = transactionIdFromCookie(cookieHeader)
  if (!requestId) return null
  const request = await store.find(requestId)
  if (!request) return null
  if (Date.parse(request.expires_at) <= Date.now()) return null
  return request
}

export function noStoreJSON(_c: unknown, status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      'content-type': 'application/json; charset=UTF-8',
      'cache-control': 'no-store',
    },
  })
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

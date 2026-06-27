// 管理コンソール / アカウントポータルを、自分自身の IdP の OIDC RP として動かす
// 最小クライアント (ADR-061 / wi-66)。authorization_code + PKCE で IdP の /authorize に
// リダイレクトし、/callback で code を /token に交換して access token を取得する。
// pure SPA RP のため token は sessionStorage に保持し、API 呼び出しに Bearer を付与する。
import { base64URL, setBearerTokenProvider, tenantBasePath, tenantURL } from './core'

export type PortalAudience = 'admin' | 'account'

type PortalConfig = { clientId: string; scope: string }

// offline_access を含めることで、access token 失効時に /authorize の全画面往復ではなく
// バックグラウンドの refresh_token grant で更新できる (silent renew, wi-66 Stage 4)。
const PORTALS: Record<PortalAudience, PortalConfig> = {
  admin: { clientId: 'ra-admin-console', scope: 'openid profile ra.admin offline_access' },
  account: { clientId: 'ra-account-portal', scope: 'openid profile ra.account offline_access' },
}

type StoredSession = { accessToken: string; refreshToken?: string; expiresAt: number }
type LoginState = { state: string; verifier: string; audience: PortalAudience; returnTo: string }

const LOGIN_KEY = 'ra_oidc_login'
const sessionKey = (audience: PortalAudience) => `ra_oidc_token_${audience}`
// access token 失効の手前で再取得するためのスキュー (秒)。
const EXPIRY_SKEW_SECONDS = 30

// activeBearer は現在のページが提示すべき access token。route loader が API を呼ぶ前に
// ensureLoggedIn で設定し、request() が同期的に読み出して Authorization に付与する。
let activeBearer: string | null = null

export function currentBearer(): string | null {
  return activeBearer
}

// request() (api/core) が Authorization に付与する access token を登録する。
setBearerTokenProvider(currentBearer)

function redirectURI(): string {
  return `${window.location.origin}${tenantBasePath()}/callback`
}

function randomToken(bytes = 32): string {
  return base64URL(crypto.getRandomValues(new Uint8Array(bytes)))
}

async function pkceChallenge(verifier: string): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier))
  return base64URL(new Uint8Array(digest))
}

function readSession(audience: PortalAudience): StoredSession | null {
  const raw = sessionStorage.getItem(sessionKey(audience))
  if (!raw) return null
  try {
    return JSON.parse(raw) as StoredSession
  } catch {
    return null
  }
}

function writeSession(audience: PortalAudience, session: StoredSession) {
  sessionStorage.setItem(sessionKey(audience), JSON.stringify(session))
}

function clearSession(audience: PortalAudience) {
  sessionStorage.removeItem(sessionKey(audience))
}

function isFresh(session: StoredSession): boolean {
  return session.expiresAt - EXPIRY_SKEW_SECONDS * 1000 > Date.now()
}

async function exchange(
  audience: PortalAudience,
  body: Record<string, string>,
): Promise<StoredSession> {
  const response = await fetch(tenantURL('/token'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    cache: 'no-store',
    body: new URLSearchParams(body).toString(),
  })
  if (!response.ok) {
    throw new Error(`token endpoint returned ${response.status}`)
  }
  const token = (await response.json()) as {
    access_token: string
    refresh_token?: string
    expires_in?: number
  }
  const session: StoredSession = {
    accessToken: token.access_token,
    refreshToken: token.refresh_token,
    expiresAt: Date.now() + (token.expires_in ?? 600) * 1000,
  }
  writeSession(audience, session)
  return session
}

async function refresh(audience: PortalAudience, refreshToken: string): Promise<StoredSession> {
  return exchange(audience, {
    grant_type: 'refresh_token',
    refresh_token: refreshToken,
    client_id: PORTALS[audience].clientId,
    scope: PORTALS[audience].scope,
  })
}

// beginLogin は PKCE 値を生成して login state を保存し、/authorize へリダイレクトする。
// 戻らない (リダイレクト) ので呼び出し側は never として扱う。
async function beginLogin(audience: PortalAudience, returnTo: string): Promise<never> {
  const verifier = randomToken()
  const state = randomToken(16)
  const nonce = randomToken(16)
  const login: LoginState = { state, verifier, audience, returnTo }
  sessionStorage.setItem(LOGIN_KEY, JSON.stringify(login))
  const params = new URLSearchParams({
    response_type: 'code',
    client_id: PORTALS[audience].clientId,
    redirect_uri: redirectURI(),
    scope: PORTALS[audience].scope,
    state,
    nonce,
    code_challenge: await pkceChallenge(verifier),
    code_challenge_method: 'S256',
  })
  window.location.assign(`${tenantURL('/authorize')}?${params.toString()}`)
  return new Promise<never>(() => {})
}

// ensureLoggedIn は audience の有効な access token を保証する。新鮮なトークンが
// あれば activeBearer に設定して返り、refresh 可能なら更新し、いずれも無ければ
// /authorize へリダイレクトする (戻らない)。
export async function ensureLoggedIn(audience: PortalAudience, returnTo: string): Promise<void> {
  const session = readSession(audience)
  if (session && isFresh(session)) {
    activeBearer = session.accessToken
    return
  }
  if (session?.refreshToken) {
    try {
      const renewed = await refresh(audience, session.refreshToken)
      activeBearer = renewed.accessToken
      return
    } catch {
      clearSession(audience)
    }
  }
  await beginLogin(audience, returnTo)
}

// completeLoginFromCallback は /callback で stored login state に対応する code を
// /token に交換し、returnTo へ戻す。RP ログインの callback でなければ false を返し、
// 既存のデモ表示 (CallbackPage) にフォールバックさせる。
export async function completeLoginFromCallback(): Promise<boolean> {
  const raw = sessionStorage.getItem(LOGIN_KEY)
  if (!raw) return false
  sessionStorage.removeItem(LOGIN_KEY)
  let login: LoginState
  try {
    login = JSON.parse(raw) as LoginState
  } catch {
    return false
  }
  const params = new URLSearchParams(window.location.search)
  const error = params.get('error')
  if (error) {
    throw new Error(params.get('error_description') ?? error)
  }
  const code = params.get('code')
  const state = params.get('state')
  if (!code || state !== login.state) {
    throw new Error('OIDC callback の state が一致しません')
  }
  await exchange(login.audience, {
    grant_type: 'authorization_code',
    code,
    redirect_uri: redirectURI(),
    client_id: PORTALS[login.audience].clientId,
    code_verifier: login.verifier,
  })
  const target =
    login.returnTo.startsWith('/') && !login.returnTo.includes('\\')
      ? login.returnTo
      : tenantURL('/admin')
  window.location.assign(target)
  return true
}

// logout は token を破棄して end_session 経由でログアウトする。
export async function logout(audience: PortalAudience): Promise<void> {
  const session = readSession(audience)
  clearSession(audience)
  activeBearer = null
  if (session) {
    await fetch(tenantURL('/revoke'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      cache: 'no-store',
      body: new URLSearchParams({
        token: session.refreshToken ?? session.accessToken,
        client_id: PORTALS[audience].clientId,
      }).toString(),
    }).catch(() => {})
  }
  window.location.assign(tenantURL('/end_session'))
}

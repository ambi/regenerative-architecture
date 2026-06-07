import type {
  BrowserFlowResponse,
  ConsentPage,
  CallbackPage,
  DevicePage,
  HomePage,
  LoginPage,
  PageData,
  StatusPage,
  TotpPage,
} from './types'

type TransactionResponse = {
  kind: 'login' | 'totp' | 'consent'
  csrf_token: string
  client_name?: string
  scopes?: string[]
}

type DeviceResponse = {
  kind: 'device'
  csrf_token: string
  user_code: string
}

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

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    credentials: 'same-origin',
    cache: 'no-store',
    ...init,
  })
  const body = (await response.json().catch(() => ({}))) as T & APIError
  if (!response.ok) {
    throw new AuthenticationAPIError(
      body.message ?? body.error_description ?? '認証サービスに接続できませんでした。',
      body.error,
    )
  }
  return body
}

export async function loadPageData(): Promise<PageData> {
  const path = window.location.pathname
  if (path === '/') {
    return { kind: 'home', demoEnabled: import.meta.env.DEV } satisfies HomePage
  }
  if (path === '/status') {
    const state = new URLSearchParams(window.location.search).get('state')
    const supported = ['approved', 'denied', 'signed-out', 'authentication-required'] as const
    const status = supported.find((value) => value === state) ?? 'authentication-required'
    return { kind: 'status', status } satisfies StatusPage
  }
  if (path === '/callback') {
    const parameters = new URLSearchParams(window.location.search)
    return {
      kind: 'callback',
      code: parameters.get('code') ?? undefined,
      error: parameters.get('error') ?? undefined,
      errorDescription: parameters.get('error_description') ?? undefined,
    } satisfies CallbackPage
  }
  if (path === '/device') {
    const userCode = new URLSearchParams(window.location.search).get('user_code') ?? ''
    const data = await request<DeviceResponse>(
      `/api/auth/device?user_code=${encodeURIComponent(userCode)}`,
    )
    return {
      kind: 'device',
      csrfToken: data.csrf_token,
      userCode: data.user_code,
    } satisfies DevicePage
  }

  const data = await request<TransactionResponse>('/api/auth/transaction')
  if (data.kind === 'consent') {
    if (path !== '/consent') {
      window.history.replaceState(null, '', '/consent')
    }
    return {
      kind: 'consent',
      csrfToken: data.csrf_token,
      clientName: data.client_name ?? '',
      scopes: data.scopes ?? [],
    } satisfies ConsentPage
  }
  if (data.kind === 'totp') {
    if (path !== '/totp') {
      window.history.replaceState(null, '', '/totp')
    }
    return { kind: 'totp', csrfToken: data.csrf_token } satisfies TotpPage
  }
  if (path !== '/login') {
    window.history.replaceState(null, '', '/login')
  }
  return { kind: 'login', csrfToken: data.csrf_token } satisfies LoginPage
}

export async function login(
  csrfToken: string,
  username: string,
  password: string,
): Promise<BrowserFlowResponse> {
  return request('/api/auth/login', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': csrfToken,
    },
    body: JSON.stringify({ username, password }),
  })
}

export async function submitConsent(
  csrfToken: string,
  action: 'allow' | 'deny',
): Promise<BrowserFlowResponse> {
  return request('/api/auth/consent', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': csrfToken,
    },
    body: JSON.stringify({ action }),
  })
}

export async function submitTOTP(csrfToken: string, code: string): Promise<BrowserFlowResponse> {
  return request('/api/auth/totp', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': csrfToken,
    },
    body: JSON.stringify({ code }),
  })
}

export async function submitDevice(
  csrfToken: string,
  userCode: string,
  action: 'allow' | 'deny',
): Promise<BrowserFlowResponse> {
  return request('/api/auth/device', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': csrfToken,
    },
    body: JSON.stringify({ user_code: userCode, action }),
  })
}

export function continueBrowserFlow(result: BrowserFlowResponse) {
  const destination = result.redirect_to ?? result.next
  if (!destination) {
    throw new AuthenticationAPIError('認証フローの遷移先がありません。')
  }
  window.location.assign(destination)
}

export async function startDemoAuthorization() {
  const verifierBytes = crypto.getRandomValues(new Uint8Array(32))
  const verifier = base64URL(verifierBytes)
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier))
  const state = base64URL(crypto.getRandomValues(new Uint8Array(16)))
  const nonce = base64URL(crypto.getRandomValues(new Uint8Array(16)))

  sessionStorage.setItem('ra-idp-demo-code-verifier', verifier)
  const parameters = new URLSearchParams({
    response_type: 'code',
    client_id: 'demo-client',
    redirect_uri: `${window.location.origin}/callback`,
    scope: 'openid profile email offline_access',
    state,
    nonce,
    code_challenge: base64URL(new Uint8Array(digest)),
    code_challenge_method: 'S256',
  })
  window.location.assign(`/authorize?${parameters.toString()}`)
}

function base64URL(value: Uint8Array) {
  let binary = ''
  for (const byte of value) {
    binary += String.fromCharCode(byte)
  }
  return btoa(binary).replaceAll('+', '-').replaceAll('/', '_').replaceAll('=', '')
}

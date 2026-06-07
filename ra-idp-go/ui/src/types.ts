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

export type PageData =
  | HomePage
  | LoginPage
  | TotpPage
  | ConsentPage
  | DevicePage
  | StatusPage
  | CallbackPage

export type BrowserFlowResponse = {
  next?: string
  redirect_to?: string
}

export interface BrowserFlowResponse {
  next?: string
  redirect_to?: string
}

export type BrowserTransactionResponse =
  | { kind: 'login'; csrf_token: string }
  | { kind: 'totp'; csrf_token: string }
  | { kind: 'consent'; csrf_token: string; client_name?: string; scopes?: string[] }

export function continueBrowserFlow(result: BrowserFlowResponse): void {
  const destination = result.redirect_to ?? result.next
  if (!destination) {
    throw new Error('認証フローの遷移先がありません')
  }
  window.location.assign(destination)
}

export async function loadBrowserTransaction(): Promise<BrowserTransactionResponse> {
  const response = await fetch('/api/auth/transaction', {
    credentials: 'same-origin',
    cache: 'no-store',
  })
  if (!response.ok) {
    throw new Error('認可トランザクションがありません')
  }
  return (await response.json()) as BrowserTransactionResponse
}

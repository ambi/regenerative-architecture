/**
 * Authentication component boundary.
 *
 * OAuth2/OIDC use cases consume this context and do not inspect password,
 * user lookup, or session-cookie details directly.
 */

export interface AuthenticationContext {
  sub: string
  auth_time: number
  amr: string[]
  acr?: string
  session_id?: string
  /** 追加 factor 検証待ちなら true。/authorize 完了経路は未認証扱いとし対応する factor challenge に誘導する。 */
  authentication_pending?: boolean
}

export interface AuthenticationContextResolver {
  resolve(headers: Headers): Promise<AuthenticationContext | null>
}

export class AuthenticationContextError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'AuthenticationContextError'
  }
}

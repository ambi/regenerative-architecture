import type { AuthenticationContext } from '../domain/authentication-context'

export interface LoginContinuation {
  continueAfterLogin(
    requestId: string,
    context: AuthenticationContext,
    options?: {
      promptLoginSatisfied?: boolean
      /** ロケール選択用。後続ページの SPA shell に伝搬する。 */
      acceptLanguage?: string
    },
  ): Promise<Response>
}

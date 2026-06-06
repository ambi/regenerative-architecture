import type { AuthenticationContext } from '../domain/authentication-context'

export interface LoginContinuation {
  continueAfterLogin(
    requestId: string,
    context: AuthenticationContext,
    options?: { promptLoginSatisfied?: boolean },
  ): Promise<Response>
}

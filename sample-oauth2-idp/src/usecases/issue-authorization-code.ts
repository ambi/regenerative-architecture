/**
 * Layer 3 — Application Logic
 *
 * consented (または authenticated + 既存コンセント) 状態の認可リクエストから
 * 短寿命の認可コードを発行する。
 */

import { generateAuthorizationCode, type AuthorizationCode } from '../domain/authorization-code'
import { advance, type AuthorizationRequest } from '../domain/authorization-request'
import type {
  AuthorizationCodeStore,
  AuthorizationRequestStore,
} from '../ports/authorization-store'

export async function issueAuthorizationCodeUseCase(
  deps: {
    codeStore: AuthorizationCodeStore
    requestStore: AuthorizationRequestStore
  },
  req: AuthorizationRequest,
): Promise<{ code: AuthorizationCode; request: AuthorizationRequest }> {
  const code = generateAuthorizationCode({
    authorization_request_id: req.id,
    client_id: req.client_id,
    sub: req.sub!,
    scopes: req.scope.split(/\s+/).filter(Boolean),
    redirect_uri: req.redirect_uri,
    code_challenge: req.code_challenge,
    code_challenge_method: req.code_challenge_method,
    nonce: req.nonce,
    auth_time: req.auth_time!,
  })
  await deps.codeStore.save(code)

  const advanced = advance(req, 'issue_code')
  await deps.requestStore.save(advanced)
  return { code, request: advanced }
}

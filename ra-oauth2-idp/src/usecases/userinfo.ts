/**
 * Layer 3 — Application Logic
 *
 * /userinfo (OIDC Core §5.3) ロジック。
 *
 * 認可は spec/scl.yaml permissions.userinfo:read に委譲する。
 */

import { evaluate } from '../spec-bindings/policy/client-authorization'
import { OAuthError } from '../domain/errors'
import type { UserRepository } from '../ports/user-repository'

export interface UserInfoInput {
  /** イントロスペクションで確認済みのアクセストークンクレーム。 */
  scopes: string[]
  sub: string
  active: boolean
  client_id: string
}

export interface UserInfoResponse {
  sub: string
  name?: string
  family_name?: string
  given_name?: string
  preferred_username?: string
  email?: string
  email_verified?: boolean
  updated_at?: number
}

export async function userInfoUseCase(
  deps: { userRepo: UserRepository },
  input: UserInfoInput,
): Promise<UserInfoResponse> {
  const decision = evaluate({
    subject: { type: 'Client', id: input.client_id },
    action: { name: 'userinfo:read' },
    resource: {
      type: 'UserInfo',
      properties: { scopes: input.scopes, revoked: !input.active },
    },
  })
  if (decision.decision === 'Deny') {
    if (decision.reasons?.includes('token_has_openid_scope')) {
      throw new OAuthError('insufficient_scope', 'openid スコープが必要です')
    }
    throw new OAuthError('invalid_request', `userinfo 拒否: ${decision.reasons?.join(', ')}`)
  }

  const user = await deps.userRepo.findBySub(input.sub)
  if (!user) {
    throw new OAuthError('invalid_request', 'ユーザーが存在しません')
  }

  const res: UserInfoResponse = { sub: user.sub }

  if (input.scopes.includes('profile')) {
    res.name = user.name
    res.family_name = user.family_name
    res.given_name = user.given_name
    res.preferred_username = user.preferred_username
    res.updated_at = Math.floor(Date.parse(user.updated_at) / 1000)
  }
  if (input.scopes.includes('email')) {
    res.email = user.email
    res.email_verified = user.email_verified
  }

  return res
}

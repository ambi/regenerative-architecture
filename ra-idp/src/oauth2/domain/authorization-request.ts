/**
 * Layer 3 — Application Logic
 *
 * 認可リクエストのドメインモデル。
 * spec/scl.yaml state_machines.AuthorizationCodeFlow に従う。
 */

import { randomUUID } from 'crypto'
import { AuthorizationRequestSchema, type AuthorizationRequest } from '../../spec-bindings/schemas'
import { transitionAuthCode } from '../../spec-bindings/flows/flows'
import type { AuthCodeState, AuthCodeEvent } from '../../spec-bindings/flows/flows'
import { OAuthError } from '../protocol/oauth-error'

export type { AuthorizationRequest }

export interface CreateAuthorizationRequestInput {
  client_id: string
  redirect_uri: string
  response_type: 'code'
  scope: string
  state_param?: string
  nonce?: string
  code_challenge?: string
  code_challenge_method?: 'S256'
  prompt?: string
  max_age?: number
  id_token_hint?: string
  par_request_uri?: string
}

const DEFAULT_TTL_SECONDS = 600

export function createAuthorizationRequest(
  input: CreateAuthorizationRequestInput,
  now: Date = new Date(),
): AuthorizationRequest {
  const expiresAt = new Date(now.getTime() + DEFAULT_TTL_SECONDS * 1000)
  return AuthorizationRequestSchema.parse({
    id: randomUUID(),
    state: 'received',
    ...input,
    created_at: now.toISOString(),
    expires_at: expiresAt.toISOString(),
  })
}

export function advance(
  req: AuthorizationRequest,
  event: AuthCodeEvent,
  patch: Partial<AuthorizationRequest> = {},
): AuthorizationRequest {
  const next = transitionAuthCode(req.state as AuthCodeState, event)
  if (next === null) {
    throw new OAuthError(
      'invalid_request',
      `認可リクエストの状態 ${req.state} に対し ${event} は不正な遷移です`,
    )
  }
  return AuthorizationRequestSchema.parse({
    ...req,
    ...patch,
    state: next,
  })
}

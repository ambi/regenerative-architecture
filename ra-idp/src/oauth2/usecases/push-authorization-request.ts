/**
 * Layer 3 — Application Logic
 *
 * /par (RFC 9126) ロジック。
 * クライアント認証済みの状態で受け取った認可パラメータを保存し、 request_uri を返す。
 */

import { randomBytes } from 'crypto'
import { OAuthError } from '../protocol/oauth-error'
import { PARRecordSchema, type PARRecord } from '../../spec-bindings/schemas'
import type { ClientRepository } from '../ports/client-repository'
import type { PARStore } from '../ports/authorization-store'

const PAR_TTL_SECONDS = 600
const REQUEST_URI_PREFIX = 'urn:ietf:params:oauth:request_uri:'

export interface PARInput {
  client_id: string
  parameters: Record<string, string>
}

export interface PARResponse {
  request_uri: string
  expires_in: number
}

export async function pushAuthorizationRequestUseCase(
  deps: {
    clientRepo: ClientRepository
    parStore: PARStore
  },
  input: PARInput,
  now: Date = new Date(),
): Promise<PARResponse> {
  const client = await deps.clientRepo.findById(input.client_id)
  if (!client) {
    throw new OAuthError('invalid_client', 'クライアント認証に失敗しました')
  }

  if (!input.parameters.code_challenge) {
    throw new OAuthError('invalid_request', 'PKCE code_challenge が必要です')
  }
  if (input.parameters.client_id && input.parameters.client_id !== input.client_id) {
    throw new OAuthError('invalid_request', 'client_id がクライアント認証と一致しません')
  }
  if (input.parameters.redirect_uri) {
    if (!client.redirect_uris.includes(input.parameters.redirect_uri)) {
      throw new OAuthError('invalid_request', 'redirect_uri が登録されていません')
    }
  }

  const requestUri = REQUEST_URI_PREFIX + randomBytes(32).toString('base64url')
  const record: PARRecord = PARRecordSchema.parse({
    request_uri: requestUri,
    client_id: input.client_id,
    parameters: input.parameters,
    issued_at: now.toISOString(),
    expires_at: new Date(now.getTime() + PAR_TTL_SECONDS * 1000).toISOString(),
    used: false,
  })
  await deps.parStore.save(record)

  return { request_uri: requestUri, expires_in: PAR_TTL_SECONDS }
}

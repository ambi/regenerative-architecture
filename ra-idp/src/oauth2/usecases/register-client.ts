/**
 * Layer 3 — Application Logic
 *
 * Dynamic Client Registration (RFC 7591) の中核。
 * 本アプリでは「管理者によるクライアント登録」のみを実装し、公開エンドポイントとしては
 * 開示しない（本番ではトークン認証付きエンドポイントとして保護する）。
 */

import { randomBytes, randomUUID, createHash } from 'crypto'
import { ClientSchema, type Client } from '../../spec-bindings/schemas'
import { OAuthError } from '../protocol/oauth-error'
import type { ClientRepository } from '../ports/client-repository'

export interface RegisterClientInput {
  client_name?: string
  client_type: 'public' | 'confidential'
  redirect_uris: string[]
  grant_types?: Client['grant_types']
  response_types?: Client['response_types']
  token_endpoint_auth_method?: Client['token_endpoint_auth_method']
  scope?: string
  /** private_key_jwt クライアントの公開鍵 (インライン JWKS)。 */
  jwks?: Record<string, unknown>
  /** private_key_jwt クライアントの JWKS エンドポイント。 */
  jwks_uri?: string
  require_pushed_authorization_requests?: boolean
  dpop_bound_access_tokens?: boolean
  fapi_profile?: Client['fapi_profile']
}

export interface RegisterClientResult {
  client: Client
  /** 機密クライアントには発行時に一度だけ平文の client_secret を返す。 */
  client_secret?: string
}

export async function registerClientUseCase(
  deps: { clientRepo: ClientRepository },
  input: RegisterClientInput,
  now: Date = new Date(),
): Promise<RegisterClientResult> {
  const client_id = `cli_${randomUUID().replace(/-/g, '')}`
  const authMethod =
    input.token_endpoint_auth_method ??
    (input.client_type === 'confidential' ? 'client_secret_basic' : 'none')

  // private_key_jwt は登録時に公開鍵を要求する (RFC 7591 §2 / ADR-008)。
  // 鍵のないクライアントは認証経路で必ず失敗するため、登録時点で弾く。
  if (authMethod === 'private_key_jwt' && !input.jwks && !input.jwks_uri) {
    throw new OAuthError(
      'invalid_request',
      'private_key_jwt クライアントは jwks または jwks_uri が必要です',
    )
  }

  // client_secret は secret ベース認証方式のときだけ発行する。
  const usesSecret = authMethod === 'client_secret_basic' || authMethod === 'client_secret_post'
  let client_secret: string | undefined
  let client_secret_hash: string | undefined
  if (input.client_type === 'confidential' && usesSecret) {
    client_secret = randomBytes(32).toString('base64url')
    client_secret_hash = createHash('sha256').update(client_secret).digest('hex')
  }
  const client = ClientSchema.parse({
    client_id,
    client_secret_hash,
    client_name: input.client_name,
    client_type: input.client_type,
    redirect_uris: input.redirect_uris,
    grant_types: input.grant_types ?? ['authorization_code', 'refresh_token'],
    response_types: input.response_types ?? ['code'],
    token_endpoint_auth_method: authMethod,
    jwks: input.jwks,
    jwks_uri: input.jwks_uri,
    scope: input.scope ?? 'openid profile email',
    require_pushed_authorization_requests: input.require_pushed_authorization_requests ?? false,
    dpop_bound_access_tokens: input.dpop_bound_access_tokens ?? false,
    fapi_profile: input.fapi_profile ?? 'none',
    created_at: now.toISOString(),
  })
  await deps.clientRepo.save(client)
  return { client, client_secret }
}

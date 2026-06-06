/**
 * Layer 4 — Adapter Layer（エラーレスポンス）
 *
 * OAuth エラー → HTTP ステータスへのマッピング。
 * RFC 6749 §5.2 ほか各 RFC に従う。
 */

import type { Context } from 'hono'
import { OAuthError, type OAuthErrorCode } from '../../src/oauth2/protocol/oauth-error'

type HttpStatus = 400 | 401 | 403 | 404 | 500 | 503

const STATUS: Record<OAuthErrorCode, HttpStatus> = {
  invalid_request: 400,
  unauthorized_client: 400,
  access_denied: 400,
  unsupported_response_type: 400,
  invalid_scope: 400,
  server_error: 500,
  temporarily_unavailable: 503,
  invalid_client: 401,
  invalid_grant: 400,
  unsupported_grant_type: 400,
  invalid_pkce: 400,
  invalid_request_uri: 400,
  invalid_dpop_proof: 400,
  use_dpop_nonce: 400,
  authorization_pending: 400,
  slow_down: 400,
  expired_token: 400,
  insufficient_scope: 403,
  not_found: 404,
}

export function oauthErrorResponse(c: Context, err: OAuthError): Response {
  const status: HttpStatus = STATUS[err.code] ?? 400
  return c.json(
    {
      error: err.code,
      error_description: err.description ?? err.message,
    },
    status,
  )
}

export function tryRespond<T>(c: Context, fn: () => Promise<T>): Promise<T | Response> {
  return fn().catch((e) => {
    if (e instanceof OAuthError) return oauthErrorResponse(c, e)
    throw e
  })
}

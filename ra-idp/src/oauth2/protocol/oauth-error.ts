/**
 * Layer 3 — Application Logic
 *
 * 型付きドメインエラー。OAuth2 / OIDC の標準エラーコード（RFC 6749 §5.2 ほか）に揃える。
 * アダプター層が HTTP ステータスコードに kind でマッピングできる。
 */

export type OAuthErrorCode =
  // 認可エンドポイント (RFC 6749 §4.1.2.1)
  | 'invalid_request'
  | 'unauthorized_client'
  | 'access_denied'
  | 'unsupported_response_type'
  | 'invalid_scope'
  | 'server_error'
  | 'temporarily_unavailable'
  // トークンエンドポイント (RFC 6749 §5.2)
  | 'invalid_client'
  | 'invalid_grant'
  | 'unsupported_grant_type'
  // PKCE (RFC 7636)
  | 'invalid_pkce'
  // PAR (RFC 9126)
  | 'invalid_request_uri'
  // DPoP (RFC 9449)
  | 'invalid_dpop_proof'
  | 'use_dpop_nonce'
  // Device Authorization Grant (RFC 8628 §3.5)
  | 'authorization_pending'
  | 'slow_down'
  | 'expired_token'
  // UserInfo (OIDC Core §5.3.3) / Bearer Token Usage (RFC 6750 §3.1)
  | 'insufficient_scope'
  | 'invalid_token'
  // 内部
  | 'not_found'

export class OAuthError extends Error {
  constructor(
    readonly code: OAuthErrorCode,
    message: string,
    readonly description?: string,
  ) {
    super(message)
    this.name = 'OAuthError'
  }
}

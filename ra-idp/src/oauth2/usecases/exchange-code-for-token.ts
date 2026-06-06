/**
 * Layer 3 — Application Logic
 *
 * /token 認可コードグラントの中核ユースケース。
 *
 * - 認可コードの存在・期限・redirect_uri 一致を確認
 * - PKCE verifier を検証
 * - クライアントが宣言した grant_type を保持しているか確認
 * - 認可コードを atomically redeemed にする（並行リプレイ防止）
 * - リプレイ検出時は同コード由来の refresh token ファミリーを失効させる
 *   （RFC 9700 §4.10、ADR-004）
 * - access_token (JWT) + refresh_token (opaque) + id_token (JWT) を発行
 */

import { OAuthError } from '../protocol/oauth-error'
import { verifyPkce } from '../domain/pkce'
import { isExpired, isRedeemed } from '../domain/authorization-code'
import { generateInitial } from '../domain/refresh-token'
import { evaluate } from '../../spec-bindings/policy/client-authorization'
import type { AuthorizationCodeStore } from '../ports/authorization-store'
import type { ClientRepository } from '../ports/client-repository'
import type { UserRepository } from '../../authentication/ports/user-repository'
import type { RefreshTokenStore } from '../ports/refresh-token-store'
import type { TokenIssuer } from '../ports/token-issuer'

export interface ExchangeCodeInput {
  client_id: string
  code: string
  code_verifier: string
  redirect_uri: string
  dpop_jkt?: string
}

/**
 * RFC 6749 §5.1 のトークン応答 + 監査用の sub / jti。
 * 監査用フィールドはアダプター層で取り出し、HTTP 応答には含めない。
 */
export interface ExchangeCodeResult {
  /** RFC 6749 §5.1 のトークン応答ボディに含めるフィールド。 */
  response: {
    access_token: string
    token_type: 'Bearer' | 'DPoP'
    expires_in: number
    refresh_token?: string
    id_token?: string
    scope: string
  }
  /** 監査ログ用（HTTP 応答には含めない）。 */
  audit: {
    sub: string
    jti: string
    scopes: string[]
    senderConstraint: 'none' | 'dpop' | 'mtls'
    refreshTokenId?: string
    refreshFamilyId?: string
  }
}

export async function exchangeCodeForTokenUseCase(
  deps: {
    clientRepo: ClientRepository
    userRepo: UserRepository
    codeStore: AuthorizationCodeStore
    refreshStore: RefreshTokenStore
    tokenIssuer: TokenIssuer
  },
  input: ExchangeCodeInput,
  now: Date = new Date(),
): Promise<ExchangeCodeResult> {
  const client = await deps.clientRepo.findById(input.client_id)
  if (!client) {
    throw new OAuthError('invalid_client', 'クライアント認証に失敗しました')
  }

  const code = await deps.codeStore.find(input.code)
  if (!code) {
    throw new OAuthError('invalid_grant', '認可コードが無効です')
  }

  // コードと提示クライアントの一致確認（RFC 6749 §4.1.3）
  if (code.client_id !== client.client_id) {
    throw new OAuthError('invalid_grant', '認可コードがクライアントと一致しません')
  }

  // 並行リプレイ・再利用の即時検知:
  // 既に redeemed なら、過去に発行されたファミリーを失効させる（RFC 9700 §4.10）
  if (isRedeemed(code) || isExpired(code, now)) {
    if (code.issued_family_id) {
      await deps.refreshStore.revokeFamily(code.issued_family_id)
    }
    throw new OAuthError('invalid_grant', '認可コードはすでに使用済みまたは期限切れです')
  }

  // PKCE 検証
  if (!verifyPkce(input.code_verifier, code.code_challenge, code.code_challenge_method)) {
    throw new OAuthError('invalid_grant', 'PKCE 検証に失敗しました')
  }

  // ポリシー評価（認可ポリシーがすべての制約を一括チェック）
  const decision = evaluate({
    subject: {
      type: 'Client',
      id: client.client_id,
      properties: {
        grantTypes: client.grant_types,
      },
    },
    action: { name: 'token:grant_authorization_code' },
    resource: {
      type: 'AuthorizationCode',
      properties: {
        codeChallenge: code.code_challenge,
        redirectUri: code.redirect_uri,
        redeemed: false,
        expiresAt: code.expires_at,
      },
    },
    context: {
      codeVerifier: input.code_verifier,
      redirectUri: input.redirect_uri,
      now: now.toISOString(),
    },
  })
  if (decision.decision === 'Deny') {
    throw new OAuthError(
      'invalid_grant',
      `認可コード交換が拒否されました: ${decision.reasons?.join(', ')}`,
    )
  }

  // 並行交換を防ぐため atomic に redeem
  const redeemed = await deps.codeStore.redeem(input.code, now)
  if (!redeemed) {
    // 並行リクエストに敗北 → このコードから発行されたファミリーがすでにあれば失効
    if (code.issued_family_id) {
      await deps.refreshStore.revokeFamily(code.issued_family_id)
    }
    throw new OAuthError('invalid_grant', '認可コードは並行リクエストにより使用済みです')
  }

  const user = await deps.userRepo.findBySub(code.sub)
  if (!user) {
    throw new OAuthError('server_error', 'ユーザーが存在しません')
  }

  // access_token (JWT) 発行
  const senderConstraint = input.dpop_jkt ? { type: 'dpop' as const, jkt: input.dpop_jkt } : null
  const { token: access_token, jti } = await deps.tokenIssuer.signAccessToken({
    client,
    sub: code.sub,
    scopes: code.scopes,
    senderConstraint,
    authTime: code.auth_time,
  })

  // OIDC Core §11: refresh_token は offline_access 付与時のみ発行する。
  let refresh_token: string | undefined
  let refreshTokenId: string | undefined
  let refreshFamilyId: string | undefined
  if (code.scopes.includes('offline_access')) {
    const { token, record } = generateInitial({
      client_id: client.client_id,
      sub: code.sub,
      scopes: code.scopes,
      sender_constraint: senderConstraint,
      now,
    })
    refresh_token = token
    refreshTokenId = record.id
    refreshFamilyId = record.family_id
    await deps.refreshStore.save(record)
    await deps.codeStore.linkFamily(redeemed.code, record.family_id)
  }

  // id_token (JWT) 発行（openid スコープ時のみ）
  let id_token: string | undefined
  if (code.scopes.includes('openid')) {
    id_token = await deps.tokenIssuer.signIdToken({
      client,
      user,
      scopes: code.scopes,
      nonce: code.nonce,
      authTime: code.auth_time,
      atHashFor: access_token,
    })
  }

  return {
    response: {
      access_token,
      token_type: senderConstraint ? 'DPoP' : 'Bearer',
      expires_in: deps.tokenIssuer.getAccessTokenTtlSeconds(),
      ...(refresh_token ? { refresh_token } : {}),
      id_token,
      scope: code.scopes.join(' '),
    },
    audit: {
      sub: code.sub,
      jti,
      scopes: code.scopes,
      senderConstraint: senderConstraint ? 'dpop' : 'none',
      refreshTokenId,
      refreshFamilyId,
    },
  }
}

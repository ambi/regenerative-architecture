/**
 * Layer 3 — Application Logic（ポート定義）
 *
 * JWT 署名は暗号ライブラリ（jose）に依存するため、ユースケース層は
 * このポート越しに「access_token / id_token に署名する」操作だけを使う。
 *
 * 実装は adapters/crypto/jwt-signer.ts。
 */

import type { Client, User } from '../../spec-bindings/schemas'

export interface SignAccessTokenInput {
  client: Client
  sub: string
  scopes: string[]
  senderConstraint: { type: 'dpop'; jkt: string } | { type: 'mtls'; 'x5t#S256': string } | null
  authTime: number
}

export interface SignIdTokenInput {
  client: Client
  user: User
  scopes: string[]
  nonce?: string
  authTime: number
  atHashFor: string
}

export interface TokenIssuer {
  signAccessToken(input: SignAccessTokenInput): Promise<{ token: string; jti: string }>
  signIdToken(input: SignIdTokenInput): Promise<string>
  getAccessTokenTtlSeconds(): number
  getIdTokenTtlSeconds(): number
}

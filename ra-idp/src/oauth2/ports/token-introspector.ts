/**
 * Layer 3 — Application Logic（ポート定義）
 *
 * アクセストークン (JWT) の検証 + イントロスペクション。
 * 実装は adapters/crypto/jwt-signer.ts に近接して置く。
 */

import type { IntrospectionResponse } from '../usecases/introspect-token'

export interface TokenIntrospector {
  introspectAccessToken(token: string): Promise<IntrospectionResponse>
}

/**
 * Layer 4 — Adapter Layer（JWT 署名・検証）
 *
 * jose を使った JWT 署名 (PS256/ES256 のみ — ADR-003) と検証。
 *
 * TokenIssuer / TokenIntrospector の両ポートを 1 つのクラスで実装する
 * （いずれも鍵管理を必要とするため）。
 */

import { SignJWT, jwtVerify, createLocalJWKSet, errors as joseErrors } from 'jose'
import type { JWK, JSONWebKeySet, KeyLike } from 'jose'
import { randomUUID, createHash } from 'crypto'
import type {
  TokenIssuer,
  SignAccessTokenInput,
  SignIdTokenInput,
} from '../../src/oauth2/ports/token-issuer'
import type { TokenIntrospector } from '../../src/oauth2/ports/token-introspector'
import type { IntrospectionResponse } from '../../src/oauth2/usecases/introspect-token'
import type { KeyStore } from '../../src/oauth2/ports/key-store'

const ACCESS_TOKEN_TTL_SECONDS = 600
const ID_TOKEN_TTL_SECONDS = 3600

export class JoseTokenSigner implements TokenIssuer, TokenIntrospector {
  constructor(
    private readonly issuer: string,
    private readonly keyStore: KeyStore,
  ) {}

  getAccessTokenTtlSeconds(): number {
    return ACCESS_TOKEN_TTL_SECONDS
  }
  getIdTokenTtlSeconds(): number {
    return ID_TOKEN_TTL_SECONDS
  }

  async signAccessToken(input: SignAccessTokenInput): Promise<{ token: string; jti: string }> {
    const key = await this.keyStore.getActiveKey()
    const jti = randomUUID()
    const cnf: Record<string, string> = {}
    if (input.senderConstraint?.type === 'dpop') {
      cnf.jkt = input.senderConstraint.jkt
    } else if (input.senderConstraint?.type === 'mtls') {
      cnf['x5t#S256'] = input.senderConstraint['x5t#S256']
    }

    const token = await new SignJWT({
      sub: input.sub,
      client_id: input.client.client_id,
      scope: input.scopes.join(' '),
      jti,
      auth_time: input.authTime,
      ...(Object.keys(cnf).length > 0 ? { cnf } : {}),
    })
      .setProtectedHeader({ alg: key.alg, kid: key.kid, typ: 'at+jwt' })
      .setIssuer(this.issuer)
      .setAudience(input.client.client_id)
      .setIssuedAt()
      .setExpirationTime(`${ACCESS_TOKEN_TTL_SECONDS}s`)
      .sign(key.privateKey as KeyLike)
    return { token, jti }
  }

  async signIdToken(input: SignIdTokenInput): Promise<string> {
    const key = await this.keyStore.getActiveKey()
    const atHash = computeAtHash(input.atHashFor, key.alg)
    const payload: Record<string, unknown> = {
      auth_time: input.authTime,
      at_hash: atHash,
    }
    if (input.nonce) payload.nonce = input.nonce
    if (input.scopes.includes('profile')) {
      payload.name = input.user.name
      payload.preferred_username = input.user.preferred_username
    }
    if (input.scopes.includes('email')) {
      payload.email = input.user.email
      payload.email_verified = input.user.email_verified
    }
    return new SignJWT(payload)
      .setProtectedHeader({ alg: key.alg, kid: key.kid })
      .setIssuer(this.issuer)
      .setSubject(input.user.sub)
      .setAudience(input.client.client_id)
      .setIssuedAt()
      .setExpirationTime(`${ID_TOKEN_TTL_SECONDS}s`)
      .sign(key.privateKey as KeyLike)
  }

  async introspectAccessToken(token: string): Promise<IntrospectionResponse> {
    const allKeys = await this.keyStore.getAllKeys()
    const jwks: JSONWebKeySet = {
      keys: allKeys.map((k) => ({ ...k.publicJwk }) as unknown as JWK),
    }
    const verifier = createLocalJWKSet(jwks)
    try {
      const { payload } = await jwtVerify(token, verifier, {
        issuer: this.issuer,
        // alg を制限することでアルゴリズム混乱攻撃を防ぐ (ADR-003)
        algorithms: ['PS256', 'ES256'],
      })
      return {
        active: true,
        scope: typeof payload.scope === 'string' ? payload.scope : undefined,
        client_id: typeof payload.client_id === 'string' ? payload.client_id : undefined,
        sub: payload.sub,
        aud: payload.aud as string | string[] | undefined,
        iss: payload.iss,
        exp: payload.exp,
        iat: payload.iat,
        nbf: payload.nbf,
        jti: typeof payload.jti === 'string' ? payload.jti : undefined,
        token_type: 'access_token',
        cnf: payload.cnf as { jkt?: string; 'x5t#S256'?: string } | undefined,
      }
    } catch (e) {
      if (
        e instanceof joseErrors.JWTExpired ||
        e instanceof joseErrors.JWSSignatureVerificationFailed ||
        e instanceof joseErrors.JWTInvalid ||
        e instanceof joseErrors.JWTClaimValidationFailed
      ) {
        return { active: false }
      }
      return { active: false }
    }
  }
}

/** at_hash の計算（OIDC Core §3.1.3.6）: alg の hash の左半分を base64url */
function computeAtHash(accessToken: string, alg: 'PS256' | 'ES256'): string {
  const hash = alg === 'ES256' ? 'sha256' : 'sha256' // PS256 / ES256 とも SHA-256
  const digest = createHash(hash).update(accessToken).digest()
  const half = digest.subarray(0, digest.length / 2)
  return half.toString('base64url')
}

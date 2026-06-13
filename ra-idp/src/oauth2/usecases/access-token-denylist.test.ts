/**
 * Layer 3 — Application Logic (scenario)
 *
 * /revoke で access_token (JWT) を失効させると、直後の /introspect が
 * active=false を返すことを検証する。
 */

import { describe, it, expect } from 'bun:test'

import { JoseTokenSigner } from '../../../adapters/crypto/jwt-signer'
import { InMemoryAccessTokenDenylist } from '../../../adapters/persistence/memory/access-token-denylist'
import { InMemoryKeyStore } from '../../../adapters/crypto/in-memory-key-store'
import { InMemoryRefreshTokenStore } from '../../../adapters/persistence/memory/refresh-store'

import { ClientSchema } from '../../spec-bindings/schemas'
import { introspectTokenUseCase } from './introspect-token'
import { revokeTokenUseCase } from './revoke-token'

const ISSUER = 'http://idp.example.com'

async function setup() {
  const keyStore = await InMemoryKeyStore.create('PS256')
  const signer = new JoseTokenSigner(ISSUER, keyStore)
  const denylist = new InMemoryAccessTokenDenylist()
  const refreshStore = new InMemoryRefreshTokenStore()
  const client = ClientSchema.parse({
    tenant_id: 'default',
    client_id: 'demo-client',
    client_secret_hash: 'x',
    client_type: 'confidential',
    redirect_uris: ['https://app.example.com/cb'],
    grant_types: ['authorization_code'],
    response_types: ['code'],
    token_endpoint_auth_method: 'client_secret_basic',
    scope: 'openid',
    id_token_signed_response_alg: 'PS256',
    require_pushed_authorization_requests: false,
    dpop_bound_access_tokens: false,
    fapi_profile: 'none',
    created_at: new Date().toISOString(),
  })
  const { token } = await signer.signAccessToken({
    sub: 'user-1',
    client,
    scopes: ['openid'],
    senderConstraint: null,
    authTime: Math.floor(Date.now() / 1000),
  })
  return { signer, denylist, refreshStore, token }
}

describe('access_token denylist (JWT 即時失効)', () => {
  it('revoke 直後の introspect は active=false を返す', async () => {
    const { signer, denylist, refreshStore, token } = await setup()

    const before = await introspectTokenUseCase(
      { introspector: signer, refreshStore, accessTokenDenylist: denylist },
      { token },
    )
    expect(before.active).toBe(true)

    const events: { type: string; tokenType?: unknown }[] = []
    await revokeTokenUseCase(
      { refreshStore, introspector: signer, accessTokenDenylist: denylist },
      token,
      (e) => events.push(e as { type: string; tokenType?: unknown }),
    )
    expect(events).toHaveLength(1)
    expect(events[0].type).toBe('TokenRevoked')
    expect(events[0].tokenType).toBe('access_token')

    const after = await introspectTokenUseCase(
      { introspector: signer, refreshStore, accessTokenDenylist: denylist },
      { token },
    )
    expect(after.active).toBe(false)
  })

  it('denylist 未設定なら revoke は無視され introspect は active のまま', async () => {
    const { signer, refreshStore, token } = await setup()

    await revokeTokenUseCase({ refreshStore, introspector: signer }, token, () => {})

    const res = await introspectTokenUseCase({ introspector: signer, refreshStore }, { token })
    expect(res.active).toBe(true)
  })
})

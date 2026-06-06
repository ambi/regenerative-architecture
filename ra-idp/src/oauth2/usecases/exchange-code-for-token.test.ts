/**
 * Layer 3 — Application Logic（ユースケース統合テスト）
 *
 * 認可コード交換の中核セキュリティプロパティを検証する:
 * - 成功パス: access_token / refresh_token / id_token 発行 + 監査情報の露出
 * - 認可コード再利用 → invalid_grant + ファミリー失効（RFC 9700 §4.10）
 * - PKCE 不一致 → invalid_grant
 * - 他クライアントの認可コード → invalid_grant
 *
 * 依存はすべて InMemory アダプターで構成し、ユースケース層を直接呼ぶ。
 */

import { describe, it, expect } from 'bun:test'
import { createHash } from 'crypto'

import { exchangeCodeForTokenUseCase } from './exchange-code-for-token'
import { generateAuthorizationCode } from '../domain/authorization-code'
import { generateInitial } from '../domain/refresh-token'
import { InMemoryClientRepository } from '../../../adapters/persistence/memory/client-repo'
import { InMemoryUserRepository } from '../../../adapters/persistence/memory/user-repo'
import { InMemoryAuthorizationCodeStore } from '../../../adapters/persistence/memory/authorization-store'
import { InMemoryRefreshTokenStore } from '../../../adapters/persistence/memory/refresh-store'
import { ClientSchema, UserSchema } from '../../spec-bindings/schemas'
import type { Client, User } from '../../spec-bindings/schemas'
import type { TokenIssuer } from '../ports/token-issuer'

// ---------------------------------------------------------------
// テスト用の fake TokenIssuer（jose 依存を避け、テストの決定性を保つ）
// ---------------------------------------------------------------

class FakeTokenIssuer implements TokenIssuer {
  public issuedJtis: string[] = []
  getAccessTokenTtlSeconds(): number {
    return 600
  }
  getIdTokenTtlSeconds(): number {
    return 3600
  }
  async signAccessToken(): Promise<{ token: string; jti: string }> {
    const jti = `jti-${this.issuedJtis.length}`
    this.issuedJtis.push(jti)
    return { token: `fake-at-${jti}`, jti }
  }
  async signIdToken(): Promise<string> {
    return 'fake-id-token'
  }
}

// ---------------------------------------------------------------
// 共通フィクスチャ
// ---------------------------------------------------------------

function makeClient(overrides: Partial<Client> = {}): Client {
  return ClientSchema.parse({
    client_id: 'web-app',
    client_secret_hash: createHash('sha256').update('s').digest('hex'),
    client_type: 'confidential',
    redirect_uris: ['https://app.example.com/cb'],
    grant_types: ['authorization_code', 'refresh_token'],
    response_types: ['code'],
    token_endpoint_auth_method: 'client_secret_basic',
    scope: 'openid profile offline_access',
    id_token_signed_response_alg: 'PS256',
    require_pushed_authorization_requests: false,
    dpop_bound_access_tokens: false,
    fapi_profile: 'none',
    created_at: new Date().toISOString(),
    ...overrides,
  })
}

function makeUser(): User {
  return UserSchema.parse({
    sub: 'user_alice',
    preferred_username: 'alice',
    password_hash: 'hash',
    name: 'Alice',
    email: 'alice@example.com',
    email_verified: true,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  })
}

const VERIFIER = 'verifier-of-known-length-ABCDEFGHIJKLMNOPQRSTUVWXY'
const CHALLENGE = createHash('sha256').update(VERIFIER).digest('base64url')

async function setup() {
  const clientRepo = new InMemoryClientRepository()
  const userRepo = new InMemoryUserRepository()
  const codeStore = new InMemoryAuthorizationCodeStore()
  const refreshStore = new InMemoryRefreshTokenStore()
  const tokenIssuer = new FakeTokenIssuer()

  const client = makeClient()
  const user = makeUser()
  await clientRepo.save(client)
  await userRepo.save(user)

  const code = generateAuthorizationCode({
    authorization_request_id: '00000000-0000-0000-0000-000000000001',
    client_id: client.client_id,
    sub: user.sub,
    scopes: ['openid', 'profile', 'offline_access'],
    redirect_uri: client.redirect_uris[0],
    code_challenge: CHALLENGE,
    code_challenge_method: 'S256',
    auth_time: 1700000000,
  })
  await codeStore.save(code)

  return { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer, client, user, code }
}

// ---------------------------------------------------------------
// テスト本体
// ---------------------------------------------------------------

describe('exchangeCodeForTokenUseCase — 成功パス', () => {
  it('access_token / refresh_token / id_token を返す + 監査情報を含む', async () => {
    const { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer, client, code } =
      await setup()

    const result = await exchangeCodeForTokenUseCase(
      { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer },
      {
        client_id: client.client_id,
        code: code.code,
        code_verifier: VERIFIER,
        redirect_uri: code.redirect_uri,
      },
    )

    const refreshToken = result.response.refresh_token
    expect(result.response.access_token).toMatch(/^fake-at-/)
    expect(refreshToken).toBeDefined()
    expect(refreshToken!.length).toBeGreaterThan(40)
    expect(result.response.id_token).toBe('fake-id-token')
    expect(result.response.token_type).toBe('Bearer')
    expect(result.response.scope).toBe('openid profile offline_access')

    // 監査情報がアダプター層に露出している（要件 §13）
    expect(result.audit.sub).toBe('user_alice')
    expect(result.audit.jti).toBe('jti-0')
    expect(result.audit.refreshTokenId).toMatch(/^[0-9a-f-]{36}$/)
    expect(result.audit.refreshFamilyId).toMatch(/^[0-9a-f-]{36}$/)
    expect(result.audit.senderConstraint).toBe('none')
  })

  it('DPoP jkt が指定されると token_type は DPoP かつ cnf にバインドされる', async () => {
    const { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer, client, code } =
      await setup()

    const result = await exchangeCodeForTokenUseCase(
      { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer },
      {
        client_id: client.client_id,
        code: code.code,
        code_verifier: VERIFIER,
        redirect_uri: code.redirect_uri,
        dpop_jkt: 'dpop-thumbprint-xyz',
      },
    )
    expect(result.response.token_type).toBe('DPoP')
    expect(result.audit.senderConstraint).toBe('dpop')
  })

  it('redeem 後に code レコードに issued_family_id がリンクされる', async () => {
    const { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer, client, code } =
      await setup()

    const result = await exchangeCodeForTokenUseCase(
      { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer },
      {
        client_id: client.client_id,
        code: code.code,
        code_verifier: VERIFIER,
        redirect_uri: code.redirect_uri,
      },
    )
    const after = await codeStore.find(code.code)
    expect(after?.issued_family_id).toBe(result.audit.refreshFamilyId)
    expect(after?.redeemed_at).toBeDefined()
  })
})

describe('exchangeCodeForTokenUseCase — 認可コード再利用検出', () => {
  it('2 回目の同一コード使用は invalid_grant で拒否される', async () => {
    const { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer, client, code } =
      await setup()

    await exchangeCodeForTokenUseCase(
      { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer },
      {
        client_id: client.client_id,
        code: code.code,
        code_verifier: VERIFIER,
        redirect_uri: code.redirect_uri,
      },
    )

    const second = exchangeCodeForTokenUseCase(
      { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer },
      {
        client_id: client.client_id,
        code: code.code,
        code_verifier: VERIFIER,
        redirect_uri: code.redirect_uri,
      },
    )
    await expect(second).rejects.toThrow(/invalid_grant|使用済み/)
  })

  it('再利用検出時に最初のリフレッシュトークンファミリーが失効する（RFC 9700 §4.10）', async () => {
    const { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer, client, code } =
      await setup()

    const first = await exchangeCodeForTokenUseCase(
      { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer },
      {
        client_id: client.client_id,
        code: code.code,
        code_verifier: VERIFIER,
        redirect_uri: code.redirect_uri,
      },
    )

    // 2 回目を試みる → ファミリーが失効されるはず
    try {
      await exchangeCodeForTokenUseCase(
        { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer },
        {
          client_id: client.client_id,
          code: code.code,
          code_verifier: VERIFIER,
          redirect_uri: code.redirect_uri,
        },
      )
    } catch {
      // expected
    }

    // 1 回目に発行された refresh_token はファミリー失効により revoked = true
    const refreshToken = first.response.refresh_token
    expect(refreshToken).toBeDefined()
    const refreshHash = createHash('sha256').update(refreshToken!).digest('hex')
    const refreshRecord = await refreshStore.findByHash(refreshHash)
    expect(refreshRecord?.revoked).toBe(true)
    expect(refreshRecord?.family_id).toBe(first.audit.refreshFamilyId)
  })
})

describe('exchangeCodeForTokenUseCase — セキュリティ境界', () => {
  it('PKCE verifier が不一致なら invalid_grant', async () => {
    const { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer, client, code } =
      await setup()

    const wrong = exchangeCodeForTokenUseCase(
      { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer },
      {
        client_id: client.client_id,
        code: code.code,
        code_verifier: 'wrong-verifier-of-equal-length-AAAAAAAAAAAAAAAAA',
        redirect_uri: code.redirect_uri,
      },
    )
    await expect(wrong).rejects.toThrow(/invalid_grant|PKCE/)
  })

  it('他クライアントの認可コードを使うと invalid_grant', async () => {
    const { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer, code } = await setup()

    const other = makeClient({
      client_id: 'other-app',
      redirect_uris: ['https://other.example.com/cb'],
    })
    await clientRepo.save(other)

    const stolen = exchangeCodeForTokenUseCase(
      { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer },
      {
        client_id: other.client_id,
        code: code.code,
        code_verifier: VERIFIER,
        redirect_uri: code.redirect_uri,
      },
    )
    await expect(stolen).rejects.toThrow(/invalid_grant|クライアントと一致/)
  })

  it('redirect_uri が認可リクエスト時と異なれば invalid_grant', async () => {
    const { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer, client, code } =
      await setup()

    const tampered = exchangeCodeForTokenUseCase(
      { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer },
      {
        client_id: client.client_id,
        code: code.code,
        code_verifier: VERIFIER,
        redirect_uri: 'https://app.example.com/other-cb',
      },
    )
    await expect(tampered).rejects.toThrow()
  })
})

describe('exchangeCodeForTokenUseCase — id_token は openid スコープのみ発行', () => {
  it('openid を含まないスコープでは id_token を返さない', async () => {
    const { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer, client, user } =
      await setup()

    const noOpenidCode = generateAuthorizationCode({
      authorization_request_id: '00000000-0000-0000-0000-000000000002',
      client_id: client.client_id,
      sub: user.sub,
      scopes: ['profile'],
      redirect_uri: client.redirect_uris[0],
      code_challenge: CHALLENGE,
      code_challenge_method: 'S256',
      auth_time: 1700000000,
    })
    await codeStore.save(noOpenidCode)

    const result = await exchangeCodeForTokenUseCase(
      { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer },
      {
        client_id: client.client_id,
        code: noOpenidCode.code,
        code_verifier: VERIFIER,
        redirect_uri: noOpenidCode.redirect_uri,
      },
    )
    expect(result.response.id_token).toBeUndefined()
  })
})

describe('exchangeCodeForTokenUseCase — refresh_token は offline_access スコープのみ発行', () => {
  it('offline_access を含まないスコープでは refresh_token を返さない', async () => {
    const { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer, client, user } =
      await setup()

    const noOfflineAccessCode = generateAuthorizationCode({
      authorization_request_id: '00000000-0000-0000-0000-000000000003',
      client_id: client.client_id,
      sub: user.sub,
      scopes: ['openid', 'profile'],
      redirect_uri: client.redirect_uris[0],
      code_challenge: CHALLENGE,
      code_challenge_method: 'S256',
      auth_time: 1700000000,
    })
    await codeStore.save(noOfflineAccessCode)

    const result = await exchangeCodeForTokenUseCase(
      { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer },
      {
        client_id: client.client_id,
        code: noOfflineAccessCode.code,
        code_verifier: VERIFIER,
        redirect_uri: noOfflineAccessCode.redirect_uri,
      },
    )

    expect(result.response.refresh_token).toBeUndefined()
    expect(result.audit.refreshTokenId).toBeUndefined()
    expect(result.audit.refreshFamilyId).toBeUndefined()
  })
})

// generateInitial が refresh-token.test.ts でカバーされていることを確認する用途で使う
void generateInitial

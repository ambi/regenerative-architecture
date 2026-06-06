/**
 * Layer 3 — Specification Binding (TypeScript)（認可ポリシー単体テスト）
 *
 * 仕様本体 ../../../spec/scl.yaml の permissions セクション宣言が、
 * evaluate() の挙動として正しく動くことを検証する。
 * 仕様変更時はまずここで期待動作を書く。
 */

import { describe, it, expect } from 'bun:test'
import { evaluate } from './client-authorization'
import type { AuthZENRequest } from './client-authorization'

const now = '2030-01-01T00:00:00.000Z'
const futureExp = '2030-01-02T00:00:00.000Z'
const pastExp = '2029-01-01T00:00:00.000Z'

describe('authorize:initiate', () => {
  it('PKCE が無い場合は Deny', () => {
    const req: AuthZENRequest = {
      subject: {
        type: 'Client',
        id: 'web-app',
        properties: {
          scopes: ['openid', 'profile'],
          redirectUris: ['https://app.example.com/cb'],
        },
      },
      action: { name: 'authorize:initiate' },
      resource: {
        type: 'AuthorizationRequest',
        properties: {
          scopes: ['openid'],
          // codeChallenge 欠落
        },
      },
      context: { redirectUri: 'https://app.example.com/cb', now },
    }
    const res = evaluate(req)
    expect(res.decision).toBe('Deny')
    expect(res.reasons).toContain('pkce_present')
  })

  it('redirect_uri が登録外なら Deny', () => {
    const req: AuthZENRequest = {
      subject: {
        type: 'Client',
        id: 'web-app',
        properties: {
          scopes: ['openid'],
          redirectUris: ['https://app.example.com/cb'],
        },
      },
      action: { name: 'authorize:initiate' },
      resource: {
        type: 'AuthorizationRequest',
        properties: { codeChallenge: 'abc', scopes: ['openid'] },
      },
      context: { redirectUri: 'https://evil.example/cb', now },
    }
    const res = evaluate(req)
    expect(res.decision).toBe('Deny')
    expect(res.reasons).toContain('redirect_uri_registered')
  })

  it('FAPI クライアントが PAR を経由しないと Deny', () => {
    const req: AuthZENRequest = {
      subject: {
        type: 'Client',
        id: 'fapi-app',
        properties: {
          scopes: ['openid'],
          redirectUris: ['https://app.example.com/cb'],
          requirePAR: true,
        },
      },
      action: { name: 'authorize:initiate' },
      resource: {
        type: 'AuthorizationRequest',
        properties: { codeChallenge: 'abc', scopes: ['openid'] },
      },
      context: { redirectUri: 'https://app.example.com/cb', parUsed: false, now },
    }
    const res = evaluate(req)
    expect(res.decision).toBe('Deny')
    expect(res.reasons).toContain('par_required_if_fapi')
  })

  it('PAR を経由した FAPI クライアントは Permit', () => {
    const req: AuthZENRequest = {
      subject: {
        type: 'Client',
        id: 'fapi-app',
        properties: {
          scopes: ['openid'],
          redirectUris: ['https://app.example.com/cb'],
          requirePAR: true,
        },
      },
      action: { name: 'authorize:initiate' },
      resource: {
        type: 'AuthorizationRequest',
        properties: { codeChallenge: 'abc', scopes: ['openid'] },
      },
      context: { redirectUri: 'https://app.example.com/cb', parUsed: true, now },
    }
    const res = evaluate(req)
    expect(res.decision).toBe('Permit')
  })

  it('要求スコープがクライアント宣言の部分集合でないと Deny', () => {
    const req: AuthZENRequest = {
      subject: {
        type: 'Client',
        id: 'web-app',
        properties: {
          scopes: ['openid'],
          redirectUris: ['https://app.example.com/cb'],
        },
      },
      action: { name: 'authorize:initiate' },
      resource: {
        type: 'AuthorizationRequest',
        properties: {
          codeChallenge: 'abc',
          scopes: ['openid', 'admin:write'], // admin:write が宣言外
        },
      },
      context: { redirectUri: 'https://app.example.com/cb', now },
    }
    const res = evaluate(req)
    expect(res.decision).toBe('Deny')
    expect(res.reasons).toContain('scope_subset_of_client_scope')
  })
})

describe('token:grant_authorization_code', () => {
  it('正規ケースは Permit', () => {
    const req: AuthZENRequest = {
      subject: {
        type: 'Client',
        id: 'web-app',
        properties: { grantTypes: ['authorization_code'] },
      },
      action: { name: 'token:grant_authorization_code' },
      resource: {
        type: 'AuthorizationCode',
        properties: {
          codeChallenge: 'abc',
          redirectUri: 'https://app.example.com/cb',
          redeemed: false,
          expiresAt: futureExp,
        },
      },
      context: {
        codeVerifier: 'verifier',
        redirectUri: 'https://app.example.com/cb',
        now,
      },
    }
    const res = evaluate(req)
    expect(res.decision).toBe('Permit')
  })

  it('redeemed なコードは Deny', () => {
    const req: AuthZENRequest = {
      subject: {
        type: 'Client',
        id: 'web-app',
        properties: { grantTypes: ['authorization_code'] },
      },
      action: { name: 'token:grant_authorization_code' },
      resource: {
        type: 'AuthorizationCode',
        properties: {
          codeChallenge: 'abc',
          redirectUri: 'https://app.example.com/cb',
          redeemed: true,
          expiresAt: futureExp,
        },
      },
      context: {
        codeVerifier: 'verifier',
        redirectUri: 'https://app.example.com/cb',
        now,
      },
    }
    const res = evaluate(req)
    expect(res.decision).toBe('Deny')
    expect(res.reasons).toContain('code_not_redeemed')
  })

  it('期限切れコードは Deny', () => {
    const req: AuthZENRequest = {
      subject: {
        type: 'Client',
        id: 'web-app',
        properties: { grantTypes: ['authorization_code'] },
      },
      action: { name: 'token:grant_authorization_code' },
      resource: {
        type: 'AuthorizationCode',
        properties: {
          codeChallenge: 'abc',
          redirectUri: 'https://app.example.com/cb',
          redeemed: false,
          expiresAt: pastExp,
        },
      },
      context: {
        codeVerifier: 'verifier',
        redirectUri: 'https://app.example.com/cb',
        now,
      },
    }
    const res = evaluate(req)
    expect(res.decision).toBe('Deny')
    expect(res.reasons).toContain('code_not_expired')
  })

  it('redirect_uri 不一致は Deny', () => {
    const req: AuthZENRequest = {
      subject: {
        type: 'Client',
        id: 'web-app',
        properties: { grantTypes: ['authorization_code'] },
      },
      action: { name: 'token:grant_authorization_code' },
      resource: {
        type: 'AuthorizationCode',
        properties: {
          codeChallenge: 'abc',
          redirectUri: 'https://app.example.com/cb',
          redeemed: false,
          expiresAt: futureExp,
        },
      },
      context: {
        codeVerifier: 'verifier',
        redirectUri: 'https://app.example.com/other',
        now,
      },
    }
    const res = evaluate(req)
    expect(res.decision).toBe('Deny')
    expect(res.reasons).toContain('redirect_uri_exact_match')
  })
})

describe('token:grant_refresh', () => {
  it('Sender-Constrained トークンで proof が無いと Deny', () => {
    const req: AuthZENRequest = {
      subject: {
        type: 'Client',
        id: 'web-app',
        properties: { grantTypes: ['refresh_token'] },
      },
      action: { name: 'token:grant_refresh' },
      resource: {
        type: 'RefreshToken',
        properties: {
          revoked: false,
          rotated: false,
          absoluteExpiresAt: futureExp,
          senderConstraint: { type: 'dpop' },
        },
      },
      context: { proofOfPossession: null, now },
    }
    const res = evaluate(req)
    expect(res.decision).toBe('Deny')
    expect(res.reasons).toContain('sender_constraint_satisfied')
  })

  it('ローテーション済みトークンの再利用は Deny', () => {
    const req: AuthZENRequest = {
      subject: {
        type: 'Client',
        id: 'web-app',
        properties: { grantTypes: ['refresh_token'] },
      },
      action: { name: 'token:grant_refresh' },
      resource: {
        type: 'RefreshToken',
        properties: {
          revoked: false,
          rotated: true,
          absoluteExpiresAt: futureExp,
        },
      },
      context: { now },
    }
    const res = evaluate(req)
    expect(res.decision).toBe('Deny')
    expect(res.reasons).toContain('token_active')
  })
})

describe('userinfo:read', () => {
  it('openid スコープなしのトークンは Deny', () => {
    const req: AuthZENRequest = {
      subject: { type: 'Client', id: 'web-app' },
      action: { name: 'userinfo:read' },
      resource: {
        type: 'UserInfo',
        properties: { scopes: ['profile'], revoked: false },
      },
      context: { now },
    }
    const res = evaluate(req)
    expect(res.decision).toBe('Deny')
    expect(res.reasons).toContain('token_has_openid_scope')
  })

  it('openid スコープと active トークンなら Permit', () => {
    const req: AuthZENRequest = {
      subject: { type: 'Client', id: 'web-app' },
      action: { name: 'userinfo:read' },
      resource: {
        type: 'UserInfo',
        properties: { scopes: ['openid', 'profile'], revoked: false },
      },
      context: { now },
    }
    const res = evaluate(req)
    expect(res.decision).toBe('Permit')
  })
})

describe('token:grant_client_credentials', () => {
  it('public クライアントは Deny', () => {
    const req: AuthZENRequest = {
      subject: {
        type: 'Client',
        id: 'spa-app',
        properties: { clientType: 'public', grantTypes: ['client_credentials'] },
      },
      action: { name: 'token:grant_client_credentials' },
      resource: { type: 'AccessToken' },
      context: { now },
    }
    const res = evaluate(req)
    expect(res.decision).toBe('Deny')
    expect(res.reasons).toContain('client_is_confidential')
  })
})

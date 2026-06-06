/**
 * Layer 3 — Specification Binding (TypeScript)（Gherkin シナリオの実行版）
 *
 * 仕様本体 ../../spec/scl.yaml の scenarios と同じ「ハッピーパス」「セキュリティ境界」を、
 * Bun の test runner で実行可能な形に表現する。
 *
 * 仕様本体の state_machines と permissions だけで成立するシナリオをここに置く
 * （HTTP は含まない）。HTTP 経由のシナリオは demo.sh で実行する。
 */

import { describe, it, expect } from 'bun:test'

import { transitionAuthCode, isAuthCodeTerminal } from './flows/flows'
import type { AuthCodeState, AuthCodeEvent } from './flows/flows'

import { evaluate } from './policy/client-authorization'
import type { AuthZENRequest } from './policy/client-authorization'

// ---------------------------------------------------------------
// 認可フローのライフサイクル（状態機械の golden パス）
// ---------------------------------------------------------------

function applyEvents(initial: AuthCodeState, events: AuthCodeEvent[]): AuthCodeState | null {
  let s: AuthCodeState | null = initial
  for (const e of events) {
    if (s === null) return null
    s = transitionAuthCode(s, e)
  }
  return s
}

describe('Scenario: 認可コードフローでアクセストークンと ID トークンを取得する', () => {
  it('received → authentication_pending → authenticated → consented → code_issued → exchanged', () => {
    const result = applyEvents('received', ['validate', 'authenticate_user', 'request_consent'])
    // request_consent は authentication_pending では発火できないため、
    // validate 後の authentication_pending → authenticate_user → authenticated → request_consent を確認
    expect(result).toBe('consent_pending')
  })

  it('full happy path: received → ... → exchanged', () => {
    const result = applyEvents('received', [
      'validate', // → authentication_pending
      'authenticate_user', // → authenticated
      'request_consent', // → consent_pending
      'grant_consent', // → consented
      'issue_code', // → code_issued
      'redeem_code', // → exchanged
    ])
    expect(result).toBe('exchanged')
    expect(isAuthCodeTerminal(result!)).toBe(true)
  })

  it('既存コンセントがあるときは authenticated → 直接 issue_code', () => {
    const result = applyEvents('received', [
      'validate',
      'authenticate_user',
      'issue_code', // request_consent をスキップ
      'redeem_code',
    ])
    expect(result).toBe('exchanged')
  })
})

describe('Scenario: 認可コードを 2 回使用すると失敗する', () => {
  it('exchanged 状態からの redeem_code は null（不正遷移）', () => {
    // code_issued → redeem_code → exchanged まで進めたあと、
    // 同じ authorization request にもう一度 redeem_code は発火できない
    const s1 = applyEvents('code_issued', ['redeem_code'])
    expect(s1).toBe('exchanged')
    const s2 = transitionAuthCode(s1!, 'redeem_code')
    expect(s2).toBeNull()
  })
})

describe('Scenario: 期限切れ後の遷移は不可', () => {
  it('expired は終端', () => {
    expect(isAuthCodeTerminal('expired')).toBe(true)
    for (const ev of [
      'authenticate_user',
      'grant_consent',
      'issue_code',
      'redeem_code',
    ] as AuthCodeEvent[]) {
      expect(transitionAuthCode('expired', ev)).toBeNull()
    }
  })
})

// ---------------------------------------------------------------
// 認可ポリシーの golden 評価
// ---------------------------------------------------------------

describe('Scenario: PKCE verifier 不一致は authorize:initiate を通過しても token 段で拒否される', () => {
  // PKCE の SHA-256 検証は usecase 層で行うため、ポリシーはコンテキストとして
  // 「verifier が提示されたか」を判定する。一致／不一致のビジネス検証は ADR-002 に従い
  // usecase（exchange-code-for-token）で行う。
  // ここではポリシー単独で「verifier 欠落 → Deny」を確認する。
  it('verifier 欠落は Deny', () => {
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
          expiresAt: '2099-01-01T00:00:00.000Z',
        },
      },
      context: {
        // codeVerifier 欠落
        redirectUri: 'https://app.example.com/cb',
      },
    }
    const res = evaluate(req)
    expect(res.decision).toBe('Deny')
    expect(res.reasons).toContain('pkce_verification_passed')
  })
})

describe('Scenario: 未登録 redirect_uri は authorize:initiate で拒否', () => {
  it('未登録 URI → Deny', () => {
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
      context: { redirectUri: 'https://evil.example/cb' },
    }
    const res = evaluate(req)
    expect(res.decision).toBe('Deny')
    expect(res.reasons).toContain('redirect_uri_registered')
  })
})

describe('Scenario: 既存コンセントがあれば consent UI をスキップして code 発行に至れる', () => {
  it('状態機械として authenticated → issue_code が許可される', () => {
    expect(transitionAuthCode('authenticated', 'issue_code')).toBe('code_issued')
  })
})

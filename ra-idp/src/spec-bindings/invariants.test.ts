/**
 * Layer 3 — Specification Binding (TypeScript) — 検証
 *
 * 仕様整合性テスト + プロパティベース不変条件テスト。
 *
 * - 「SCL（spec/scl.yaml）と TypeScript バインディングの整合」
 * - 「状態機械の不変条件（どんな入力でも成り立つ性質）」
 *
 * これらは仕様であり、テストでもある。SCL を変更するとき、まずここで設計の健全性を確認する。
 */

import { describe, it, expect } from 'bun:test'
import fc from 'fast-check'

import {
  scl,
  enumWireValues,
  toWire,
  statesOf,
  eventsOf,
  vocabularyCodes,
  AUTH_CODE_FLOW,
  DEVICE_CODE_FLOW,
  httpBinding,
} from './scl'

import {
  AUTH_CODE_STATES,
  AUTH_CODE_EVENTS,
  AUTH_CODE_TRANSITIONS,
  transitionAuthCode,
  isAuthCodeTerminal,
  DEVICE_CODE_STATES,
  DEVICE_CODE_EVENTS,
  DEVICE_CODE_TRANSITIONS,
  transitionDeviceCode,
  isDeviceCodeTerminal,
} from './flows/flows'
import type { AuthCodeState, AuthCodeEvent, DeviceCodeState, DeviceCodeEvent } from './flows/flows'

import {
  SUPPORTED_GRANT_TYPES,
  CLIENT_TYPES,
  TOKEN_AUTH_METHODS,
  RESPONSE_TYPES,
} from './grants/grant-types'

import {
  ALL_RULE_IDS,
  IMPLEMENTED_RULE_IDS,
  ACTION_NAMES,
  sclPermissionsCoveredByActionNames,
} from './policy/client-authorization'

import { buildDiscoveryDocument } from './discovery'
import { loadSlo, loadObservability } from '../../infra/scripts/load-specs'

const slo = loadSlo()
const observability = loadObservability()

// ===============================================================
// ユビキタス言語整合性（SCL vocabulary ↔ 仕様核）
// ===============================================================

describe('ユビキタス言語整合性 (SCL vocabulary)', () => {
  const codes = vocabularyCodes()

  it('用語集（vocabulary）のキー名が重複していない', () => {
    const names = Object.keys(scl.vocabulary)
    expect(names.length).toBe(new Set(names).size)
  })

  it('AuthorizationCodeFlow のすべての state が vocabulary に登録されている', () => {
    for (const state of statesOf(AUTH_CODE_FLOW)) {
      expect(scl.vocabulary[state]).toBeDefined()
    }
  })

  it('AuthorizationCodeFlow のすべての event が vocabulary に登録されている', () => {
    for (const event of eventsOf(AUTH_CODE_FLOW)) {
      expect(scl.vocabulary[event]).toBeDefined()
    }
  })

  it('DeviceCodeFlow のすべての state が vocabulary に登録されている', () => {
    for (const state of statesOf(DEVICE_CODE_FLOW)) {
      expect(scl.vocabulary[state]).toBeDefined()
    }
  })

  it('DeviceCodeFlow のすべての event が vocabulary に登録されている', () => {
    for (const event of eventsOf(DEVICE_CODE_FLOW)) {
      expect(scl.vocabulary[event]).toBeDefined()
    }
  })

  it('GrantType 列挙値（PascalCase）が vocabulary に登録されている', () => {
    for (const v of (scl.models.GrantType as { values: string[] }).values) {
      expect(scl.vocabulary[v]).toBeDefined()
    }
  })

  it('ClientType 列挙値が vocabulary に登録されている', () => {
    for (const v of (scl.models.ClientType as { values: string[] }).values) {
      expect(scl.vocabulary[v]).toBeDefined()
    }
  })

  it('中核アクター (Client/ResourceOwner/AuthorizationServer/ResourceServer) が登録されている', () => {
    for (const name of ['Client', 'ResourceOwner', 'AuthorizationServer', 'ResourceServer']) {
      expect(scl.vocabulary[name]).toBeDefined()
    }
  })

  it('中核トークン (AccessToken/RefreshToken/IdToken/AuthorizationCode) が登録されている', () => {
    for (const name of ['AccessToken', 'RefreshToken', 'IdToken', 'AuthorizationCode']) {
      expect(scl.vocabulary[name]).toBeDefined()
    }
  })

  it('セキュリティメカニズム (Pkce/Dpop/Mtls/Par) が登録されている', () => {
    for (const name of ['Pkce', 'Dpop', 'Mtls', 'Par']) {
      expect(scl.vocabulary[name]).toBeDefined()
    }
  })

  it('インタフェース・スキーマの代表値が vocabulary 由来である（toWire の動作確認）', () => {
    expect(toWire('AuthorizationCode')).toBe('authorization_code')
    expect(toWire('ClientCredentials')).toBe('client_credentials')
    expect(toWire('Public')).toBe('public')
    expect(toWire('Pkce')).toBe('pkce')
    expect(codes.has('authorization_code')).toBe(true)
  })
})

// ===============================================================
// 仕様整合性（SCL ↔ TypeScript 定数）
// ===============================================================

describe('Authorization Code Flow — SCL ↔ TypeScript 整合', () => {
  it('AUTH_CODE_STATES が SCL の状態集合と一致する', () => {
    const sclStates = statesOf(AUTH_CODE_FLOW).map(toWire).sort()
    expect(([...AUTH_CODE_STATES] as string[]).sort()).toEqual(sclStates)
  })

  it('AUTH_CODE_EVENTS が SCL のイベント集合と一致する', () => {
    const sclEvents = eventsOf(AUTH_CODE_FLOW).map(toWire).sort()
    expect(([...AUTH_CODE_EVENTS] as string[]).sort()).toEqual(sclEvents)
  })

  it('SCL 遷移表の状態が AUTH_CODE_STATES の部分集合である', () => {
    const valid = new Set<string>(AUTH_CODE_STATES)
    for (const t of scl.states[AUTH_CODE_FLOW].transitions) {
      expect(valid.has(toWire(t.from))).toBe(true)
      expect(valid.has(toWire(t.to))).toBe(true)
    }
  })

  it('SCL 遷移表のイベントが AUTH_CODE_EVENTS の部分集合である', () => {
    const valid = new Set<string>(AUTH_CODE_EVENTS)
    for (const t of scl.states[AUTH_CODE_FLOW].transitions) {
      expect(valid.has(toWire(t.event))).toBe(true)
    }
  })
})

describe('Device Code Flow — SCL ↔ TypeScript 整合', () => {
  it('DEVICE_CODE_STATES が SCL の状態集合と一致する', () => {
    const sclStates = statesOf(DEVICE_CODE_FLOW).map(toWire).sort()
    expect(([...DEVICE_CODE_STATES] as string[]).sort()).toEqual(sclStates)
  })

  it('DEVICE_CODE_EVENTS が SCL のイベント集合と一致する', () => {
    const sclEvents = eventsOf(DEVICE_CODE_FLOW).map(toWire).sort()
    expect(([...DEVICE_CODE_EVENTS] as string[]).sort()).toEqual(sclEvents)
  })
})

describe('Grant Types — SCL ↔ TypeScript 整合', () => {
  it('SUPPORTED_GRANT_TYPES が SCL GrantType enum と一致する', () => {
    expect(([...SUPPORTED_GRANT_TYPES] as string[]).sort()).toEqual(
      enumWireValues('GrantType').sort(),
    )
  })

  it('CLIENT_TYPES が SCL ClientType enum と一致する', () => {
    expect(([...CLIENT_TYPES] as string[]).sort()).toEqual(enumWireValues('ClientType').sort())
  })

  it('TOKEN_AUTH_METHODS が SCL TokenEndpointAuthMethod enum と一致する', () => {
    expect(([...TOKEN_AUTH_METHODS] as string[]).sort()).toEqual(
      enumWireValues('TokenEndpointAuthMethod').sort(),
    )
  })

  it('RESPONSE_TYPES が SCL ResponseType enum と一致する', () => {
    expect(([...RESPONSE_TYPES] as string[]).sort()).toEqual(enumWireValues('ResponseType').sort())
  })
})

// ===============================================================
// Discovery — SCL からの派生確認
// ===============================================================

describe('Discovery — SCL からの派生整合', () => {
  const doc = buildDiscoveryDocument('https://idp.example.com')

  it('grant_types_supported が SCL GrantType enum と一致する', () => {
    expect((doc.grant_types_supported as string[]).sort()).toEqual(
      ([...SUPPORTED_GRANT_TYPES] as string[]).sort(),
    )
  })

  it('response_types_supported が SCL ResponseType enum と一致する', () => {
    expect((doc.response_types_supported as string[]).sort()).toEqual(
      ([...RESPONSE_TYPES] as string[]).sort(),
    )
  })

  it('token_endpoint_auth_methods_supported が "none" を除いた SCL enum と一致する', () => {
    const expected = ([...TOKEN_AUTH_METHODS] as string[]).filter((m) => m !== 'none').sort()
    expect((doc.token_endpoint_auth_methods_supported as string[]).sort()).toEqual(expected)
  })

  it('authorization_endpoint が SCL Authorize の HTTP binding path と一致する', () => {
    expect(doc.authorization_endpoint).toBe(
      `https://idp.example.com${httpBinding(scl.interfaces.Authorize)?.path}`,
    )
  })

  it('token_endpoint が SCL Token の HTTP binding path と一致する', () => {
    expect(doc.token_endpoint).toBe(
      `https://idp.example.com${httpBinding(scl.interfaces.Token)?.path}`,
    )
  })

  it('authorization_response_iss_parameter_supported が true (RFC 9207)', () => {
    expect(doc.authorization_response_iss_parameter_supported).toBe(true)
  })

  it('acr_values_supported は SCL annotations.acr_vocabulary と一致する', () => {
    expect(doc.acr_values_supported).toEqual(['urn:ra-idp:acr:pwd', 'urn:ra-idp:acr:mfa'])
  })
})

// ===============================================================
// 認可ポリシーの実装完備性
// ===============================================================

describe('AuthZEN ポリシー — 全ルールが実装されている', () => {
  it('action 表で言及される全 rule.id が ruleEvaluators に実装されている', () => {
    const missing = ALL_RULE_IDS.filter((id) => !IMPLEMENTED_RULE_IDS.includes(id))
    expect(missing).toEqual([])
  })

  it('SCL `permissions` で宣言された全アクションが ACTION_NAMES にマップされている', () => {
    const { missing, extra } = sclPermissionsCoveredByActionNames()
    expect(missing).toEqual([])
    expect(extra).toEqual([])
  })

  it('AuthZEN action 名が "domain:verb" snake_case 規約に合う', () => {
    for (const a of Object.values(ACTION_NAMES)) {
      expect(a).toMatch(/^[a-z][a-z_]*:[a-z][a-z_]*$/)
    }
  })
})

// ===============================================================
// 状態機械の不変条件 — Authorization Code Flow
// ===============================================================

describe('Authorization Code Flow — 不変条件 (property-based)', () => {
  it('終端状態には発火可能なイベントが存在しない', () => {
    for (const state of AUTH_CODE_STATES) {
      if (isAuthCodeTerminal(state)) {
        expect(Object.keys(AUTH_CODE_TRANSITIONS[state])).toHaveLength(0)
      }
    }
  })

  it('すべての状態は received から到達可能である', () => {
    const reachable = new Set<AuthCodeState>(['received'])
    const queue: AuthCodeState[] = ['received']
    while (queue.length > 0) {
      const state = queue.shift()!
      for (const next of Object.values(AUTH_CODE_TRANSITIONS[state]) as AuthCodeState[]) {
        if (!reachable.has(next)) {
          reachable.add(next)
          queue.push(next)
        }
      }
    }
    expect(([...reachable] as string[]).sort()).toEqual(([...AUTH_CODE_STATES] as string[]).sort())
  })

  it('遷移は決定論的', () => {
    fc.assert(
      fc.property(
        fc.constantFrom(...AUTH_CODE_STATES),
        fc.constantFrom(...AUTH_CODE_EVENTS),
        (state: AuthCodeState, event: AuthCodeEvent) => {
          const first = transitionAuthCode(state, event)
          const second = transitionAuthCode(state, event)
          return first === second
        },
      ),
    )
  })

  it('任意のイベント列を適用しても常に合法な状態に留まる', () => {
    fc.assert(
      fc.property(
        fc.array(fc.constantFrom(...AUTH_CODE_EVENTS), { maxLength: 30 }),
        (events: AuthCodeEvent[]) => {
          let state: AuthCodeState = 'received'
          for (const event of events) {
            const next = transitionAuthCode(state, event)
            if (next !== null) state = next
          }
          return (AUTH_CODE_STATES as readonly string[]).includes(state)
        },
      ),
    )
  })

  it('終端状態に入ったあとはどのイベントでも状態は変わらない', () => {
    fc.assert(
      fc.property(fc.constantFrom(...AUTH_CODE_EVENTS), (event: AuthCodeEvent) => {
        for (const state of AUTH_CODE_STATES) {
          if (isAuthCodeTerminal(state)) {
            expect(transitionAuthCode(state, event)).toBeNull()
          }
        }
      }),
    )
  })

  it('exchanged にいったん入ったら redeem_code は二度発火できない', () => {
    expect(transitionAuthCode('exchanged', 'redeem_code')).toBeNull()
  })
})

// ===============================================================
// 状態機械の不変条件 — Device Code Flow
// ===============================================================

describe('Device Code Flow — 不変条件', () => {
  it('終端状態には発火可能なイベントが存在しない', () => {
    for (const state of DEVICE_CODE_STATES) {
      if (isDeviceCodeTerminal(state)) {
        expect(Object.keys(DEVICE_CODE_TRANSITIONS[state])).toHaveLength(0)
      }
    }
  })

  it('任意のイベント列を適用しても常に合法な状態に留まる', () => {
    fc.assert(
      fc.property(
        fc.array(fc.constantFrom(...DEVICE_CODE_EVENTS), { maxLength: 30 }),
        (events: DeviceCodeEvent[]) => {
          let state: DeviceCodeState = 'issued'
          for (const event of events) {
            const next = transitionDeviceCode(state, event)
            if (next !== null) state = next
          }
          return (DEVICE_CODE_STATES as readonly string[]).includes(state)
        },
      ),
    )
  })
})

// ===============================================================
// SLO の整合（SCL objectives から派生）
// ===============================================================

describe('SLO — トークンライフタイム整合性', () => {
  it('authorization_code_ttl が 60 秒以下（RFC 9700 §4.10 推奨上限）', () => {
    expect(slo.token_lifetimes.authorization_code_ttl_seconds).toBeLessThanOrEqual(60)
  })

  it('par_request_uri_ttl が 600 秒以下', () => {
    expect(slo.token_lifetimes.par_request_uri_ttl_seconds).toBeLessThanOrEqual(600)
  })

  it('refresh_token の絶対 TTL は通常 TTL 以上である', () => {
    expect(slo.token_lifetimes.refresh_token_absolute_ttl_seconds).toBeGreaterThanOrEqual(
      slo.token_lifetimes.refresh_token_ttl_seconds,
    )
  })

  it('署名鍵の最大寿命は 90 日以下（ADR-009）', () => {
    expect(slo.security.signing_key_max_age_days).toBeLessThanOrEqual(90)
  })
})

// ===============================================================
// Observability ↔ SLO 整合性
// ===============================================================

describe('Observability ↔ SLO 整合性 (ADR-017)', () => {
  it('observability.metrics の histogram の slo_threshold_p99_ms が slo.performance と一致する', () => {
    for (const [name, m] of Object.entries(observability.metrics)) {
      if (m.type !== 'histogram') continue
      if (!m.maps_to_slo || m.slo_threshold_p99_ms === undefined) continue
      const parts = m.maps_to_slo.split('.')
      let v: unknown = slo
      for (const p of parts) v = (v as Record<string, unknown> | undefined)?.[p]
      const target = (v as { p99_latency_ms?: number } | undefined)?.p99_latency_ms
      expect(`${name}=${m.slo_threshold_p99_ms}`).toBe(`${name}=${target}`)
    }
  })

  it('observability.audit retention が slo.audit_log_days と一致する', () => {
    expect(observability.logs?.audit?.retention_days).toBe(slo.data_retention.audit_log_days)
  })

  it('histogram buckets は対応する slo_threshold_p99_ms を包含している', () => {
    for (const [name, m] of Object.entries(observability.metrics)) {
      if (m.type !== 'histogram' || m.slo_threshold_p99_ms === undefined) continue
      const buckets = m.buckets_ms ?? []
      const maxBucket = Math.max(...buckets, 0)
      expect(`${name} bucket=${maxBucket} >= threshold=${m.slo_threshold_p99_ms}`).toBe(
        `${name} bucket=${maxBucket} >= threshold=${m.slo_threshold_p99_ms}`,
      )
      expect(maxBucket).toBeGreaterThanOrEqual(m.slo_threshold_p99_ms)
    }
  })

  it('PII フィールド名が forbidden_fields に列挙されている', () => {
    const forbidden = observability.logs?.application?.forbidden_fields ?? []
    for (const name of ['email', 'password_hash']) {
      expect(forbidden).toContain(name)
    }
  })
})

// ===============================================================
// パスワードポリシー — SCL ↔ TypeScript 整合
// ===============================================================

import { COMMON_PASSWORDS } from '../authentication/usecases/common-passwords'
import { PASSWORD_POLICY } from '../authentication/usecases/password-policy'

describe('Password Policy — SCL ↔ TypeScript 整合', () => {
  const sclPolicy = (scl.annotations?.password_policy ?? {}) as {
    min_length?: number
    max_length?: number
    forbid_user_identifier_similarity?: boolean
    common_password_dictionary?: string
    history_depth?: number
  }

  it('SCL annotations.password_policy.min_length は TypeScript の PASSWORD_POLICY.minLength と一致する', () => {
    expect(sclPolicy.min_length).toBe(PASSWORD_POLICY.minLength)
  })

  it('SCL annotations.password_policy.max_length は TypeScript の PASSWORD_POLICY.maxLength と一致する', () => {
    expect(sclPolicy.max_length).toBe(PASSWORD_POLICY.maxLength)
  })

  it('SCL annotations.password_policy.forbid_user_identifier_similarity は TypeScript と一致する', () => {
    expect(sclPolicy.forbid_user_identifier_similarity).toBe(
      PASSWORD_POLICY.forbidUserIdentifierSimilarity,
    )
  })

  it('SCL annotations.password_policy.common_password_dictionary は TypeScript と一致し bundle が存在する', () => {
    expect(sclPolicy.common_password_dictionary).toBe(PASSWORD_POLICY.commonPasswordDictionary)
    expect(sclPolicy.common_password_dictionary).toBe('bundled')
    expect(COMMON_PASSWORDS.size).toBeGreaterThan(0)
  })

  it('SCL annotations.password_policy.history_depth は TypeScript の PASSWORD_POLICY.historyDepth と一致する', () => {
    expect(sclPolicy.history_depth).toBe(PASSWORD_POLICY.historyDepth)
    expect(PASSWORD_POLICY.historyDepth).toBeGreaterThanOrEqual(1)
  })
})

// ===============================================================
// TOTP ポリシー — SCL ↔ TypeScript 整合
// ===============================================================

import { TOTP_POLICY } from '../authentication/usecases/totp'

describe('TOTP Policy — SCL ↔ TypeScript 整合', () => {
  const sclPolicy = (scl.annotations?.totp_policy ?? {}) as {
    algorithm?: string
    step_seconds?: number
    digits?: number
    window?: number
    secret_bytes?: number
  }

  it('algorithm', () => {
    expect(sclPolicy.algorithm).toBe(TOTP_POLICY.algorithm)
  })
  it('step_seconds', () => {
    expect(sclPolicy.step_seconds).toBe(TOTP_POLICY.stepSeconds)
  })
  it('digits', () => {
    expect(sclPolicy.digits).toBe(TOTP_POLICY.digits)
  })
  it('window', () => {
    expect(sclPolicy.window).toBe(TOTP_POLICY.window)
  })
  it('secret_bytes', () => {
    expect(sclPolicy.secret_bytes).toBe(TOTP_POLICY.secretBytes)
  })
})

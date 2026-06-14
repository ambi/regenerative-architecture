/**
 * Layer 3 — Specification Binding (TypeScript)
 *
 * 状態機械の本体は spec/scl.yaml の `states` セクション。
 * このファイルは TypeScript の narrow タプル・実行ユーティリティを提供するバインディング。
 * SCL との整合は ../invariants.test.ts で検証する。
 */

import {
  scl,
  transitionsAsWireMap,
  terminalWireStates,
  AUTH_CODE_FLOW,
  DEVICE_CODE_FLOW,
} from '../scl'

// ---------------------------------------------------------------
// Authorization Code Flow
// ---------------------------------------------------------------

export const AUTH_CODE_STATES = [
  'received',
  'authentication_pending',
  'authenticated',
  'consent_pending',
  'consented',
  'code_issued',
  'exchanged',
  'rejected',
  'expired',
] as const
export type AuthCodeState = (typeof AUTH_CODE_STATES)[number]

export const AUTH_CODE_EVENTS = [
  'validate',
  'authenticate_user',
  'request_consent',
  'grant_consent',
  'issue_code',
  'redeem_code',
  'reject',
  'expire',
] as const
export type AuthCodeEvent = (typeof AUTH_CODE_EVENTS)[number]

export const AUTH_CODE_TRANSITIONS = transitionsAsWireMap(AUTH_CODE_FLOW) as Readonly<
  Record<AuthCodeState, Partial<Record<AuthCodeEvent, AuthCodeState>>>
>

const AUTH_CODE_TERMINAL = new Set<string>(terminalWireStates(AUTH_CODE_FLOW))

export function transitionAuthCode(
  from: AuthCodeState,
  event: AuthCodeEvent,
): AuthCodeState | null {
  return (AUTH_CODE_TRANSITIONS[from]?.[event] as AuthCodeState | undefined) ?? null
}

export function isAuthCodeTerminal(state: AuthCodeState): boolean {
  return AUTH_CODE_TERMINAL.has(state)
}

// ---------------------------------------------------------------
// Device Code Flow
// ---------------------------------------------------------------

export const DEVICE_CODE_STATES = [
  'issued',
  'user_code_entered',
  'approved',
  'denied',
  'exchanged',
  'expired',
] as const
export type DeviceCodeState = (typeof DEVICE_CODE_STATES)[number]

/**
 * SCL `states.DeviceCodeFlow.transitions` から派生。
 * `slow_down` はポーリング応答（OAuthErrorCode）であり遷移イベントではないため含まない。
 */
export const DEVICE_CODE_EVENTS = [
  'enter_user_code',
  'approve',
  'deny',
  'exchange',
  'expire',
] as const
export type DeviceCodeEvent = (typeof DEVICE_CODE_EVENTS)[number]

export const DEVICE_CODE_TRANSITIONS = transitionsAsWireMap(DEVICE_CODE_FLOW) as Readonly<
  Record<DeviceCodeState, Partial<Record<DeviceCodeEvent, DeviceCodeState>>>
>

const DEVICE_CODE_TERMINAL = new Set<string>(terminalWireStates(DEVICE_CODE_FLOW))

export function transitionDeviceCode(
  from: DeviceCodeState,
  event: DeviceCodeEvent,
): DeviceCodeState | null {
  return (DEVICE_CODE_TRANSITIONS[from]?.[event] as DeviceCodeState | undefined) ?? null
}

export function isDeviceCodeTerminal(state: DeviceCodeState): boolean {
  return DEVICE_CODE_TERMINAL.has(state)
}

/**
 * RFC 8628 §3.5 のポーリング動作仕様（SCL states.DeviceCodeFlow.polling）。
 */
const devicePolling = scl.states[DEVICE_CODE_FLOW]?.polling as
  | {
      default_interval_seconds: number
      slow_down_increment_seconds: number
      responses: Record<string, string>
    }
  | undefined

export const DEVICE_CODE_POLLING = devicePolling ?? {
  default_interval_seconds: 5,
  slow_down_increment_seconds: 5,
  responses: {},
}

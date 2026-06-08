/**
 * Layer 3 — Application Logic
 *
 * verification_uri (RFC 8628 §3.3) でのユーザーによる user_code 承認 / 拒否。
 *
 * 状態遷移は spec/scl.yaml states.DeviceCodeFlow.transitions に従う:
 *   issued --enter_user_code--> user_code_entered --approve--> approved
 *                                                 --deny----> denied
 *
 * ユーザー認証はアダプタ層 (本アプリでは X-User-Sub ヘッダー) で済んでいる前提。
 */

import { OAuthError } from '../protocol/oauth-error'
import { isDeviceExpired, normalizeUserCode } from '../domain/device-authorization'
import {
  transitionDeviceCode,
  isDeviceCodeTerminal,
  type DeviceCodeState,
} from '../../spec-bindings/flows/flows'
import type { DeviceAuthorization, DomainEvent } from '../../spec-bindings/schemas'
import type { DeviceCodeStore } from '../ports/device-code-store'

export interface VerifyUserCodeInput {
  user_code: string
  sub: string
  auth_time: number
  action: 'allow' | 'deny'
}

export async function verifyUserCodeUseCase(
  deps: { deviceCodeStore: DeviceCodeStore },
  input: VerifyUserCodeInput,
  emit: (e: DomainEvent) => void,
  now: Date = new Date(),
): Promise<{ result: 'approved' | 'denied'; clientId: string }> {
  const normalized = normalizeUserCode(input.user_code)
  const rec = await deps.deviceCodeStore.findByUserCode(normalized)
  if (!rec) {
    throw new OAuthError('invalid_request', 'user_code が無効です')
  }
  if (isDeviceExpired(rec, now)) {
    throw new OAuthError('expired_token', 'user_code の有効期限が切れています')
  }
  if (isDeviceCodeTerminal(rec.state as DeviceCodeState)) {
    throw new OAuthError('invalid_request', 'この user_code はすでに処理済みです')
  }

  // issued → user_code_entered (まだコード入力前の状態なら進める)
  let state = rec.state as DeviceCodeState
  if (state === 'issued') {
    state = transitionDeviceCode(state, 'enter_user_code') ?? state
  }

  const event = input.action === 'allow' ? 'approve' : 'deny'
  const next = transitionDeviceCode(state, event)
  if (!next) {
    throw new OAuthError('invalid_request', `現在の状態 (${state}) では ${event} できません`)
  }

  const updated: DeviceAuthorization = {
    ...rec,
    state: next,
    ...(input.action === 'allow' ? { sub: input.sub, auth_time: input.auth_time } : {}),
  }
  await deps.deviceCodeStore.update(updated)

  const occurredAt = now.toISOString()
  if (input.action === 'allow') {
    emit({
      type: 'DeviceAuthorizationApproved',
      occurredAt,
      clientId: rec.client_id,
      sub: input.sub,
    })
    return { result: 'approved', clientId: rec.client_id }
  }
  emit({
    type: 'DeviceAuthorizationDenied',
    occurredAt,
    clientId: rec.client_id,
    sub: input.sub,
  })
  return { result: 'denied', clientId: rec.client_id }
}

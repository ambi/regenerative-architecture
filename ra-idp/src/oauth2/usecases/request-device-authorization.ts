/**
 * Layer 3 — Application Logic
 *
 * /device_authorization (RFC 8628 §3.1)。
 * device_code / user_code を発行し、issued 状態のレコードを保存する。
 *
 * クライアント認証はアダプタ層 (authenticateClient) で先行実行される。
 * ここではグラント宣言とスコープ部分集合性を検証する。
 */

import { OAuthError } from '../protocol/oauth-error'
import {
  generateDeviceCode,
  generateUserCode,
  hashDeviceCode,
  normalizeUserCode,
  DEVICE_CODE_TTL_SECONDS,
} from '../domain/device-authorization'
import {
  DeviceAuthorizationSchema,
  type DeviceAuthorization,
  type Client,
} from '../../spec-bindings/schemas'
import { DEVICE_CODE_POLLING } from '../../spec-bindings/flows/flows'
import type { DeviceCodeStore } from '../ports/device-code-store'

const DEVICE_GRANT = 'urn:ietf:params:oauth:grant-type:device_code'

export interface RequestDeviceAuthorizationInput {
  client: Client
  scope?: string
}

export interface DeviceAuthorizationResponse {
  device_code: string
  user_code: string
  verification_uri: string
  verification_uri_complete: string
  expires_in: number
  interval: number
}

export async function requestDeviceAuthorizationUseCase(
  deps: { deviceCodeStore: DeviceCodeStore; issuer: string },
  input: RequestDeviceAuthorizationInput,
  now: Date = new Date(),
): Promise<{ response: DeviceAuthorizationResponse; record: DeviceAuthorization }> {
  const { client } = input

  if (!client.grant_types.includes(DEVICE_GRANT)) {
    throw new OAuthError(
      'unauthorized_client',
      'クライアントは device_code グラントを宣言していません',
    )
  }

  // スコープ部分集合性 (client_credentials と同じ規則)
  const declared = client.scope.split(/\s+/).filter(Boolean)
  const requested = (input.scope ?? client.scope).split(/\s+/).filter(Boolean)
  if (!requested.every((s) => declared.includes(s))) {
    throw new OAuthError('invalid_scope', '宣言外のスコープが含まれます')
  }

  const deviceCode = generateDeviceCode()
  const userCode = generateUserCode()
  const interval = DEVICE_CODE_POLLING.default_interval_seconds

  const record: DeviceAuthorization = DeviceAuthorizationSchema.parse({
    device_code_hash: hashDeviceCode(deviceCode),
    tenant_id: client.tenant_id,
    user_code: normalizeUserCode(userCode),
    client_id: client.client_id,
    scopes: requested,
    state: 'issued',
    interval_seconds: interval,
    issued_at: now.toISOString(),
    expires_at: new Date(now.getTime() + DEVICE_CODE_TTL_SECONDS * 1000).toISOString(),
  })
  await deps.deviceCodeStore.save(record)

  const base = deps.issuer.replace(/\/$/, '')
  const verification_uri = `${base}/device`
  return {
    response: {
      device_code: deviceCode,
      user_code: userCode,
      verification_uri,
      verification_uri_complete: `${verification_uri}?user_code=${encodeURIComponent(userCode)}`,
      expires_in: DEVICE_CODE_TTL_SECONDS,
      interval,
    },
    record,
  }
}

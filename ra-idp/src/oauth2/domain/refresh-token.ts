/**
 * Layer 3 — Application Logic
 *
 * リフレッシュトークンのドメインモデル。
 * ADR-004 に従いローテーション + ファミリー失効を実装する基盤を提供する。
 *
 * 仕様核 `spec/scl.yaml` の models.RefreshTokenRecord（派生: `gen/RefreshTokenRecord.json`）の制約に従う。
 * トークン文字列自体は不透明 (opaque) で、ストアレコードに SHA-256 ハッシュとして保存する。
 */

import { randomBytes, createHash, randomUUID } from 'crypto'
import { RefreshTokenRecordSchema, type RefreshTokenRecord } from '../../spec-bindings/schemas'

export type { RefreshTokenRecord }

const TOKEN_BYTES = 48 // 384 ビット
const DEFAULT_TTL_SECONDS = 60 * 60 * 24 * 14 // 14 日
const ABSOLUTE_TTL_SECONDS = 60 * 60 * 24 * 30 // 30 日（ADR-004）

export interface GeneratedRefreshToken {
  /** クライアントに返却するトークン文字列（不透明）。 */
  token: string
  /** ストアに保存するレコード。 */
  record: RefreshTokenRecord
}

export function hashToken(token: string): string {
  return createHash('sha256').update(token).digest('hex')
}

export function generateInitial(input: {
  tenant_id: string
  client_id: string
  sub: string
  scopes: string[]
  sender_constraint?: RefreshTokenRecord['sender_constraint']
  now?: Date
}): GeneratedRefreshToken {
  const now = input.now ?? new Date()
  const token = randomBytes(TOKEN_BYTES).toString('base64url')
  const familyId = randomUUID()
  const record = RefreshTokenRecordSchema.parse({
    id: randomUUID(),
    tenant_id: input.tenant_id,
    hash: hashToken(token),
    family_id: familyId,
    client_id: input.client_id,
    sub: input.sub,
    scopes: input.scopes,
    issued_at: now.toISOString(),
    expires_at: new Date(now.getTime() + DEFAULT_TTL_SECONDS * 1000).toISOString(),
    absolute_expires_at: new Date(now.getTime() + ABSOLUTE_TTL_SECONDS * 1000).toISOString(),
    revoked: false,
    rotated: false,
    sender_constraint: input.sender_constraint ?? null,
  })
  return { token, record }
}

export function rotate(parent: RefreshTokenRecord, now: Date = new Date()): GeneratedRefreshToken {
  const token = randomBytes(TOKEN_BYTES).toString('base64url')
  const record = RefreshTokenRecordSchema.parse({
    id: randomUUID(),
    tenant_id: parent.tenant_id,
    hash: hashToken(token),
    family_id: parent.family_id,
    parent_id: parent.id,
    client_id: parent.client_id,
    sub: parent.sub,
    scopes: parent.scopes,
    issued_at: now.toISOString(),
    // expires_at は再延長、ただし absolute_expires_at は親から継承
    expires_at: new Date(
      Math.min(now.getTime() + DEFAULT_TTL_SECONDS * 1000, Date.parse(parent.absolute_expires_at)),
    ).toISOString(),
    absolute_expires_at: parent.absolute_expires_at,
    revoked: false,
    rotated: false,
    sender_constraint: parent.sender_constraint ?? null,
  })
  return { token, record }
}

export function isReplay(record: RefreshTokenRecord): boolean {
  return record.rotated || record.revoked
}

export function isAbsoluteExpired(record: RefreshTokenRecord, now: Date = new Date()): boolean {
  return now.getTime() >= Date.parse(record.absolute_expires_at)
}

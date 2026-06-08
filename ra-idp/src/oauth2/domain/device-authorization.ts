/**
 * Layer 3 — Application Logic（ドメイン: Device Authorization Grant, RFC 8628）
 *
 * device_code / user_code の生成と判定。状態機械本体は
 * spec/scl.yaml states.DeviceCodeFlow（src/spec-bindings/flows/flows.ts 経由で参照）。
 *
 * セキュリティ要件 (RFC 8628 §5.1 / §5.2):
 *   - device_code は推測不能なベアラ秘密 (高エントロピー、ハッシュ保存)
 *   - user_code は人間が入力するため短いが、十分なエントロピー + 入力レート制限で
 *     総当たりを防ぐ
 */

import { randomBytes, randomInt, createHash } from 'crypto'

/**
 * user_code の文字集合。
 * 母音を除いた子音 + 数字を避けることで:
 *   - 偶発的に意味のある単語ができない
 *   - 0/O, 1/I/L のような視認混同を避ける
 * RFC 8628 §6.1 の推奨に沿う。
 */
const USER_CODE_CHARSET = 'BCDFGHJKLMNPQRSTVWXZ' // 20 文字
const USER_CODE_LENGTH = 8 // 20^8 ≈ 2.56e10 (約 34 bit)

/**
 * device_code / user_code の寿命 (秒)。RFC 8628 §3.2 の expires_in。
 * authorization_code (60s) より長く、ユーザーが別デバイスで承認する時間を与える。
 * Redis volatile store の TTL もこの値を使う。
 */
export const DEVICE_CODE_TTL_SECONDS = 600

/** device_code: 32 バイトのランダム base64url (ベアラ秘密)。 */
export function generateDeviceCode(): string {
  return randomBytes(32).toString('base64url')
}

/** device_code はハッシュのみ保存する (refresh token と同じ方針)。 */
export function hashDeviceCode(deviceCode: string): string {
  return createHash('sha256').update(deviceCode).digest('hex')
}

/**
 * user_code を生成する。表示は "WDJB-MJHT" 形式 (ハイフン区切り)。
 * randomInt で偏りのない選択を行う。
 */
export function generateUserCode(): string {
  let raw = ''
  for (let i = 0; i < USER_CODE_LENGTH; i++) {
    raw += USER_CODE_CHARSET[randomInt(USER_CODE_CHARSET.length)]
  }
  return `${raw.slice(0, 4)}-${raw.slice(4)}`
}

/** 入力された user_code を索引キーに正規化する (大文字化・ハイフン/空白除去)。 */
export function normalizeUserCode(input: string): string {
  return input.toUpperCase().replace(/[^A-Z0-9]/g, '')
}

export function isDeviceExpired(
  rec: { expires_at: string; state: string },
  now: Date = new Date(),
): boolean {
  if (rec.state === 'expired') return true
  return Date.parse(rec.expires_at) <= now.getTime()
}

/**
 * Layer 3 — Application Logic (TOTP / RFC 6238)
 *
 * Time-based One-Time Password (RFC 6238) を HMAC-SHA1 で計算する pure 関数群。
 * T0 = 0 / STEP = 30s / DIGITS = 6 という Authenticator アプリ互換の固定パラメータで
 * 動作する。secret は raw bytes (Uint8Array) で扱い、永続化は base32 文字列で行う。
 */

import { createHmac, randomBytes } from 'node:crypto'

/**
 * spec/scl.yaml annotations.totp_policy と双子定義。乖離すると spec↔impl drift と
 * なるため invariants.test.ts で突き合わせる。
 */
export const TOTP_POLICY = {
  algorithm: 'SHA1',
  stepSeconds: 30,
  digits: 6,
  window: 1,
  /** RFC 4226 §4 推奨の 160-bit。 */
  secretBytes: 20,
} as const

const STEP_SECONDS = TOTP_POLICY.stepSeconds
const DIGITS = TOTP_POLICY.digits
const DEFAULT_WINDOW = TOTP_POLICY.window
const SECRET_BYTES = TOTP_POLICY.secretBytes

const BASE32_ALPHABET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'

/** 暗号学的乱数で TOTP シークレットを 1 つ生成し base32 で返す。 */
export function generateTotpSecret(): string {
  return base32Encode(randomBytes(SECRET_BYTES))
}

/**
 * RFC 6238 §4: code = HOTP(secret, T) where T = floor((now - T0) / step)。
 * 戻り値は 0 埋め 6 桁の文字列。
 */
export function generateTotp(secretBase32: string, atUnixSeconds: number): string {
  const counter = Math.floor(atUnixSeconds / STEP_SECONDS)
  const secret = base32Decode(secretBase32)
  // 8-byte big-endian counter (RFC 4226 §5.1)
  const counterBuf = Buffer.alloc(8)
  counterBuf.writeBigUInt64BE(BigInt(counter))
  const hmac = createHmac('sha1', secret).update(counterBuf).digest()
  // RFC 4226 §5.3 dynamic truncation
  const offset = hmac[hmac.length - 1] & 0x0f
  const bin =
    ((hmac[offset] & 0x7f) << 24) |
    ((hmac[offset + 1] & 0xff) << 16) |
    ((hmac[offset + 2] & 0xff) << 8) |
    (hmac[offset + 3] & 0xff)
  return (bin % 10 ** DIGITS).toString().padStart(DIGITS, '0')
}

/**
 * 提示されたコードを現在ステップ ± window で照合する (時計ズレ吸収)。
 * 比較は length-equal な timing-safe 比較で行う。
 */
export function verifyTotp(
  secretBase32: string,
  submittedCode: string,
  atUnixSeconds: number,
  window: number = DEFAULT_WINDOW,
): boolean {
  if (submittedCode.length !== DIGITS) return false
  if (!/^\d+$/.test(submittedCode)) return false
  for (let i = -window; i <= window; i++) {
    const candidate = generateTotp(secretBase32, atUnixSeconds + i * STEP_SECONDS)
    if (timingSafeEqualString(candidate, submittedCode)) return true
  }
  return false
}

/**
 * Authenticator アプリ向けの otpauth:// URI を組み立てる (Google Authenticator
 * Key URI Format)。QR エンコード対象として SPA に渡す。
 */
export function buildOtpauthUri(input: {
  secretBase32: string
  accountName: string
  issuer: string
}): string {
  const params = new URLSearchParams({
    secret: input.secretBase32,
    issuer: input.issuer,
    algorithm: 'SHA1',
    digits: String(DIGITS),
    period: String(STEP_SECONDS),
  })
  // ラベルは "Issuer:account" 形式が慣例。
  const label = encodeURIComponent(`${input.issuer}:${input.accountName}`)
  return `otpauth://totp/${label}?${params.toString()}`
}

function timingSafeEqualString(a: string, b: string): boolean {
  if (a.length !== b.length) return false
  let diff = 0
  for (let i = 0; i < a.length; i++) {
    diff |= a.charCodeAt(i) ^ b.charCodeAt(i)
  }
  return diff === 0
}

/** RFC 4648 §6 base32 (padding なし大文字)。 */
function base32Encode(bytes: Uint8Array): string {
  let bits = 0
  let value = 0
  let out = ''
  for (const b of bytes) {
    value = (value << 8) | b
    bits += 8
    while (bits >= 5) {
      out += BASE32_ALPHABET[(value >>> (bits - 5)) & 0x1f]
      bits -= 5
    }
  }
  if (bits > 0) {
    out += BASE32_ALPHABET[(value << (5 - bits)) & 0x1f]
  }
  return out
}

function base32Decode(input: string): Uint8Array {
  const cleaned = input.replace(/=+$/, '').toUpperCase()
  let bits = 0
  let value = 0
  const out: number[] = []
  for (const ch of cleaned) {
    const idx = BASE32_ALPHABET.indexOf(ch)
    if (idx < 0) throw new Error(`invalid base32 character: ${ch}`)
    value = (value << 5) | idx
    bits += 5
    if (bits >= 8) {
      out.push((value >>> (bits - 8)) & 0xff)
      bits -= 8
    }
  }
  return Uint8Array.from(out)
}

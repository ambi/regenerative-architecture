/**
 * Layer 4 — Adapter: HMAC stateless DPoP-Nonce サービス。
 *
 * nonce = base64url( timestamp(8B big-endian) || HMAC-SHA256(timestamp, secret) )
 *
 * stateless なため複数インスタンス間で共有秘密を持つだけで一致する。
 * TTL を過ぎた nonce は verify で拒否する。
 */

import { createHmac, randomBytes, timingSafeEqual } from 'crypto'
import type { DpopNonceService } from '../../src/oauth2/ports/dpop-nonce-service'

const TIMESTAMP_BYTES = 8
const MAC_BYTES = 32

export class HmacDpopNonceService implements DpopNonceService {
  constructor(
    private readonly secret: Buffer,
    private readonly ttlSeconds: number,
  ) {}

  static withRandomSecret(ttlSeconds: number): HmacDpopNonceService {
    return new HmacDpopNonceService(randomBytes(32), ttlSeconds)
  }

  issue(now: Date = new Date()): string {
    const tsBuf = Buffer.alloc(TIMESTAMP_BYTES)
    tsBuf.writeBigInt64BE(BigInt(Math.floor(now.getTime() / 1000)))
    const mac = createHmac('sha256', this.secret).update(tsBuf).digest()
    return Buffer.concat([tsBuf, mac]).toString('base64url')
  }

  verify(nonce: string, now: Date = new Date()): boolean {
    let buf: Buffer
    try {
      buf = Buffer.from(nonce, 'base64url')
    } catch {
      return false
    }
    if (buf.length !== TIMESTAMP_BYTES + MAC_BYTES) return false
    const tsBuf = buf.subarray(0, TIMESTAMP_BYTES)
    const mac = buf.subarray(TIMESTAMP_BYTES)
    const expected = createHmac('sha256', this.secret).update(tsBuf).digest()
    if (!timingSafeEqual(mac, expected)) return false
    const issuedAt = Number(tsBuf.readBigInt64BE())
    const ageSeconds = Math.floor(now.getTime() / 1000) - issuedAt
    return ageSeconds >= 0 && ageSeconds <= this.ttlSeconds
  }
}

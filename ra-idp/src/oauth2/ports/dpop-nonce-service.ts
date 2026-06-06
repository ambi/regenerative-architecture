/**
 * Layer 3 — Port: DPoP-Nonce 発行・検証サービス (RFC 9449 §8)。
 *
 * stateless 実装 (HMAC) と stateful 実装 (Redis 等) を差し替え可能にする。
 */

export interface DpopNonceService {
  /** 新しい nonce を発行する。レスポンスの DPoP-Nonce ヘッダーに載せる。 */
  issue(now?: Date): string
  /** nonce が本物 (我々が発行したもの) かつ TTL 内かを判定する。 */
  verify(nonce: string, now?: Date): boolean
}

/**
 * Layer 3 — Application Logic（ポート定義）
 *
 * private_key_jwt クライアント認証 (RFC 7523) の client_assertion jti を
 * 単回使用に制約するためのリプレイ防止ストア。
 *
 * DPoP の jti リプレイ (DpopReplayStore) とは Redis 名前空間・TTL・監査意味論が
 * 異なるため、別ポートとして分離する (同一メカニズムでも責務が違う)。
 */

export interface ClientAssertionReplayStore {
  /**
   * jti が直近 windowSeconds 以内に観測されたかチェックし、
   * 未観測なら記録して true（新規）を返す。観測済みなら false（リプレイ）。
   */
  recordIfNew(jti: string, windowSeconds: number, now?: Date): Promise<boolean>
}

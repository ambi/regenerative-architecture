/**
 * Layer 3 — Application Logic（ポート定義）
 *
 * Device Authorization Grant (RFC 8628) の volatile state。
 * device_code (ハッシュ) と user_code の 2 つの索引でレコードを引ける。
 * TTL は spec の device_code 寿命に従う (Redis 実装では EX で自動失効)。
 */

import type { DeviceAuthorization } from '../spec-bindings/schemas'

export interface DeviceCodeStore {
  save(rec: DeviceAuthorization): Promise<void>
  /** device_code のハッシュで引く (token エンドポイントのポーリング)。 */
  findByDeviceCodeHash(hash: string): Promise<DeviceAuthorization | null>
  /** user_code (正規化済み) で引く (verification_uri の承認画面)。 */
  findByUserCode(userCode: string): Promise<DeviceAuthorization | null>
  /** 状態遷移・ポーリング時刻の更新。TTL は維持する。 */
  update(rec: DeviceAuthorization): Promise<void>
}

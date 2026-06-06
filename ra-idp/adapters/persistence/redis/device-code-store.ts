/**
 * Layer 4 — Adapter Layer (Redis DeviceCodeStore)
 *
 * device_code ハッシュをキー (idp:device:<hash>) に JSON で保存し、
 * user_code → ハッシュの副索引 (idp:device:uc:<userCode>) を同じ TTL で張る。
 *
 * TTL は device_code の寿命 (DEVICE_CODE_TTL_SECONDS)。
 * update は KEEPTTL で寿命を変えずに状態だけ書き換える。
 */

import type { DeviceAuthorization } from '../../../src/spec-bindings/schemas'
import { DeviceAuthorizationSchema } from '../../../src/spec-bindings/schemas'
import type { DeviceCodeStore } from '../../../src/oauth2/ports/device-code-store'
import type { Redis } from './client'

const KEY = 'idp:device:'
const UC_KEY = 'idp:device:uc:'

export class RedisDeviceCodeStore implements DeviceCodeStore {
  constructor(
    private readonly redis: Redis,
    private readonly ttlSeconds: number,
  ) {}

  async save(rec: DeviceAuthorization): Promise<void> {
    await this.redis.set(KEY + rec.device_code_hash, JSON.stringify(rec), 'EX', this.ttlSeconds)
    await this.redis.set(UC_KEY + rec.user_code, rec.device_code_hash, 'EX', this.ttlSeconds)
  }

  async findByDeviceCodeHash(hash: string): Promise<DeviceAuthorization | null> {
    const v = await this.redis.get(KEY + hash)
    if (!v) return null
    return DeviceAuthorizationSchema.parse(JSON.parse(v))
  }

  async findByUserCode(userCode: string): Promise<DeviceAuthorization | null> {
    const hash = await this.redis.get(UC_KEY + userCode)
    if (!hash) return null
    return this.findByDeviceCodeHash(hash)
  }

  async update(rec: DeviceAuthorization): Promise<void> {
    // 状態遷移のみ。device_code の寿命 (TTL) は維持する。
    await this.redis.set(KEY + rec.device_code_hash, JSON.stringify(rec), 'KEEPTTL')
  }
}

/**
 * Layer 4 — Adapter Layer (in-memory DeviceCodeStore)
 *
 * device_code ハッシュをキーに保持し、user_code → ハッシュの副索引を併せ持つ。
 * 本番では Redis adapter (../redis/device-code-store.ts) を使う。
 */

import type { DeviceAuthorization } from '../../../src/spec-bindings/schemas'
import type { DeviceCodeStore } from '../../../src/oauth2/ports/device-code-store'

export class InMemoryDeviceCodeStore implements DeviceCodeStore {
  private readonly byHash = new Map<string, DeviceAuthorization>()
  private readonly userCodeToHash = new Map<string, string>()

  async save(rec: DeviceAuthorization): Promise<void> {
    this.byHash.set(rec.device_code_hash, { ...rec })
    this.userCodeToHash.set(rec.user_code, rec.device_code_hash)
  }

  async findByDeviceCodeHash(hash: string): Promise<DeviceAuthorization | null> {
    const v = this.byHash.get(hash)
    return v ? { ...v } : null
  }

  async findByUserCode(userCode: string): Promise<DeviceAuthorization | null> {
    const hash = this.userCodeToHash.get(userCode)
    if (!hash) return null
    const v = this.byHash.get(hash)
    return v ? { ...v } : null
  }

  async update(rec: DeviceAuthorization): Promise<void> {
    this.byHash.set(rec.device_code_hash, { ...rec })
    this.userCodeToHash.set(rec.user_code, rec.device_code_hash)
  }
}

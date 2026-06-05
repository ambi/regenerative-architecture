/**
 * Layer 4 — Adapter Layer（暗号鍵ストア）
 *
 * jose を使ったインメモリ KeyStore。サンプル用。
 * 本番では KMS / HSM / Vault を使う（ADR-009）。
 */

import { generateKeyPair, exportJWK } from 'jose'
import type { JWK } from 'jose'
import { createHash } from 'crypto'
import type { KeyStore, SigningKey } from '../../src/ports/key-store'

export class InMemoryKeyStore implements KeyStore {
  private readonly keys: SigningKey[] = []
  private activeKid: string | null = null

  static async create(alg: 'PS256' | 'ES256' = 'PS256'): Promise<InMemoryKeyStore> {
    const ks = new InMemoryKeyStore()
    await ks.rotateInternal(alg)
    return ks
  }

  async getActiveKey(): Promise<SigningKey> {
    const k = this.keys.find((x) => x.kid === this.activeKid)
    if (!k) throw new Error('アクティブな署名鍵がありません')
    return k
  }

  async getAllKeys(): Promise<SigningKey[]> {
    return [...this.keys]
  }

  async findByKid(kid: string): Promise<SigningKey | null> {
    return this.keys.find((k) => k.kid === kid) ?? null
  }

  async rotate(): Promise<SigningKey> {
    const previous = await this.getActiveKey()
    return this.rotateInternal(previous.alg)
  }

  private async rotateInternal(alg: 'PS256' | 'ES256'): Promise<SigningKey> {
    const { publicKey, privateKey } = await generateKeyPair(alg, { extractable: true })
    const jwk = await exportJWK(publicKey)
    const kid = computeKid(jwk)
    jwk.kid = kid
    jwk.alg = alg
    jwk.use = 'sig'

    // 旧鍵を inactive にする
    for (const k of this.keys) {
      k.active = false
    }

    const key: SigningKey = {
      kid,
      alg,
      privateKey,
      publicKey,
      publicJwk: jwk as unknown as Record<string, unknown>,
      active: true,
      created_at: new Date().toISOString(),
    }
    this.keys.push(key)
    this.activeKid = kid
    return key
  }
}

/** JWK の SHA-256 サムプリント (RFC 7638) を kid として使う */
function computeKid(jwk: JWK): string {
  // 順序を仕様どおりに整える
  const src = jwk as unknown as Record<string, unknown>
  const ordered: Record<string, unknown> = {}
  for (const k of Object.keys(src).sort()) {
    ordered[k] = src[k]
  }
  const json = JSON.stringify(ordered)
  return createHash('sha256').update(json).digest('base64url')
}

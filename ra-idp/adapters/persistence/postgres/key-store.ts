/**
 * Layer 4 — Adapter Layer (Postgres KeyStore)
 *
 * ADR-009 の署名鍵を durable かつ複数レプリカ間で共有する実装。
 *
 * InMemoryKeyStore はプロセスローカルなので:
 *   - 再起動で全鍵が消える → 既発行トークンが全部検証不能
 *   - レプリカごとに別の鍵 → /jwks が不一致 → 水平スケール不能
 * という本番では致命的な問題がある。本アダプタは signing_keys テーブル
 * (infra/migrations/0001_init.sql) を唯一の鍵ソースとして共有する。
 *
 * 秘密鍵は private_jwk(JSONB) として保存する (現実装の簡略化)。
 * 本番では KMS / HSM に置き、private_jwk カラムは「KMS 内の鍵参照 ID」に置換する
 * (ADR-009「鍵の保管」節)。署名鍵オブジェクト本体の表現はポート層で unknown 扱いなので
 * この差し替えは KeyStore アダプタ内に閉じる。
 */

import { generateKeyPair, exportJWK, importJWK, calculateJwkThumbprint } from 'jose'
import type { JWK } from 'jose'
import type { KeyStore, SigningKey } from '../../../src/oauth2/ports/key-store'
import type { PgPool } from './pool'

export class PostgresKeyStore implements KeyStore {
  /** kid → import 済み鍵オブジェクト。JWK→KeyLike 変換コストを避けるためのキャッシュ。 */
  private readonly importCache = new Map<string, { privateKey: unknown; publicKey: unknown }>()

  private constructor(
    private readonly pool: PgPool,
    private readonly alg: 'PS256' | 'ES256',
  ) {}

  static async create(pool: PgPool, alg: 'PS256' | 'ES256' = 'PS256'): Promise<PostgresKeyStore> {
    const ks = new PostgresKeyStore(pool, alg)
    await ks.ensureActiveKey()
    return ks
  }

  async getActiveKey(): Promise<SigningKey> {
    const { rows } = await this.pool.query(`SELECT * FROM signing_keys WHERE active = TRUE LIMIT 1`)
    if (!rows[0]) throw new Error('アクティブな署名鍵がありません')
    return this.rowToKey(rows[0])
  }

  async getAllKeys(): Promise<SigningKey[]> {
    // archived_at が立った鍵は JWKS から外す (ADR-009)。検証用途には findByKid で個別取得。
    const { rows } = await this.pool.query(
      `SELECT * FROM signing_keys WHERE archived_at IS NULL ORDER BY created_at DESC`,
    )
    return Promise.all(rows.map((r: any) => this.rowToKey(r)))
  }

  async findByKid(kid: string): Promise<SigningKey | null> {
    const { rows } = await this.pool.query(`SELECT * FROM signing_keys WHERE kid = $1`, [kid])
    return rows[0] ? this.rowToKey(rows[0]) : null
  }

  async rotate(): Promise<SigningKey> {
    const generated = await this.generate()
    const client = await this.pool.connect()
    try {
      await client.query('BEGIN')
      // 旧 active をロックして倒す。並行 rotate はこの行ロックで直列化される。
      await client.query(
        `UPDATE signing_keys SET active = FALSE, rotated_at = now() WHERE active = TRUE`,
      )
      await client.query(
        `INSERT INTO signing_keys (kid, alg, public_jwk, private_jwk, active)
         VALUES ($1, $2, $3::jsonb, $4::jsonb, TRUE)`,
        [
          generated.kid,
          generated.alg,
          JSON.stringify(generated.publicJwk),
          JSON.stringify(generated.privateJwk),
        ],
      )
      await client.query('COMMIT')
    } catch (err) {
      await client.query('ROLLBACK')
      throw err
    } finally {
      client.release()
    }
    return this.getActiveKey()
  }

  /** active 鍵が無ければ 1 つ生成して挿入する (競合安全)。 */
  private async ensureActiveKey(): Promise<void> {
    const { rows } = await this.pool.query(`SELECT 1 FROM signing_keys WHERE active = TRUE LIMIT 1`)
    if (rows.length > 0) return
    const generated = await this.generate()
    // 部分一意インデックス signing_keys_single_active_idx により、
    // 複数レプリカが同時シードしても最初の 1 つだけが active になる。
    await this.pool.query(
      `INSERT INTO signing_keys (kid, alg, public_jwk, private_jwk, active)
       VALUES ($1, $2, $3::jsonb, $4::jsonb, TRUE)
       ON CONFLICT DO NOTHING`,
      [
        generated.kid,
        generated.alg,
        JSON.stringify(generated.publicJwk),
        JSON.stringify(generated.privateJwk),
      ],
    )
  }

  private async generate(): Promise<{
    kid: string
    alg: 'PS256' | 'ES256'
    publicJwk: JWK
    privateJwk: JWK
  }> {
    const { publicKey, privateKey } = await generateKeyPair(this.alg, { extractable: true })
    const publicJwk = await exportJWK(publicKey)
    const privateJwk = await exportJWK(privateKey)
    const kid = await calculateJwkThumbprint(publicJwk) // RFC 7638
    publicJwk.kid = kid
    publicJwk.alg = this.alg
    publicJwk.use = 'sig'
    privateJwk.kid = kid
    privateJwk.alg = this.alg
    return { kid, alg: this.alg, publicJwk, privateJwk }
  }

  private async rowToKey(row: any): Promise<SigningKey> {
    const alg = row.alg as 'PS256' | 'ES256'
    let cached = this.importCache.get(row.kid)
    if (!cached) {
      cached = {
        privateKey: await importJWK(row.private_jwk as JWK, alg),
        publicKey: await importJWK(row.public_jwk as JWK, alg),
      }
      this.importCache.set(row.kid, cached)
    }
    return {
      kid: row.kid,
      alg,
      privateKey: cached.privateKey,
      publicKey: cached.publicKey,
      publicJwk: row.public_jwk as Record<string, unknown>,
      active: row.active,
      created_at: row.created_at instanceof Date ? row.created_at.toISOString() : row.created_at,
    }
  }
}

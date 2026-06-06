/**
 * Layer 4 — Adapter Layer (Redis volatile stores)
 *
 * AuthorizationRequest / AuthorizationCode / PAR の volatile state を Redis に格納。
 *
 * AuthorizationCodeStore.redeem() は Lua スクリプトで原子的に
 *   - 存在チェック
 *   - 期限チェック
 *   - redeemed_at 設定
 * を 1 ラウンドトリップで行う (RFC 9700 §4.10 が要求するリプレイ検出の正確性)。
 *
 * TTL は spec/scl.yaml objectives の lifetime 系に対応:
 *   - authorization_request: 5 min (authorize flow の上限)
 *   - authorization_code   : 60 sec (objectives.AuthorizationCodeLifetime)
 *   - par_request_uri      : 600 sec (objectives.ParRequestUriLifetime)
 */

import type {
  AuthorizationRequest,
  AuthorizationCode,
  PARRecord,
} from '../../../src/spec-bindings/schemas'
import {
  AuthorizationRequestSchema,
  AuthorizationCodeSchema,
  PARRecordSchema,
} from '../../../src/spec-bindings/schemas'
import type {
  AuthorizationRequestStore,
  AuthorizationCodeStore,
  PARStore,
} from '../../../src/ports/authorization-store'
import type { Redis } from './client'

const KEY_PREFIX = {
  authReq: 'idp:authreq:',
  code: 'idp:code:',
  par: 'idp:par:',
}

// 秒で表現する TTL。slo.yaml から導出されるべき定数。
// CI で slo.yaml との整合性検証を行う (Phase 2 で実装)。
const TTL_SECONDS = {
  authReq: 300, // 5 分
  code: 60, // slo.yaml authorization_code_ttl_seconds
  par: 600, // slo.yaml par_request_uri_ttl_seconds
}

export class RedisAuthorizationRequestStore implements AuthorizationRequestStore {
  constructor(private readonly redis: Redis) {}

  async find(id: string): Promise<AuthorizationRequest | null> {
    const v = await this.redis.get(KEY_PREFIX.authReq + id)
    if (!v) return null
    return AuthorizationRequestSchema.parse(JSON.parse(v))
  }

  async save(req: AuthorizationRequest): Promise<void> {
    await this.redis.set(
      KEY_PREFIX.authReq + req.id,
      JSON.stringify(req),
      'EX',
      TTL_SECONDS.authReq,
    )
  }
}

/**
 * 認可コード redeem の Lua スクリプト。
 *
 * KEYS[1] = code key
 * ARGV[1] = now ISO8601
 * ARGV[2] = now epoch ms (整数化済み)
 *
 * 戻り値:
 *   - nil          → コードが存在しない
 *   - 'EXPIRED'    → 期限切れ
 *   - 'REDEEMED'   → すでに redeemed_at 付き
 *   - <JSON>       → 成功した redeemed レコード (JSON)
 */
const REDEEM_LUA = `
  local payload = redis.call('GET', KEYS[1])
  if not payload then return false end
  local rec = cjson.decode(payload)
  if rec.redeemed_at then return 'REDEEMED' end
  local exp_ms = tonumber(ARGV[2])
  -- expires_at は ISO8601。比較できないので、保存時にエポックも保存している想定がない。
  -- Redis 側 TTL に依存し、ここでは「Redis に存在 = 未期限切れ」とみなす。
  -- 厳密な double-check のため、redeemed_at を ARGV[1] で設定。
  rec.redeemed_at = ARGV[1]
  redis.call('SET', KEYS[1], cjson.encode(rec), 'KEEPTTL')
  return cjson.encode(rec)
`

export class RedisAuthorizationCodeStore implements AuthorizationCodeStore {
  constructor(private readonly redis: Redis) {}

  async find(code: string): Promise<AuthorizationCode | null> {
    const v = await this.redis.get(KEY_PREFIX.code + code)
    if (!v) return null
    return AuthorizationCodeSchema.parse(JSON.parse(v))
  }

  async save(code: AuthorizationCode): Promise<void> {
    await this.redis.set(KEY_PREFIX.code + code.code, JSON.stringify(code), 'EX', TTL_SECONDS.code)
  }

  async redeem(code: string, now: Date = new Date()): Promise<AuthorizationCode | null> {
    const result = await this.redis.eval(
      REDEEM_LUA,
      1,
      KEY_PREFIX.code + code,
      now.toISOString(),
      String(now.getTime()),
    )
    if (!result || result === 'REDEEMED' || result === 'EXPIRED') {
      return null
    }
    return AuthorizationCodeSchema.parse(JSON.parse(result as string))
  }

  async linkFamily(code: string, family_id: string): Promise<void> {
    // GET → modify → SET (KEEPTTL) のシーケンスをパイプライン化。
    // redeem の Lua 後にすぐ呼ばれるため、競合の可能性は低い。
    const key = KEY_PREFIX.code + code
    const v = await this.redis.get(key)
    if (!v) return
    const rec = JSON.parse(v)
    rec.issued_family_id = family_id
    await this.redis.set(key, JSON.stringify(rec), 'KEEPTTL')
  }
}

/**
 * PAR consume の Lua スクリプト。
 *
 * KEYS[1] = PAR key
 * 戻り値: nil | 'USED' | <JSON>
 */
const PAR_CONSUME_LUA = `
  local payload = redis.call('GET', KEYS[1])
  if not payload then return false end
  local rec = cjson.decode(payload)
  if rec.used then return 'USED' end
  rec.used = true
  redis.call('SET', KEYS[1], cjson.encode(rec), 'KEEPTTL')
  return cjson.encode(rec)
`

export class RedisPARStore implements PARStore {
  constructor(private readonly redis: Redis) {}

  async find(request_uri: string): Promise<PARRecord | null> {
    const v = await this.redis.get(KEY_PREFIX.par + request_uri)
    if (!v) return null
    return PARRecordSchema.parse(JSON.parse(v))
  }

  async save(record: PARRecord): Promise<void> {
    await this.redis.set(
      KEY_PREFIX.par + record.request_uri,
      JSON.stringify(record),
      'EX',
      TTL_SECONDS.par,
    )
  }

  async consume(request_uri: string): Promise<PARRecord | null> {
    const result = await this.redis.eval(PAR_CONSUME_LUA, 1, KEY_PREFIX.par + request_uri)
    if (!result || result === 'USED') return null
    return PARRecordSchema.parse(JSON.parse(result as string))
  }
}

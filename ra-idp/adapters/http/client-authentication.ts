/**
 * Layer 4 — Adapter Layer（クライアント認証ミドルウェア）
 *
 * トークンエンドポイント・PAR・introspection・revocation で共通に使う
 * クライアント認証ロジック。
 *
 * ADR-008 に従い、本アプリでは以下を実装する:
 *   - client_secret_basic / client_secret_post / none
 *   - private_key_jwt (RFC 7523, ADR-008 推奨方式)
 * tls_client_auth はランタイム (mTLS 終端) 依存のため枠組みのみ。
 */

import { createHash, timingSafeEqual } from 'crypto'
import {
  jwtVerify,
  decodeJwt,
  decodeProtectedHeader,
  createLocalJWKSet,
  createRemoteJWKSet,
} from 'jose'
import type { JWK } from 'jose'
import type { Context } from 'hono'
import { OAuthError } from '../../src/oauth2/protocol/oauth-error'
import type { Client } from '../../src/spec-bindings/schemas'
import type { ClientRepository } from '../../src/oauth2/ports/client-repository'
import type { ClientAssertionReplayStore } from '../../src/oauth2/ports/client-assertion-replay-store'
import { clientCertSubjectMatches, parseClientCertificateHeader } from '../crypto/mtls-client-cert'
import { requestTenantId } from './middleware/tenant-middleware'

/** TLS 終端プロキシが検証済みクライアント証明書を載せるヘッダ名 (ADR-005)。 */
export const CLIENT_CERT_HEADER = 'X-Client-Certificate'

/** RFC 7523 §2.2 で固定された client_assertion_type 値 */
const CLIENT_ASSERTION_TYPE = 'urn:ietf:params:oauth:client-assertion-type:jwt-bearer'
/** client_assertion の最大寿命。これより長寿命のものは拒否しリプレイ窓を有界にする。 */
const MAX_ASSERTION_LIFETIME_SECONDS = 300
/** クロックスキュー許容 (DPoP と揃える) */
const CLOCK_SKEW_SECONDS = 60

export interface AuthenticatedClient {
  client: Client
  method: Client['token_endpoint_auth_method']
  /** tls_client_auth で認証された場合の提示証明書の SHA-256 サムプリント (cnf.x5t#S256)。 */
  mtlsThumbprintS256?: string
}

/**
 * private_key_jwt 検証に必要なランタイム依存。
 * client_secret_basic / post / none のみのリクエストでは不要。
 */
export interface ClientAuthOptions {
  issuer: string
  clientAssertionReplayStore: ClientAssertionReplayStore
}

export async function authenticateClient(
  c: Context,
  body: Record<string, string>,
  clientRepo: ClientRepository,
  opts?: ClientAuthOptions,
): Promise<AuthenticatedClient> {
  const basicAuth = c.req.header('Authorization')
  const hasAssertion = Boolean(body.client_assertion || body.client_assertion_type)

  // 0. private_key_jwt (RFC 7523) — client_assertion が存在する場合
  if (hasAssertion) {
    // RFC 6749 §2.3: 複数のクライアント認証方式の同時使用は禁止
    if (basicAuth?.startsWith('Basic ') || body.client_secret) {
      throw new OAuthError('invalid_request', '複数のクライアント認証方式が混在しています')
    }
    if (body.client_assertion_type !== CLIENT_ASSERTION_TYPE) {
      throw new OAuthError('invalid_request', `未対応の client_assertion_type です`)
    }
    if (!body.client_assertion) {
      throw new OAuthError('invalid_request', 'client_assertion が必要です')
    }
    if (!opts) {
      // 合成ルートの構成漏れ。クライアントのせいではないので 5xx 系。
      throw new OAuthError('server_error', 'private_key_jwt 検証の依存が構成されていません')
    }
    const url = new URL(c.req.url)
    const requestUrl = `${url.origin}${url.pathname}`
    const audiences = buildAcceptableAudiences(opts.issuer, requestUrl)
    const client = await verifyClientAssertion(body.client_assertion, clientRepo, {
      tenantId: requestTenantId(c),
      audiences,
      replayStore: opts.clientAssertionReplayStore,
    })
    return { client, method: 'private_key_jwt' }
  }

  // 1. Basic 認証ヘッダーをチェック
  if (basicAuth?.startsWith('Basic ')) {
    const decoded = Buffer.from(basicAuth.slice(6), 'base64').toString('utf8')
    const idx = decoded.indexOf(':')
    if (idx < 0) throw new OAuthError('invalid_client', 'Basic 認証のフォーマット不正')
    const id = decodeURIComponent(decoded.slice(0, idx))
    const secret = decodeURIComponent(decoded.slice(idx + 1))
    const client = await loadClient(clientRepo, requestTenantId(c), id)
    verifySecret(client, secret, 'client_secret_basic')
    return { client, method: 'client_secret_basic' }
  }

  // 2. ボディの client_id + client_secret
  if (body.client_id && body.client_secret) {
    const client = await loadClient(clientRepo, requestTenantId(c), body.client_id)
    verifySecret(client, body.client_secret, 'client_secret_post')
    return { client, method: 'client_secret_post' }
  }

  // 3. mTLS クライアント証明書認証 (RFC 8705 §2.1.2, tls_client_auth)。
  //    TLS 終端プロキシが検証済みの証明書を CLIENT_CERT_HEADER に載せる前提 (ADR-005)。
  //    本層は subject DN 一致と x5t#S256 計算のみを行う。
  const certHeader = c.req.header(CLIENT_CERT_HEADER)
  if (body.client_id && certHeader) {
    const client = await loadClient(clientRepo, requestTenantId(c), body.client_id)
    if (client.token_endpoint_auth_method === 'tls_client_auth') {
      const cert = parseClientCertificateHeader(certHeader)
      if (!cert) {
        throw new OAuthError('invalid_client', 'クライアント証明書をパースできません')
      }
      if (!client.tls_client_auth_subject_dn) {
        throw new OAuthError(
          'invalid_client',
          'クライアントに tls_client_auth_subject_dn が登録されていません',
        )
      }
      if (!clientCertSubjectMatches(client.tls_client_auth_subject_dn, cert.subjectDn)) {
        throw new OAuthError('invalid_client', '提示証明書の subject DN が一致しません')
      }
      return { client, method: 'tls_client_auth', mtlsThumbprintS256: cert.thumbprintS256 }
    }
  }

  // 4. 公開クライアント (none)
  if (body.client_id) {
    const client = await loadClient(clientRepo, requestTenantId(c), body.client_id)
    if (client.token_endpoint_auth_method !== 'none') {
      throw new OAuthError('invalid_client', 'クライアント認証が必要です')
    }
    return { client, method: 'none' }
  }

  throw new OAuthError('invalid_client', 'クライアント識別情報がありません')
}

/** aud として受理する値の集合 (issuer 識別子 または 各エンドポイント URL)。 */
export function buildAcceptableAudiences(issuer: string, requestUrl?: string): string[] {
  const base = issuer.replace(/\/$/, '')
  const set = new Set<string>([
    base,
    `${base}/token`,
    `${base}/par`,
    `${base}/introspect`,
    `${base}/revoke`,
  ])
  if (requestUrl) set.add(requestUrl)
  return [...set]
}

/**
 * private_key_jwt の client_assertion (RFC 7523) を検証する。
 *
 * `c` (HTTP Context) に依存しない純粋な検証関数として切り出し、単体テスト可能にする。
 * authenticateClient はここに aud / replayStore を渡すだけのラッパー。
 *
 * 検証項目:
 *   - alg ∈ {PS256, ES256} (ADR-003 と整合)
 *   - iss === sub === client_id (RFC 7523 §3)
 *   - クライアントが private_key_jwt を宣言している
 *   - 署名がクライアント登録鍵 (jwks / jwks_uri) で検証できる
 *   - aud がこのサーバーを指す
 *   - exp が存在し、寿命が MAX_ASSERTION_LIFETIME_SECONDS 以内
 *   - jti が存在し単回使用 (リプレイ検出)
 */
export async function verifyClientAssertion(
  assertion: string,
  clientRepo: ClientRepository,
  opts: { tenantId: string; audiences: string[]; replayStore: ClientAssertionReplayStore },
  now: Date = new Date(),
): Promise<Client> {
  // 署名前ヘッダ: alg 制約 (alg confusion 対策)
  let header: { alg?: string }
  try {
    header = decodeProtectedHeader(assertion)
  } catch {
    throw new OAuthError('invalid_client', 'client_assertion のヘッダが不正です')
  }
  if (header.alg !== 'PS256' && header.alg !== 'ES256') {
    throw new OAuthError('invalid_client', 'client_assertion の alg は PS256 / ES256 のみ許可')
  }

  // 署名前ペイロード: sub からクライアントを引く
  let claimedSub: unknown
  try {
    claimedSub = decodeJwt(assertion).sub
  } catch {
    throw new OAuthError('invalid_client', 'client_assertion がパースできません')
  }
  if (typeof claimedSub !== 'string' || claimedSub.length === 0) {
    throw new OAuthError('invalid_client', 'client_assertion に sub がありません')
  }

  const client = await loadClient(clientRepo, opts.tenantId, claimedSub)
  if (client.token_endpoint_auth_method !== 'private_key_jwt') {
    throw new OAuthError('invalid_client', 'クライアントは private_key_jwt を宣言していません')
  }

  const keyResolver = resolveClientKeys(client)

  let payload: Record<string, unknown>
  try {
    const verified = await jwtVerify(assertion, keyResolver, {
      algorithms: ['PS256', 'ES256'],
      issuer: client.client_id,
      subject: client.client_id,
      audience: opts.audiences,
      clockTolerance: CLOCK_SKEW_SECONDS,
      currentDate: now,
    })
    payload = verified.payload as Record<string, unknown>
  } catch (e) {
    if (e instanceof OAuthError) throw e
    throw new OAuthError('invalid_client', 'client_assertion の検証に失敗しました')
  }

  // exp 必須 + 寿命を有界に (リプレイ窓を確定させる)
  const exp = payload.exp
  if (typeof exp !== 'number') {
    throw new OAuthError('invalid_client', 'client_assertion に exp がありません')
  }
  const nowSec = Math.floor(now.getTime() / 1000)
  if (exp - nowSec > MAX_ASSERTION_LIFETIME_SECONDS + CLOCK_SKEW_SECONDS) {
    throw new OAuthError('invalid_client', 'client_assertion の寿命が長すぎます')
  }

  // jti 必須 + 単回使用
  const jti = payload.jti
  if (typeof jti !== 'string' || jti.length === 0) {
    throw new OAuthError('invalid_client', 'client_assertion に jti がありません')
  }
  const isNew = await opts.replayStore.recordIfNew(
    jti,
    MAX_ASSERTION_LIFETIME_SECONDS + CLOCK_SKEW_SECONDS,
    now,
  )
  if (!isNew) {
    throw new OAuthError('invalid_client', 'client_assertion の jti リプレイを検出しました')
  }

  return client
}

/** クライアント登録鍵 (インライン jwks 優先、なければ jwks_uri) から鍵リゾルバを作る。 */
function resolveClientKeys(client: Client): ReturnType<typeof createLocalJWKSet> {
  if (client.jwks) {
    const jwks = client.jwks as { keys?: unknown }
    if (!Array.isArray(jwks.keys) || jwks.keys.length === 0) {
      throw new OAuthError('invalid_client', 'クライアントの jwks が空です')
    }
    return createLocalJWKSet({ keys: jwks.keys as JWK[] })
  }
  if (client.jwks_uri) {
    // 本番: jwks_uri は登録時に SSRF 対策で許可ホストを検証済みとする (ADR-008 影響節)。
    return createRemoteJWKSet(new URL(client.jwks_uri))
  }
  throw new OAuthError('invalid_client', 'private_key_jwt 用の鍵が登録されていません')
}

async function loadClient(repo: ClientRepository, tenantId: string, id: string): Promise<Client> {
  const client = await repo.findById(tenantId, id)
  if (!client) {
    // タイミング差を出さないため、ハッシュ計算を空打ちする
    createHash('sha256').update('decoy').digest()
    throw new OAuthError('invalid_client', 'クライアント認証に失敗しました')
  }
  return client
}

function verifySecret(
  client: Client,
  presented: string,
  method: 'client_secret_basic' | 'client_secret_post',
): void {
  if (client.token_endpoint_auth_method !== method) {
    throw new OAuthError('invalid_client', '宣言された認証方式と一致しません')
  }
  if (!client.client_secret_hash) {
    throw new OAuthError('invalid_client', 'クライアントに秘密鍵が登録されていません')
  }
  const computed = createHash('sha256').update(presented).digest('hex')
  const a = Buffer.from(computed)
  const b = Buffer.from(client.client_secret_hash)
  if (a.length !== b.length || !timingSafeEqual(a, b)) {
    throw new OAuthError('invalid_client', 'クライアント認証に失敗しました')
  }
}

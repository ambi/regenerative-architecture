/**
 * Layer 4 — Adapter Layer（mTLS クライアント証明書ハンドリング）
 *
 * RFC 8705 §2 (tls_client_auth) + §3 (Certificate-Bound Access Tokens) に従い、
 * TLS 終端プロキシが渡す `X-Client-Certificate` ヘッダから
 *   - subject DN (登録 DN と照合)
 *   - DER の SHA-256 サムプリント (cnf.x5t#S256 として埋め込み)
 * を取り出す。
 *
 * 終端プロキシは検証済みの証明書のみを当ヘッダに載せる責務を持つ
 * (ADR-005)。本モジュールは PEM パースと thumbprint 計算のみを行い、
 * 信頼チェイン検証はしない。
 */

import { X509Certificate, createHash } from 'crypto'

export interface ParsedClientCertificate {
  /** RFC 4514 形式の subject DN。 */
  subjectDn: string
  /** base64url(SHA-256(DER))。cnf.x5t#S256 に使う。 */
  thumbprintS256: string
}

/**
 * `X-Client-Certificate` ヘッダの中身から証明書を取り出す。
 *
 * nginx `$ssl_client_escaped_cert` / envoy など、プロキシ実装ごとに
 * BEGIN/END 行の有無や URL エンコードの有無が異なるため両方を許容する。
 */
export function parseClientCertificateHeader(headerValue: string): ParsedClientCertificate | null {
  if (!headerValue) return null
  let pem: string
  try {
    pem = decodeURIComponent(headerValue.trim())
  } catch {
    return null
  }
  if (!pem.includes('-----BEGIN')) {
    const body = pem.replace(/\s+/g, '')
    if (body.length === 0) return null
    pem = `-----BEGIN CERTIFICATE-----\n${body}\n-----END CERTIFICATE-----\n`
  }
  let cert: X509Certificate
  try {
    cert = new X509Certificate(pem)
  } catch {
    return null
  }
  return {
    subjectDn: cert.subject,
    thumbprintS256: createHash('sha256').update(cert.raw).digest('base64url'),
  }
}

/**
 * 登録 DN と提示 DN の一致判定。
 *
 * RFC 4514 形式の string 表現は実装ごとに RDN 区切り (`,` / `\n`) や
 * 大文字小文字の扱いがゆれるため、正規化して比較する。
 * 厳密な ASN.1 レベルの DN 比較は本実装の射程外 (Phase 9 で証明書チェイン検証
 * を導入する際に差し替え予定)。
 */
export function clientCertSubjectMatches(expected: string, presented: string): boolean {
  return normalizeDn(expected) === normalizeDn(presented)
}

function normalizeDn(dn: string): string {
  return dn
    .split(/[\n,]+/)
    .map((rdn) => rdn.trim().toLowerCase())
    .filter((rdn) => rdn.length > 0)
    .join(',')
}

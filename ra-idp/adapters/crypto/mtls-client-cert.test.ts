/**
 * Layer 4 — Adapter test: mTLS クライアント証明書ハンドリング。
 *
 * テスト証明書は P-256 self-signed (CN=mtls-app, O=Example) を固定値で同梱する。
 * 別途 openssl などのランタイム依存を持ち込まず、決定論的に thumbprint を検証できる。
 */

import { describe, expect, it } from 'bun:test'
import { X509Certificate, createHash } from 'crypto'

import {
  clientCertSubjectMatches,
  parseClientCertificateHeader,
} from './mtls-client-cert'
import { TEST_CERT_PEM } from './mtls-test-fixtures'

describe('parseClientCertificateHeader', () => {
  it('生 PEM を受け取り subject と thumbprint を返す', () => {
    const parsed = parseClientCertificateHeader(TEST_CERT_PEM)
    expect(parsed).not.toBeNull()
    expect(parsed!.subjectDn.toLowerCase()).toContain('cn=mtls-app')
    const der = new X509Certificate(TEST_CERT_PEM).raw
    const expected = createHash('sha256').update(der).digest('base64url')
    expect(parsed!.thumbprintS256).toBe(expected)
  })

  it('URL エンコードされた PEM もパースできる', () => {
    const parsed = parseClientCertificateHeader(encodeURIComponent(TEST_CERT_PEM))
    expect(parsed).not.toBeNull()
    expect(parsed!.subjectDn.toLowerCase()).toContain('cn=mtls-app')
  })

  it('BEGIN/END 行が剥がされた base64 ボディも復元してパースする', () => {
    const body = TEST_CERT_PEM
      .replace(/-----BEGIN CERTIFICATE-----/, '')
      .replace(/-----END CERTIFICATE-----/, '')
      .replace(/\s+/g, '')
    const parsed = parseClientCertificateHeader(body)
    expect(parsed).not.toBeNull()
  })

  it('不正な入力は null を返す (例外を投げない)', () => {
    expect(parseClientCertificateHeader('not a cert')).toBeNull()
    expect(parseClientCertificateHeader('')).toBeNull()
  })
})

describe('clientCertSubjectMatches', () => {
  it('完全一致は true', () => {
    expect(clientCertSubjectMatches('CN=mtls-app,O=Example', 'CN=mtls-app,O=Example')).toBe(true)
  })

  it('大文字小文字差は無視', () => {
    expect(clientCertSubjectMatches('CN=mtls-app', 'cn=MTLS-APP')).toBe(true)
  })

  it('whitespace 差は無視', () => {
    expect(clientCertSubjectMatches('CN=mtls-app, O=Example', 'CN=mtls-app,  O=Example')).toBe(true)
  })

  it('改行で RDN を区切るプロキシ表現も受理', () => {
    expect(clientCertSubjectMatches('CN=mtls-app,O=Example', 'CN=mtls-app\nO=Example')).toBe(true)
  })

  it('別 DN は false', () => {
    expect(clientCertSubjectMatches('CN=mtls-app', 'CN=attacker')).toBe(false)
  })
})

/**
 * Layer 3 — Application Logic（ポート定義）
 *
 * 署名鍵ストア。本番では KMS / HSM、本アプリではメモリ。
 * 公開鍵は JWKS として配布される。
 *
 * 鍵オブジェクト本体（`privateKey` / `publicKey`）の表現は実行環境依存
 * （Web Crypto の `CryptoKey`、Node の `KeyObject`、HSM ハンドル等）。
 * ポート層では「不透明な鍵参照」として `unknown` 扱いとし、
 * 署名アダプター層が自分の知っている形に narrowing する。
 */

export interface SigningKey {
  kid: string
  alg: 'PS256' | 'ES256'
  privateKey: unknown
  publicKey: unknown
  publicJwk: Record<string, unknown>
  active: boolean
  created_at: string
}

export interface KeyStore {
  /** 現在アクティブな署名鍵（1つ）。 */
  getActiveKey(): Promise<SigningKey>
  /** 旧鍵も含めた全鍵（JWKS で公開する用途）。 */
  getAllKeys(): Promise<SigningKey[]>
  /** 指定 kid の鍵（検証用途）。 */
  findByKid(kid: string): Promise<SigningKey | null>
  /** 新しい鍵に回転する。 */
  rotate(): Promise<SigningKey>
}

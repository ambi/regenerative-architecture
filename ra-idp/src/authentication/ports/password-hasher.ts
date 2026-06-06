/**
 * Layer 3 — Application Logic（ポート定義）
 *
 * Password storage の境界。実装は Argon2id 等の OWASP 推奨アルゴリズムを用い、
 * 戻り値の encoded string にアルゴリズム・パラメータ・salt を内包する PHC 形式
 * を期待する。
 */

export interface PasswordHasher {
  hash(password: string): Promise<string>
  verify(password: string, encoded: string): Promise<boolean>
}

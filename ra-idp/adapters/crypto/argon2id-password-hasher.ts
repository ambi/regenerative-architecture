/**
 * Layer 4 — Adapter Layer (crypto: Argon2id password hasher)
 *
 * Bun 組み込みの Bun.password を使用し、OWASP 2024 推奨の Argon2id パラメータ
 * (m=19456 KiB, t=2, p=1) を用いてパスワードを PHC 形式でエンコードする。
 * verify は PHC 文字列に埋め込まれたパラメータでデコードするため、将来
 * パラメータを更新しても旧 hash と互換性を保つ。
 */

import type { PasswordHasher } from '../../src/authentication/ports/password-hasher'

const OWASP_MEMORY_COST_KIB = 19456
const OWASP_TIME_COST = 2

export class Argon2idPasswordHasher implements PasswordHasher {
  constructor(
    private readonly memoryCost: number = OWASP_MEMORY_COST_KIB,
    private readonly timeCost: number = OWASP_TIME_COST,
  ) {}

  hash(password: string): Promise<string> {
    return Bun.password.hash(password, {
      algorithm: 'argon2id',
      memoryCost: this.memoryCost,
      timeCost: this.timeCost,
    })
  }

  verify(password: string, encoded: string): Promise<boolean> {
    return Bun.password.verify(password, encoded)
  }
}

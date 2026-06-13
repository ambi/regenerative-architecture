/**
 * Layer 4 — Adapter Layer (Noop BreachedPasswordChecker)
 *
 * memory モード / in-memory テスト / 外部依存を持ちたくない起動用の既定実装。
 * 常に false を返す。詳細は ADR-028。
 */

import type { BreachedPasswordChecker } from '../../src/authentication/ports/breached-password-checker'

export class NoopBreachedPasswordChecker implements BreachedPasswordChecker {
  async isBreached(_plain: string): Promise<boolean> {
    return false
  }
}

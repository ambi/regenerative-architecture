/**
 * Layer 4 — Adapter Layer (in-memory TransactionRunner)
 *
 * Map ベースの操作はそれ自体が JavaScript の単一スレッド内で atomic とみなせるため、
 * トランザクションは「fn をそのまま実行する」だけでよい。
 *
 * これは正確には「複数 Map にまたがる書き込みの可視性を atomic に保つ」保証はないが、
 * テスト用途では十分。本番用途では postgres TransactionRunner を使う。
 */

import type { TransactionContext, TransactionRunner } from '../../../src/shared/ports/transaction'

export class InMemoryTransactionRunner implements TransactionRunner {
  async runInTransaction<T>(fn: (tx: TransactionContext) => Promise<T>): Promise<T> {
    return fn(null)
  }
}

/**
 * Layer 4 — Adapter Layer (Postgres TransactionRunner)
 *
 * BEGIN / COMMIT / ROLLBACK を回し、fn には PoolClient を tx として渡す。
 * ユースケース層がトランザクションに参加する store/event-sink に同じ tx を
 * 透過させることで、認可コード redeem + refresh 発行 + outbox INSERT を
 * 不可分にする (ADR-016, RFC 9700 §4.10)。
 */

import type { TransactionContext, TransactionRunner } from '../../../src/shared/ports/transaction'
import type { PgPool } from './pool'

export class PostgresTransactionRunner implements TransactionRunner {
  constructor(private readonly pool: PgPool) {}

  async runInTransaction<T>(fn: (tx: TransactionContext) => Promise<T>): Promise<T> {
    const client = await this.pool.connect()
    try {
      await client.query('BEGIN')
      try {
        const result = await fn(client)
        await client.query('COMMIT')
        return result
      } catch (err) {
        await client.query('ROLLBACK')
        throw err
      }
    } finally {
      client.release()
    }
  }
}

/**
 * tx context が postgres PoolClient なら返す。
 * tx === null ならプール (Pool) を返す。これにより各 store は
 * 「トランザクション内なら tx を、無ければ pool を」使う実装に統一できる。
 */
export function pgQuerier(pool: PgPool, tx: TransactionContext): any {
  return tx ?? pool
}

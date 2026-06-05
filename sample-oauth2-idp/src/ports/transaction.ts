/**
 * Layer 3 — Application Logic (ポート定義)
 *
 * トランザクション境界の抽象。
 *
 * ADR-016 が「認可コード redeem + refresh token 発行 + outbox INSERT」を
 * 1 つの不可分操作にすることを要求するため、ユースケースから
 * 「これらをまとめて実行」を表現できる必要がある。
 *
 * 実装側 (adapters/persistence/*):
 *  - InMemoryTransactionRunner   no-op (Map 操作はそれ自体が atomic とみなす)
 *  - PostgresTransactionRunner   BEGIN / COMMIT / ROLLBACK
 *
 * トランザクションコンテキスト (TransactionContext) は不透明 (unknown) として扱い、
 * 実装側が自分の知っている型 (PoolClient 等) に narrowing する。
 * これは KeyStore で `privateKey: unknown` としたのと同じ方針。
 */

/**
 * トランザクションコンテキスト。実装ごとに具体型は異なる。
 * 例: Postgres なら `PoolClient`、SQLite なら `Database`、in-memory なら null。
 */
export type TransactionContext = unknown

export interface TransactionRunner {
  /**
   * 関数 fn をトランザクション内で実行する。
   * fn が throw した場合は ROLLBACK、正常終了した場合は COMMIT。
   *
   * @example
   * await txRunner.runInTransaction(async (tx) => {
   *   await codeStore.redeem(code, now, tx)
   *   await refreshStore.save(record, tx)
   *   await eventSink.publish(event, tx)
   * })
   */
  runInTransaction<T>(fn: (tx: TransactionContext) => Promise<T>): Promise<T>
}

/**
 * Layer 5 — Runtime: SIGTERM / SIGINT で observer / pool / redis を flush する。
 *
 * postgres モードのときだけ pool / redis をクローズする (memory モードでは何もしない)。
 */

import type { Observer } from '../src/shared/ports/observer'
import type { RuntimeConfig } from './config'

export function registerShutdownHandlers(config: RuntimeConfig, observer: Observer): void {
  const shutdown = async (signal: string): Promise<void> => {
    // eslint-disable-next-line no-console
    console.log(`[main] received ${signal}, shutting down...`)
    try {
      await observer.shutdown()
    } catch {
      /* noop */
    }
    if (config.persistenceMode === 'postgres') {
      try {
        const { closePool } = await import('../adapters/persistence/postgres/pool')
        await closePool()
      } catch {
        /* noop */
      }
      try {
        const { closeRedis } = await import('../adapters/persistence/redis/client')
        await closeRedis()
      } catch {
        /* noop */
      }
    }
    process.exit(0)
  }
  process.on('SIGTERM', () => shutdown('SIGTERM'))
  process.on('SIGINT', () => shutdown('SIGINT'))
}

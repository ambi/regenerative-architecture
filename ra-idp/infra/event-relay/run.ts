/**
 * Layer 5 — Runtime (event-relay): pool 取得 → KafkaOutboxRelay 起動 → shutdown 登録。
 *
 * Kubernetes Deployment で 1 個以上のレプリカ起動。FOR UPDATE SKIP LOCKED により
 * 複数レプリカで安全に並走。Graceful shutdown (SIGTERM / SIGINT) で in-flight ループ
 * 完了を待つ。
 */

import { KafkaOutboxRelay } from '../../adapters/event-sink/kafka-relay'
import { closePool, getPool } from '../../adapters/persistence/postgres/pool'

import { loadRelayConfig } from './config'

export async function runRelay(): Promise<void> {
  const config = loadRelayConfig()
  const pool = await getPool({ connectionString: config.databaseUrl })
  const relay = new KafkaOutboxRelay(pool, {
    brokers: config.kafkaBrokers,
    pollIntervalMs: config.pollIntervalMs,
    batchSize: config.batchSize,
    clientId: config.clientId,
  })

  // eslint-disable-next-line no-console
  console.log(`[event-relay] starting; brokers=${config.kafkaBrokers.join(',')}`)
  await relay.start()

  const shutdown = async (signal: string): Promise<void> => {
    // eslint-disable-next-line no-console
    console.log(`[event-relay] received ${signal}, shutting down...`)
    await relay.stop()
    await closePool()
    process.exit(0)
  }
  process.on('SIGTERM', () => shutdown('SIGTERM'))
  process.on('SIGINT', () => shutdown('SIGINT'))
}

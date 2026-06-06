/**
 * Layer 5 — Runtime (event-relay process)
 *
 * Postgres outbox → Kafka 配送の常駐プロセス。
 *
 * 起動:
 *   DATABASE_URL=... KAFKA_BROKERS=host1:9092,host2:9092 bun run infra/event-relay/main.ts
 *
 * Kubernetes では Deployment として 1 個以上のレプリカを起動する。
 * FOR UPDATE SKIP LOCKED により複数レプリカで安全に並走できる。
 *
 * Graceful shutdown (SIGTERM / SIGINT) で in-flight ループ完了を待ってから終了する。
 */

import { getPool, closePool } from '../../adapters/persistence/postgres/pool'
import { KafkaOutboxRelay } from '../../adapters/event-sink/kafka-relay'

async function main() {
  const databaseUrl = process.env.DATABASE_URL
  const kafkaBrokers = (process.env.KAFKA_BROKERS ?? '').split(',').filter(Boolean)
  if (!databaseUrl) throw new Error('DATABASE_URL required')
  if (kafkaBrokers.length === 0) throw new Error('KAFKA_BROKERS required (comma-separated)')

  const pool = await getPool({ connectionString: databaseUrl })
  const relay = new KafkaOutboxRelay(pool, {
    brokers: kafkaBrokers,
    pollIntervalMs: Number(process.env.POLL_INTERVAL_MS ?? 200),
    batchSize: Number(process.env.BATCH_SIZE ?? 100),
    clientId: process.env.RELAY_CLIENT_ID ?? 'ra-idp-event-relay',
  })

  // eslint-disable-next-line no-console
  console.log(`[event-relay] starting; brokers=${kafkaBrokers.join(',')}`)
  await relay.start()

  const shutdown = async (signal: string) => {
    // eslint-disable-next-line no-console
    console.log(`[event-relay] received ${signal}, shutting down...`)
    await relay.stop()
    await closePool()
    process.exit(0)
  }
  process.on('SIGTERM', () => shutdown('SIGTERM'))
  process.on('SIGINT', () => shutdown('SIGINT'))
}

main().catch((e) => {
  // eslint-disable-next-line no-console
  console.error('[event-relay] fatal:', e)
  process.exit(1)
})

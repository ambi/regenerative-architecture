/**
 * Layer 5 — Runtime (event-relay): 環境変数から起動構成を読み出す。
 */

export interface RelayConfig {
  databaseUrl: string
  kafkaBrokers: string[]
  pollIntervalMs: number
  batchSize: number
  clientId: string
}

export function loadRelayConfig(): RelayConfig {
  const databaseUrl = process.env.DATABASE_URL
  const kafkaBrokers = (process.env.KAFKA_BROKERS ?? '').split(',').filter(Boolean)
  if (!databaseUrl) throw new Error('DATABASE_URL required')
  if (kafkaBrokers.length === 0) throw new Error('KAFKA_BROKERS required (comma-separated)')
  return {
    databaseUrl,
    kafkaBrokers,
    pollIntervalMs: Number(process.env.POLL_INTERVAL_MS ?? 200),
    batchSize: Number(process.env.BATCH_SIZE ?? 100),
    clientId: process.env.RELAY_CLIENT_ID ?? 'ra-idp-event-relay',
  }
}

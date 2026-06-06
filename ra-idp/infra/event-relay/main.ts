/**
 * Layer 5 — Runtime (event-relay): エントリポイント。
 *
 * 起動シーケンスは run.ts に集約。
 *
 * 起動:
 *   DATABASE_URL=... KAFKA_BROKERS=host1:9092,host2:9092 bun run infra/event-relay/main.ts
 */

import { runRelay } from './run'

runRelay().catch((e) => {
  // eslint-disable-next-line no-console
  console.error('[event-relay] fatal:', e)
  process.exit(1)
})

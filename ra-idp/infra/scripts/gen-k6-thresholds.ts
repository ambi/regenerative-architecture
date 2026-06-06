/**
 * Generator: spec/scl.yaml (objectives)
 *  → infra/load-tests/k6/thresholds.json
 *
 * k6 の thresholds オプションを SLO から生成する。
 * シナリオファイル (token-exchange.js 等) は本 JSON を import して
 * `options.thresholds` に当てる。
 *
 * これにより、SLO の値を変えるだけで負荷試験の合否判定基準が同期する。
 */

import { writeFile, mkdir } from 'fs/promises'
import { join, dirname } from 'path'
import { loadSlo } from './load-specs'

async function main() {
  const slo = loadSlo()
  const thresholds: Record<string, string[]> = {}

  for (const [endpoint, entry] of Object.entries(slo.performance.endpoints)) {
    // k6 のメトリクス命名規約: tag によりラベル付け
    thresholds[`http_req_duration{endpoint:${endpoint}}`] = [`p(99)<${entry.p99_latency_ms}`]
    thresholds[`http_req_failed{endpoint:${endpoint}}`] = [`rate<${entry.error_rate_max}`]
  }

  const out = {
    _generated_from: 'spec/scl.yaml',
    thresholds,
    scalability: slo.scalability,
  }

  const outPath = join(import.meta.dir, '../load-tests/k6/thresholds.json')
  await mkdir(dirname(outPath), { recursive: true })
  await writeFile(outPath, JSON.stringify(out, null, 2) + '\n')
  // eslint-disable-next-line no-console
  console.log(`Wrote k6 thresholds → ${outPath}`)
}

main().catch((e) => {
  console.error(e)
  process.exit(1)
})

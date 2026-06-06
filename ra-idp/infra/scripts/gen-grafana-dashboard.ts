/**
 * Generator: spec/scl.yaml (objectives + interface annotations)
 *  → infra/observability/grafana/dashboards/ra-idp.json
 *
 * SLO threshold ライン付きの最小ダッシュボードを生成する。
 * 本格的なダッシュボードは現場で grafana エディタを使って組むが、
 * **権威は spec 側にあり、ダッシュボードはそれを描画するだけ**という構造を実証する。
 */

import { writeFile, mkdir } from 'fs/promises'
import { join, dirname } from 'path'
import { loadSlo, loadObservability } from './load-specs'

async function main() {
  const slo = loadSlo()
  const obs = loadObservability()

  const panels: any[] = []
  let id = 1
  let y = 0

  for (const [endpoint, entry] of Object.entries(slo.performance.endpoints)) {
    panels.push({
      id: id++,
      title: `${endpoint} — p99 latency`,
      type: 'timeseries',
      gridPos: { h: 8, w: 12, x: 0, y },
      datasource: { type: 'prometheus', uid: 'prom' },
      targets: [
        {
          expr: `oauth2_${endpoint}_request_duration_seconds:p99`,
          legendFormat: 'p99',
        },
      ],
      fieldConfig: {
        defaults: {
          unit: 's',
          thresholds: {
            mode: 'absolute',
            steps: [
              { color: 'green', value: 0 },
              { color: 'red', value: entry.p99_latency_ms / 1000 },
            ],
          },
        },
      },
    })

    panels.push({
      id: id++,
      title: `${endpoint} — error rate`,
      type: 'timeseries',
      gridPos: { h: 8, w: 12, x: 12, y },
      datasource: { type: 'prometheus', uid: 'prom' },
      targets: [
        {
          expr: `rate(oauth2_${endpoint}_requests_total{result="error"}[5m]) / rate(oauth2_${endpoint}_requests_total[5m])`,
          legendFormat: 'error_rate',
        },
      ],
      fieldConfig: {
        defaults: {
          unit: 'percentunit',
          thresholds: {
            mode: 'absolute',
            steps: [
              { color: 'green', value: 0 },
              { color: 'red', value: entry.error_rate_max },
            ],
          },
        },
      },
    })

    y += 8
  }

  // Security panels
  panels.push({
    id: id++,
    title: 'Refresh token reuse detected (CRITICAL)',
    type: 'stat',
    gridPos: { h: 6, w: 12, x: 0, y },
    datasource: { type: 'prometheus', uid: 'prom' },
    targets: [{ expr: 'increase(oauth2_refresh_token_reuse_detected_total[1m])' }],
    fieldConfig: {
      defaults: {
        thresholds: {
          mode: 'absolute',
          steps: [
            { color: 'green', value: 0 },
            { color: 'red', value: 1 },
          ],
        },
      },
    },
  })
  panels.push({
    id: id++,
    title: 'Client auth failures / minute',
    type: 'stat',
    gridPos: { h: 6, w: 12, x: 12, y },
    datasource: { type: 'prometheus', uid: 'prom' },
    targets: [{ expr: 'rate(oauth2_client_auth_failures_total[1m]) * 60' }],
    fieldConfig: {
      defaults: {
        thresholds: {
          mode: 'absolute',
          steps: [
            { color: 'green', value: 0 },
            { color: 'red', value: slo.security.client_auth_failure_rate_limit_per_minute },
          ],
        },
      },
    },
  })

  const dashboard = {
    _generated_from: 'spec/scl.yaml',
    title: `${obs.service.name} — SLO & Security`,
    uid: 'ra-idp',
    schemaVersion: 39,
    version: 1,
    refresh: '30s',
    panels,
    tags: ['oauth2', 'idp', 'slo'],
  }

  const outPath = join(import.meta.dir, '../observability/grafana/dashboards/ra-idp.json')
  await mkdir(dirname(outPath), { recursive: true })
  await writeFile(outPath, JSON.stringify(dashboard, null, 2) + '\n')
  // eslint-disable-next-line no-console
  console.log(`Wrote Grafana dashboard → ${outPath}`)
}

main().catch((e) => {
  console.error(e)
  process.exit(1)
})

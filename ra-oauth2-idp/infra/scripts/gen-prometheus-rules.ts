/**
 * Generator: spec/scl.yaml (objectives + interface annotations)
 *  → infra/observability/prometheus/recording-rules.yaml
 *  → infra/observability/prometheus/alerts.yaml
 *
 * RA §3.1「非機能要件の仕様化」と §7「ドリフト検知」を実装。
 * CI で `bun run gen:prometheus && git diff --exit-code` を回し、
 * 仕様核と派生物の drift を検出する。
 *
 * 実行: bun run infra/scripts/gen-prometheus-rules.ts
 */

import { writeFile } from 'fs/promises'
import { mkdir } from 'fs/promises'
import { join, dirname } from 'path'
import { loadSlo, loadObservability, expandTemplate } from './load-specs'

const HEADER = `# THIS FILE IS GENERATED FROM:
#   spec/scl.yaml
# Do not edit by hand. Run: bun run gen:prometheus
`

interface RecordingRule {
  record: string
  expr: string
  labels?: Record<string, string>
}

interface AlertRule {
  alert: string
  expr: string
  for?: string
  labels?: Record<string, string>
  annotations?: Record<string, string>
}

async function main() {
  const slo = loadSlo()
  const obs = loadObservability()

  // -----------------------------------------------------------------
  // Recording rules: histogram から p99 を抽出する省力化
  // -----------------------------------------------------------------
  const recordingRules: RecordingRule[] = []
  for (const [metricName, metric] of Object.entries(obs.metrics)) {
    if (metric.type !== 'histogram') continue
    recordingRules.push({
      record: `${metricName}:p99`,
      expr: `histogram_quantile(0.99, sum by (le) (rate(${metricName}_bucket[5m])))`,
    })
    recordingRules.push({
      record: `${metricName}:p95`,
      expr: `histogram_quantile(0.95, sum by (le) (rate(${metricName}_bucket[5m])))`,
    })
  }

  // -----------------------------------------------------------------
  // Alert rules
  // -----------------------------------------------------------------
  const alertRules: AlertRule[] = []
  for (const alert of obs.alerts ?? []) {
    const expr = alert.expression_template
      ? expandTemplate(alert.expression_template, slo)
      : (alert.expression ?? '')
    alertRules.push({
      alert: alert.name,
      expr: expr.trim(),
      for: alert.for ?? '5m',
      labels: { severity: alert.severity },
      annotations: alert.runbook ? { runbook: alert.runbook } : undefined,
    })
  }

  // 各エンドポイントの p99 breach アラートを SLO から自動生成
  for (const [endpoint, slo_entry] of Object.entries(slo.performance.endpoints)) {
    alertRules.push({
      alert: `oauth2_${endpoint}_p99_breach`,
      expr: `oauth2_${endpoint}_request_duration_seconds:p99 > ${slo_entry.p99_latency_ms / 1000}`,
      for: '5m',
      labels: { severity: 'warning', endpoint },
      annotations: {
        summary: `${endpoint} endpoint p99 latency above SLO`,
        description: `Current p99 > ${slo_entry.p99_latency_ms}ms (SLO: spec/scl.yaml objectives.${endpoint} p99_latency_ms)`,
      },
    })
    alertRules.push({
      alert: `oauth2_${endpoint}_error_rate_breach`,
      expr: `rate(oauth2_${endpoint}_requests_total{result="error"}[5m]) / rate(oauth2_${endpoint}_requests_total[5m]) > ${slo_entry.error_rate_max}`,
      for: '5m',
      labels: { severity: 'warning', endpoint },
      annotations: {
        summary: `${endpoint} endpoint error rate above SLO`,
        description: `error_rate > ${slo_entry.error_rate_max}`,
      },
    })
  }

  // -----------------------------------------------------------------
  // Write outputs
  // -----------------------------------------------------------------
  const outRecording = renderRules('ra-oauth2-idp.recording', recordingRules)
  const outAlerts = renderAlerts('ra-oauth2-idp.alerts', alertRules)

  const recPath = join(import.meta.dir, '../observability/prometheus/recording-rules.yaml')
  const altPath = join(import.meta.dir, '../observability/prometheus/alerts.yaml')
  await mkdir(dirname(recPath), { recursive: true })
  await writeFile(recPath, HEADER + outRecording)
  await writeFile(altPath, HEADER + outAlerts)

  // eslint-disable-next-line no-console
  console.log(`Wrote ${recordingRules.length} recording rules → ${recPath}`)
  // eslint-disable-next-line no-console
  console.log(`Wrote ${alertRules.length} alert rules → ${altPath}`)
}

function renderRules(groupName: string, rules: RecordingRule[]): string {
  const lines: string[] = []
  lines.push(`groups:`)
  lines.push(`  - name: ${groupName}`)
  lines.push(`    interval: 30s`)
  lines.push(`    rules:`)
  for (const r of rules) {
    lines.push(`      - record: ${r.record}`)
    lines.push(`        expr: ${quote(r.expr)}`)
  }
  return lines.join('\n') + '\n'
}

function renderAlerts(groupName: string, rules: AlertRule[]): string {
  const lines: string[] = []
  lines.push(`groups:`)
  lines.push(`  - name: ${groupName}`)
  lines.push(`    rules:`)
  for (const r of rules) {
    lines.push(`      - alert: ${r.alert}`)
    lines.push(`        expr: ${quote(r.expr)}`)
    if (r.for) lines.push(`        for: ${r.for}`)
    if (r.labels && Object.keys(r.labels).length > 0) {
      lines.push(`        labels:`)
      for (const [k, v] of Object.entries(r.labels)) {
        lines.push(`          ${k}: ${quote(v)}`)
      }
    }
    if (r.annotations && Object.keys(r.annotations).length > 0) {
      lines.push(`        annotations:`)
      for (const [k, v] of Object.entries(r.annotations)) {
        lines.push(`          ${k}: ${quote(v)}`)
      }
    }
  }
  return lines.join('\n') + '\n'
}

function quote(s: string): string {
  if (/^[0-9.]+$/.test(s)) return s
  return `'${s.replace(/'/g, "''")}'`
}

main().catch((e) => {
  console.error(e)
  process.exit(1)
})

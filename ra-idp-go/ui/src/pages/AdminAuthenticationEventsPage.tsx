import { IconDownload, IconSearch } from '@tabler/icons-react'
import { type FormEvent, useState } from 'react'
import {
  type AdminAuthenticationEventQuery,
  AuthenticationAPIError,
  listAdminAuthenticationEvents,
  tenantURL,
} from '../api'
import { AdminShell } from '../components/AdminShell'
import { Alert } from '../components/ui/alert'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import type {
  AdminAuditEvent,
  AdminAuthenticationEventsPage as AdminAuthenticationEventsPageData,
} from '../types'

const DEFAULT_TENANT_ID = 'default'

const FAIL_TYPES = new Set(['AuthenticationFailed', 'AuthenticationStepFailed', 'MfaChallengeFailed'])
const AGGREGATED_TYPES = new Set(['AuthenticationEventAggregated', 'LoginThrottled'])

type EventKind = 'success' | 'fail' | 'aggregated'

function eventKind(type: string): EventKind {
  if (FAIL_TYPES.has(type)) return 'fail'
  if (AGGREGATED_TYPES.has(type)) return 'aggregated'
  return 'success'
}

const KIND_BADGE: Record<EventKind, string> = {
  success: 'bg-emerald-50 text-emerald-700',
  fail: 'bg-rose-50 text-rose-700',
  aggregated: 'bg-amber-50 text-amber-700',
}

// datetime-local input は秒なしのローカル時刻。ISO の先頭 16 文字でフォームを埋める。
function toLocalInput(iso: string): string {
  try {
    const d = new Date(iso)
    const offset = d.getTimezoneOffset() * 60000
    return new Date(d.getTime() - offset).toISOString().slice(0, 16)
  } catch {
    return ''
  }
}

export function AdminAuthenticationEventsPage({
  actorUsername,
  actorRoles,
  actorTenantID,
  from: initialFrom,
  to: initialTo,
  events: initial,
}: AdminAuthenticationEventsPageData) {
  const [events, setEvents] = useState(initial)
  const [selected, setSelected] = useState<AdminAuditEvent | null>(initial[0] ?? null)
  const [kind, setKind] = useState<'' | EventKind>('')
  const [sub, setSub] = useState('')
  const [usernameHash, setUsernameHash] = useState('')
  const [ipTruncated, setIpTruncated] = useState('')
  const [from, setFrom] = useState(toLocalInput(initialFrom))
  const [to, setTo] = useState(toLocalInput(initialTo))
  const [allTenants, setAllTenants] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const canCrossTenant =
    actorRoles.includes('system_admin') && actorTenantID === DEFAULT_TENANT_ID

  function buildQuery(): AdminAuthenticationEventQuery | null {
    if (!from.trim() || !to.trim()) {
      setError('期間 (From / To) は必須です。')
      return null
    }
    return {
      from: new Date(from).toISOString(),
      to: new Date(to).toISOString(),
      kind: kind || undefined,
      sub: sub.trim() || undefined,
      usernameHash: usernameHash.trim() || undefined,
      ipTruncated: ipTruncated.trim() || undefined,
      limit: 200,
      allTenants: canCrossTenant && allTenants,
    }
  }

  async function handleQuery(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    const q = buildQuery()
    if (!q) return
    setBusy(true)
    try {
      const next = await listAdminAuthenticationEvents(q)
      setEvents(next)
      setSelected(next[0] ?? null)
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : '認証イベントを取得できませんでした。',
      )
    } finally {
      setBusy(false)
    }
  }

  function handleExport() {
    setError('')
    const q = buildQuery()
    if (!q) return
    const params = new URLSearchParams()
    params.set('from', q.from)
    params.set('to', q.to)
    if (q.kind) params.set('kind', q.kind)
    if (q.sub) params.set('sub', q.sub)
    if (q.usernameHash) params.set('username_hash', q.usernameHash)
    if (q.ipTruncated) params.set('ip_truncated', q.ipTruncated)
    if (q.allTenants) params.set('all_tenants', 'true')
    window.open(tenantURL(`/api/admin/authentication_events/export?${params.toString()}`), '_blank')
  }

  return (
    <AdminShell
      active="authentication-events"
      actorUsername={actorUsername}
      title="認証イベント"
      description="ログイン成功・失敗・MFA・セッションの時系列。攻撃時の失敗は bucket に集約され、調査の起点になります。"
    >
      {error ? <Alert variant="destructive">{error}</Alert> : null}

      <Card className="p-5">
        <form onSubmit={handleQuery} className="grid gap-4 lg:grid-cols-3">
          <Field label="From (必須)">
            <Input type="datetime-local" value={from} onChange={(e) => setFrom(e.target.value)} required />
          </Field>
          <Field label="To (必須)">
            <Input type="datetime-local" value={to} onChange={(e) => setTo(e.target.value)} required />
          </Field>
          <Field label="種別">
            <select
              value={kind}
              onChange={(e) => setKind(e.target.value as '' | EventKind)}
              className="h-9 rounded-md border border-slate-300 bg-white px-3 text-sm"
            >
              <option value="">すべて</option>
              <option value="success">成功</option>
              <option value="fail">失敗</option>
              <option value="aggregated">集約</option>
            </select>
          </Field>
          <Field label="Sub">
            <Input value={sub} onChange={(e) => setSub(e.target.value)} placeholder="user_..." />
          </Field>
          <Field label="Username hash">
            <Input
              value={usernameHash}
              onChange={(e) => setUsernameHash(e.target.value)}
              placeholder="sha256 hex"
            />
          </Field>
          <Field label="IP (truncated)">
            <Input
              value={ipTruncated}
              onChange={(e) => setIpTruncated(e.target.value)}
              placeholder="203.0.113.0"
            />
          </Field>
          <div className="flex items-end gap-2 lg:col-span-3">
            <Button type="submit" disabled={busy}>
              <IconSearch size={16} aria-hidden="true" />
              絞り込み
            </Button>
            <Button type="button" variant="ghost" onClick={handleExport} disabled={busy}>
              <IconDownload size={16} aria-hidden="true" />
              エクスポート
            </Button>
          </div>
        </form>
        {canCrossTenant ? (
          <label className="mt-4 inline-flex items-center gap-2 text-sm text-slate-700">
            <input
              type="checkbox"
              checked={allTenants}
              onChange={(e) => setAllTenants(e.target.checked)}
              className="size-4 rounded border-slate-300"
            />
            全テナント横断 (system_admin)
          </label>
        ) : null}
      </Card>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_420px]">
        <Card className="overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
              <tr>
                <th className="px-4 py-3">発生日時</th>
                <th className="px-4 py-3">種別</th>
                <th className="px-4 py-3">イベント</th>
              </tr>
            </thead>
            <tbody>
              {events.length === 0 ? (
                <tr>
                  <td colSpan={3} className="px-4 py-12 text-center text-sm text-slate-500">
                    一致する認証イベントはありません。
                  </td>
                </tr>
              ) : null}
              {events.map((e) => (
                <tr
                  key={e.id}
                  onClick={() => setSelected(e)}
                  className={`cursor-pointer border-t border-slate-100 hover:bg-slate-50 ${
                    selected?.id === e.id ? 'bg-blue-50/60' : ''
                  }`}
                >
                  <td className="px-4 py-3 font-mono text-xs">{formatDate(e.occurred_at)}</td>
                  <td className="px-4 py-3">
                    <span
                      className={`rounded px-2 py-0.5 text-xs font-medium ${KIND_BADGE[eventKind(e.type)]}`}
                    >
                      {eventKind(e.type)}
                    </span>
                  </td>
                  <td className="px-4 py-3">{e.type}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>

        <Card className="p-5">
          <h2 className="text-sm font-semibold text-slate-700">詳細</h2>
          {selected ? (
            <>
              <dl className="mt-4 grid grid-cols-[110px_minmax(0,1fr)] gap-y-2 text-xs">
                <dt className="text-slate-500">イベント</dt>
                <dd>{selected.type}</dd>
                <dt className="text-slate-500">日時</dt>
                <dd>{formatDate(selected.occurred_at)}</dd>
                <dt className="text-slate-500">テナント</dt>
                <dd className="font-mono">{selected.tenant_id}</dd>
                {payloadString(selected, 'sub') ? (
                  <>
                    <dt className="text-slate-500">Sub</dt>
                    <dd className="break-all font-mono">{payloadString(selected, 'sub')}</dd>
                  </>
                ) : null}
                {payloadString(selected, 'ipTruncated') ? (
                  <>
                    <dt className="text-slate-500">IP (trunc)</dt>
                    <dd className="font-mono">{payloadString(selected, 'ipTruncated')}</dd>
                  </>
                ) : null}
                {payloadString(selected, 'countryCode') ? (
                  <>
                    <dt className="text-slate-500">国</dt>
                    <dd>{payloadString(selected, 'countryCode')}</dd>
                  </>
                ) : null}
                {eventKind(selected.type) === 'aggregated' && payloadString(selected, 'count') ? (
                  <>
                    <dt className="text-slate-500">集約件数</dt>
                    <dd className="font-mono">{payloadString(selected, 'count')}</dd>
                  </>
                ) : null}
              </dl>
              <pre className="mt-4 max-h-[420px] overflow-auto rounded-md bg-slate-950 p-3 text-xs text-slate-50">
                {JSON.stringify(selected.payload, null, 2)}
              </pre>
            </>
          ) : (
            <p className="mt-4 text-sm text-slate-500">イベントを選択してください。</p>
          )}
        </Card>
      </div>
    </AdminShell>
  )
}

function payloadString(event: AdminAuditEvent, key: string): string {
  const value = event.payload[key]
  if (value === undefined || value === null) return ''
  return typeof value === 'string' ? value : String(value)
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid gap-1.5">
      <Label className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</Label>
      {children}
    </div>
  )
}

function formatDate(value: string): string {
  try {
    return new Date(value).toLocaleString()
  } catch {
    return value
  }
}

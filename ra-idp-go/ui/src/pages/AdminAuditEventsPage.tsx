import { IconRefresh, IconSearch } from '@tabler/icons-react'
import { type FormEvent, useState } from 'react'
import {
  type AdminAuditEventQuery,
  AuthenticationAPIError,
  listAdminAuditEvents,
} from '../api'
import { AdminShell } from '../components/AdminShell'
import { Alert } from '../components/ui/alert'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import type { AdminAuditEvent, AdminAuditEventsPage as AdminAuditEventsPageData } from '../types'

const DEFAULT_TENANT_ID = 'default'

export function AdminAuditEventsPage({
  actorUsername,
  actorRoles,
  actorTenantID,
  events: initial,
}: AdminAuditEventsPageData) {
  const [events, setEvents] = useState(initial)
  const [selected, setSelected] = useState<AdminAuditEvent | null>(initial[0] ?? null)
  const [type, setType] = useState('')
  const [sub, setSub] = useState('')
  const [after, setAfter] = useState('')
  const [before, setBefore] = useState('')
  const [limit, setLimit] = useState('100')
  const [allTenants, setAllTenants] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const canCrossTenant =
    actorRoles.includes('system_admin') && actorTenantID === DEFAULT_TENANT_ID

  async function handleQuery(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setBusy(true)
    setError('')
    try {
      const parsedLimit = limit.trim() ? Number.parseInt(limit, 10) : undefined
      const q: AdminAuditEventQuery = {
        type: type.trim() || undefined,
        sub: sub.trim() || undefined,
        after: after.trim() ? new Date(after).toISOString() : undefined,
        before: before.trim() ? new Date(before).toISOString() : undefined,
        limit: Number.isFinite(parsedLimit) ? parsedLimit : undefined,
        allTenants: canCrossTenant && allTenants,
      }
      const next = await listAdminAuditEvents(q)
      setEvents(next)
      setSelected(next[0] ?? null)
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : '監査ログを取得できませんでした。',
      )
    } finally {
      setBusy(false)
    }
  }

  return (
    <AdminShell
      active="audit-events"
      actorUsername={actorUsername}
      title="監査ログ (Audit Events)"
      description="所属テナント内の DomainEvent。コンプライアンスや調査の起点として利用します。"
    >
      {error ? <Alert variant="destructive">{error}</Alert> : null}

      <Card className="p-5">
        <form onSubmit={handleQuery} className="grid gap-4 lg:grid-cols-[repeat(5,minmax(0,1fr))_auto]">
          <Field label="Type">
            <Input
              value={type}
              onChange={(e) => setType(e.target.value)}
              placeholder="UserAuthenticated"
            />
          </Field>
          <Field label="Sub">
            <Input value={sub} onChange={(e) => setSub(e.target.value)} placeholder="user_..." />
          </Field>
          <Field label="From">
            <Input type="datetime-local" value={after} onChange={(e) => setAfter(e.target.value)} />
          </Field>
          <Field label="To">
            <Input type="datetime-local" value={before} onChange={(e) => setBefore(e.target.value)} />
          </Field>
          <Field label="Limit">
            <Input
              type="number"
              min={1}
              max={1000}
              value={limit}
              onChange={(e) => setLimit(e.target.value)}
            />
          </Field>
          <div className="flex items-end">
            <Button type="submit" disabled={busy} className="w-full lg:w-auto">
              <IconSearch size={16} aria-hidden="true" />
              絞り込み
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
                <th className="px-4 py-3">テナント</th>
              </tr>
            </thead>
            <tbody>
              {events.length === 0 ? (
                <tr>
                  <td colSpan={3} className="px-4 py-12 text-center text-sm text-slate-500">
                    一致するイベントはありません。
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
                  <td className="px-4 py-3">{e.type}</td>
                  <td className="px-4 py-3 font-mono text-xs">{e.tenant_id}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>

        <Card className="p-5">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-semibold text-slate-700">ペイロード</h2>
            {selected ? (
              <Button
                variant="ghost"
                onClick={() => navigator.clipboard?.writeText(JSON.stringify(selected.payload, null, 2))}
                aria-label="payload をコピー"
              >
                <IconRefresh size={14} aria-hidden="true" />
                コピー
              </Button>
            ) : null}
          </div>
          {selected ? (
            <>
              <dl className="mt-4 grid grid-cols-[80px_minmax(0,1fr)] gap-y-2 text-xs">
                <dt className="text-slate-500">ID</dt>
                <dd className="break-all font-mono">{selected.id}</dd>
                <dt className="text-slate-500">種別</dt>
                <dd>{selected.type}</dd>
                <dt className="text-slate-500">テナント</dt>
                <dd className="font-mono">{selected.tenant_id}</dd>
                <dt className="text-slate-500">日時</dt>
                <dd>{formatDate(selected.occurred_at)}</dd>
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

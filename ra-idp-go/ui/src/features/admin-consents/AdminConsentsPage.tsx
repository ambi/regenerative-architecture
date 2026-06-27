import { IconBan, IconRefresh } from '@tabler/icons-react'
import { useMemo, useState } from 'react'
import { AuthenticationAPIError, listAdminConsents, revokeAdminConsent } from '../../api'
import { AdminShell } from '../../components/AdminShell'
import { Alert } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import { Input } from '../../components/ui/input'
import type { AdminConsent } from '../../types'

export function AdminConsentsPage({
  csrfToken,
  actorUsername,
  consents: initial,
}: {
  csrfToken: string
  actorUsername?: string
  consents: AdminConsent[]
}) {
  const [consents, setConsents] = useState(initial)
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState<AdminConsent | null>(initial[0] ?? null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [confirmTarget, setConfirmTarget] = useState<AdminConsent | null>(null)

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase()
    if (!needle) return consents
    return consents.filter((c) =>
      [c.sub, c.client_id, c.state, ...c.scopes].some((value) =>
        value.toLowerCase().includes(needle),
      ),
    )
  }, [consents, query])

  async function refresh(preferred?: AdminConsent) {
    const next = await listAdminConsents()
    setConsents(next)
    const match = preferred
      ? next.find((c) => c.sub === preferred.sub && c.client_id === preferred.client_id)
      : null
    setSelected(match ?? next[0] ?? null)
  }

  async function run(action: () => Promise<void>, success: string) {
    setBusy(true)
    setError('')
    setNotice('')
    try {
      await action()
      setNotice(success)
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : '同意の操作を完了できませんでした。',
      )
    } finally {
      setBusy(false)
    }
  }

  async function handleRevoke(target: AdminConsent) {
    await run(async () => {
      await revokeAdminConsent(csrfToken, target.sub, target.client_id)
      await refresh(target)
    }, '同意を取り消しました。')
    setConfirmTarget(null)
  }

  return (
    <AdminShell
      active="consents"
      actorUsername={actorUsername}
      title="同意"
      description="ユーザーがアプリケーションに与えた scope の付与状況。取り消しは即時に反映されます。"
    >
      {error ? <Alert variant="destructive">{error}</Alert> : null}
      {notice ? <Alert variant="success">{notice}</Alert> : null}

      <Card className="flex flex-col gap-3 p-4 md:flex-row md:items-center md:justify-between">
        <Input
          placeholder="sub / client_id / scope で絞り込み"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="max-w-md"
        />
        <Button
          variant="outline"
          className="size-9 shrink-0 px-0"
          aria-label="一覧を再読み込み"
          disabled={busy}
          onClick={() => run(() => refresh(selected ?? undefined), '一覧を更新しました。')}
        >
          <IconRefresh size={16} aria-hidden="true" />
        </Button>
      </Card>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_360px]">
        <Card className="overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
              <tr>
                <th className="px-4 py-3">ユーザー (sub)</th>
                <th className="px-4 py-3">Client ID</th>
                <th className="px-4 py-3">状態</th>
                <th className="px-4 py-3">付与</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody>
              {filtered.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-4 py-12 text-center text-sm text-slate-500">
                    一致する同意レコードはありません。
                  </td>
                </tr>
              ) : null}
              {filtered.map((c) => (
                <tr
                  key={`${c.sub}:${c.client_id}`}
                  onClick={() => setSelected(c)}
                  className={`cursor-pointer border-t border-slate-100 hover:bg-slate-50 ${
                    selected?.sub === c.sub && selected.client_id === c.client_id
                      ? 'bg-blue-50/60'
                      : ''
                  }`}
                >
                  <td className="px-4 py-3 font-mono text-xs">{c.sub}</td>
                  <td className="px-4 py-3 font-mono text-xs">{c.client_id}</td>
                  <td className="px-4 py-3">
                    <StateBadge state={c.state} />
                  </td>
                  <td className="px-4 py-3 text-xs text-slate-500">{formatDate(c.granted_at)}</td>
                  <td className="px-4 py-3 text-right">
                    {c.state === 'granted' ? (
                      <Button
                        variant="ghost"
                        className="text-rose-700 hover:bg-rose-50"
                        disabled={busy}
                        onClick={(e) => {
                          e.stopPropagation()
                          setConfirmTarget(c)
                        }}
                      >
                        <IconBan size={16} aria-hidden="true" />
                        取消
                      </Button>
                    ) : null}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>

        <Card className="p-5">
          <h2 className="text-sm font-semibold text-slate-700">詳細</h2>
          {selected ? (
            <dl className="mt-4 grid grid-cols-[110px_minmax(0,1fr)] gap-y-3 text-sm">
              <dt className="text-slate-500">ユーザー (sub)</dt>
              <dd className="font-mono text-xs">{selected.sub}</dd>
              <dt className="text-slate-500">アプリケーション</dt>
              <dd className="font-mono text-xs">{selected.client_id}</dd>
              <dt className="text-slate-500">テナント</dt>
              <dd className="font-mono text-xs">{selected.tenant_id}</dd>
              <dt className="text-slate-500">スコープ</dt>
              <dd className="flex flex-wrap gap-1">
                {selected.scopes.length === 0 ? (
                  <span className="text-slate-400">なし</span>
                ) : (
                  selected.scopes.map((scope) => (
                    <span
                      key={scope}
                      className="rounded-md bg-slate-100 px-1.5 py-0.5 font-mono text-[11px] text-slate-700"
                    >
                      {scope}
                    </span>
                  ))
                )}
              </dd>
              <dt className="text-slate-500">状態</dt>
              <dd>
                <StateBadge state={selected.state} />
              </dd>
              <dt className="text-slate-500">付与</dt>
              <dd>{formatDate(selected.granted_at)}</dd>
              <dt className="text-slate-500">期限</dt>
              <dd>{formatDate(selected.expires_at)}</dd>
              {selected.revoked_at ? (
                <>
                  <dt className="text-slate-500">取消</dt>
                  <dd>{formatDate(selected.revoked_at)}</dd>
                </>
              ) : null}
            </dl>
          ) : (
            <p className="mt-4 text-sm text-slate-500">同意レコードを選択してください。</p>
          )}
        </Card>
      </div>

      {confirmTarget ? (
        <ConfirmDialog
          title="同意を取り消す"
          message={`${confirmTarget.sub} の ${confirmTarget.client_id} への同意を取り消します。再認可するまでアクセストークンは発行されません。`}
          confirmLabel="取り消す"
          onCancel={() => setConfirmTarget(null)}
          onConfirm={() => handleRevoke(confirmTarget)}
          busy={busy}
        />
      ) : null}
    </AdminShell>
  )
}

function StateBadge({ state }: { state: AdminConsent['state'] }) {
  const variants: Record<AdminConsent['state'], string> = {
    granted: 'bg-emerald-50 text-emerald-700',
    revoked: 'bg-rose-50 text-rose-700',
    expired: 'bg-amber-50 text-amber-700',
  }
  return (
    <span className={`rounded-md px-2 py-0.5 text-xs font-semibold ${variants[state]}`}>
      {state}
    </span>
  )
}

function formatDate(value?: string): string {
  if (!value) return '—'
  try {
    return new Date(value).toLocaleString()
  } catch {
    return value
  }
}

function ConfirmDialog({
  title,
  message,
  confirmLabel,
  onCancel,
  onConfirm,
  busy,
}: {
  title: string
  message: string
  confirmLabel: string
  onCancel: () => void
  onConfirm: () => void
  busy: boolean
}) {
  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center bg-slate-900/40 px-4">
      <Card className="w-full max-w-md p-6">
        <h2 className="text-base font-semibold text-slate-900">{title}</h2>
        <p className="mt-3 text-sm text-slate-600">{message}</p>
        <div className="mt-5 flex justify-end gap-2">
          <Button variant="outline" onClick={onCancel} disabled={busy}>
            キャンセル
          </Button>
          <Button onClick={onConfirm} disabled={busy} className="bg-rose-600 hover:bg-rose-700">
            {confirmLabel}
          </Button>
        </div>
      </Card>
    </div>
  )
}

import { IconBan, IconCheck, IconPlus, IconRefresh } from '@tabler/icons-react'
import { type FormEvent, useState } from 'react'
import {
  AuthenticationAPIError,
  createAdminTenant,
  listAdminTenants,
  setAdminTenantDisabled,
  updateAdminTenant,
} from '../../api'
import { AdminShell } from '../../components/AdminShell'
import { Alert } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import { Input } from '../../components/ui/input'
import { Label } from '../../components/ui/label'
import type { AdminTenant } from '../../types'

export function AdminTenantsPage({
  csrfToken,
  actorUsername,
  tenants: initial,
}: {
  csrfToken: string
  actorUsername?: string
  tenants: AdminTenant[]
}) {
  const [tenants, setTenants] = useState(initial)
  const [selected, setSelected] = useState<AdminTenant | null>(initial[0] ?? null)
  const [showCreate, setShowCreate] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  async function refresh(preferredID?: string) {
    const next = await listAdminTenants()
    setTenants(next)
    const match = preferredID
      ? next.find((t) => t.id === preferredID)
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
          : 'テナント操作を完了できませんでした。',
      )
    } finally {
      setBusy(false)
    }
  }

  async function handleCreate(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const form = e.currentTarget
    const data = new FormData(form)
    await run(async () => {
      const created = await createAdminTenant(csrfToken, {
        id: String(data.get('id') ?? ''),
        display_name: String(data.get('display_name') ?? ''),
      })
      form.reset()
      setShowCreate(false)
      await refresh(created.id)
    }, 'テナントを作成しました。')
  }

  async function handleToggleDisabled(target: AdminTenant) {
    const disabled = target.status === 'active'
    await run(async () => {
      await setAdminTenantDisabled(csrfToken, target.id, disabled)
      await refresh(target.id)
    }, disabled ? 'テナントを無効化しました。' : 'テナントを再有効化しました。')
  }

  return (
    <AdminShell
      active="tenants"
      actorUsername={actorUsername}
      title="テナント (Tenants)"
      description="RA Identity が分離するテナントの一覧と管理。default は無効化できません。"
      actions={
        <>
          <Button
            variant="outline"
            className="size-9 px-0"
            aria-label="一覧を再読み込み"
            onClick={() => run(() => refresh(selected?.id), '一覧を更新しました。')}
            disabled={busy}
          >
            <IconRefresh size={16} aria-hidden="true" />
          </Button>
          <Button onClick={() => setShowCreate(true)} disabled={busy}>
            <IconPlus size={16} aria-hidden="true" />
            新規テナント
          </Button>
        </>
      }
    >
      {error ? <Alert variant="destructive">{error}</Alert> : null}
      {notice ? <Alert variant="success">{notice}</Alert> : null}

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_420px]">
        <Card className="overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
              <tr>
                <th className="px-4 py-3">ID</th>
                <th className="px-4 py-3">表示名</th>
                <th className="px-4 py-3">状態</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody>
              {tenants.map((t) => (
                <tr
                  key={t.id}
                  onClick={() => setSelected(t)}
                  className={`cursor-pointer border-t border-slate-100 hover:bg-slate-50 ${
                    selected?.id === t.id ? 'bg-blue-50/60' : ''
                  }`}
                >
                  <td className="px-4 py-3 font-mono text-xs">{t.id}</td>
                  <td className="px-4 py-3">{t.display_name}</td>
                  <td className="px-4 py-3">
                    <StatusBadge status={t.status} />
                  </td>
                  <td className="px-4 py-3 text-right">
                    {t.id !== 'default' ? (
                      <Button
                        variant="ghost"
                        className={t.status === 'active' ? 'text-rose-700 hover:bg-rose-50' : 'text-emerald-700 hover:bg-emerald-50'}
                        disabled={busy}
                        onClick={(e) => {
                          e.stopPropagation()
                          handleToggleDisabled(t)
                        }}
                      >
                        {t.status === 'active' ? <IconBan size={14} aria-hidden="true" /> : <IconCheck size={14} aria-hidden="true" />}
                        {t.status === 'active' ? '無効化' : '有効化'}
                      </Button>
                    ) : null}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>

        <TenantDetailCard
          tenant={selected}
          csrfToken={csrfToken}
          busy={busy}
          onSaved={(id) => run(() => refresh(id), 'テナントを更新しました。')}
        />
      </div>

      {showCreate ? (
        <div className="fixed inset-0 z-40 flex items-center justify-center bg-slate-900/40 px-4">
          <Card className="w-full max-w-md p-6">
            <h2 className="text-base font-semibold text-slate-900">新規テナント</h2>
            <form onSubmit={handleCreate} className="mt-4 grid gap-4">
              <div className="grid gap-1.5">
                <Label htmlFor="tenant-id">ID</Label>
                <Input
                  id="tenant-id"
                  name="id"
                  required
                  pattern="^[a-z0-9][a-z0-9-]{0,62}$"
                  placeholder="acme"
                />
                <p className="text-xs text-slate-500">URL-safe slug (a-z 0-9 -)。</p>
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="tenant-display">表示名</Label>
                <Input id="tenant-display" name="display_name" required placeholder="Acme Inc." />
              </div>
              <div className="flex justify-end gap-2">
                <Button type="button" variant="outline" onClick={() => setShowCreate(false)} disabled={busy}>
                  キャンセル
                </Button>
                <Button type="submit" disabled={busy}>作成</Button>
              </div>
            </form>
          </Card>
        </div>
      ) : null}
    </AdminShell>
  )
}

function TenantDetailCard({
  tenant,
  csrfToken,
  busy,
  onSaved,
}: {
  tenant: AdminTenant | null
  csrfToken: string
  busy: boolean
  onSaved: (id: string) => void
}) {
  if (!tenant) {
    return (
      <Card className="p-5">
        <p className="text-sm text-slate-500">テナントを選択してください。</p>
      </Card>
    )
  }
  return (
    <Card className="p-5">
      <h2 className="text-sm font-semibold text-slate-700">詳細</h2>
      <dl className="mt-4 grid grid-cols-[110px_minmax(0,1fr)] gap-y-3 text-sm">
        <dt className="text-slate-500">ID</dt>
        <dd className="font-mono text-xs">{tenant.id}</dd>
        <dt className="text-slate-500">表示名</dt>
        <dd>{tenant.display_name}</dd>
        <dt className="text-slate-500">状態</dt>
        <dd><StatusBadge status={tenant.status} /></dd>
        <dt className="text-slate-500">作成</dt>
        <dd>{formatDate(tenant.created_at)}</dd>
        {tenant.disabled_at ? (
          <>
            <dt className="text-slate-500">無効化</dt>
            <dd>{formatDate(tenant.disabled_at)}</dd>
          </>
        ) : null}
      </dl>
      <TenantEditor tenant={tenant} csrfToken={csrfToken} busy={busy} onSaved={onSaved} />
    </Card>
  )
}

function TenantEditor({
  tenant,
  csrfToken,
  busy,
  onSaved,
}: {
  tenant: AdminTenant
  csrfToken: string
  busy: boolean
  onSaved: (id: string) => void
}) {
  const [displayName, setDisplayName] = useState(tenant.display_name)
  const [minLength, setMinLength] = useState(tenant.password_policy_override?.min_length?.toString() ?? '')
  const [maxLength, setMaxLength] = useState(tenant.password_policy_override?.max_length?.toString() ?? '')
  const [historyDepth, setHistoryDepth] = useState(
    tenant.password_policy_override?.history_depth?.toString() ?? '',
  )
  const [saving, setSaving] = useState(false)
  const [editError, setEditError] = useState('')

  async function handleSave(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setSaving(true)
    setEditError('')
    try {
      const policy: AdminTenant['password_policy_override'] = {}
      if (minLength.trim()) policy.min_length = Number.parseInt(minLength, 10)
      if (maxLength.trim()) policy.max_length = Number.parseInt(maxLength, 10)
      if (historyDepth.trim()) policy.history_depth = Number.parseInt(historyDepth, 10)
      const hasPolicy = Object.keys(policy).length > 0
      await updateAdminTenant(csrfToken, tenant.id, {
        display_name: displayName !== tenant.display_name ? displayName : undefined,
        password_policy_override: hasPolicy ? policy : undefined,
      })
      onSaved(tenant.id)
    } catch (cause) {
      setEditError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : 'テナントを更新できませんでした。',
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <form onSubmit={handleSave} className="mt-5 grid gap-3 border-t border-slate-100 pt-5">
      <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">編集</p>
      {editError ? <Alert variant="destructive">{editError}</Alert> : null}
      <div className="grid gap-1.5">
        <Label htmlFor={`name-${tenant.id}`}>表示名</Label>
        <Input
          id={`name-${tenant.id}`}
          value={displayName}
          onChange={(e) => setDisplayName(e.target.value)}
        />
      </div>
      <p className="mt-2 text-xs font-semibold uppercase tracking-wide text-slate-500">パスワードポリシー上書き</p>
      <p className="text-xs text-slate-500">空欄なら global default を継承します。</p>
      <div className="grid grid-cols-3 gap-2">
        <div className="grid gap-1.5">
          <Label htmlFor={`min-${tenant.id}`}>Min</Label>
          <Input
            id={`min-${tenant.id}`}
            type="number"
            min={1}
            value={minLength}
            onChange={(e) => setMinLength(e.target.value)}
          />
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor={`max-${tenant.id}`}>Max</Label>
          <Input
            id={`max-${tenant.id}`}
            type="number"
            min={1}
            value={maxLength}
            onChange={(e) => setMaxLength(e.target.value)}
          />
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor={`hist-${tenant.id}`}>History</Label>
          <Input
            id={`hist-${tenant.id}`}
            type="number"
            min={0}
            value={historyDepth}
            onChange={(e) => setHistoryDepth(e.target.value)}
          />
        </div>
      </div>
      <Button type="submit" disabled={busy || saving} className="mt-2 justify-self-start">
        {saving ? '保存中…' : '保存'}
      </Button>
    </form>
  )
}

function StatusBadge({ status }: { status: AdminTenant['status'] }) {
  return status === 'active' ? (
    <span className="rounded-md bg-emerald-50 px-2 py-0.5 text-xs font-semibold text-emerald-700">
      active
    </span>
  ) : (
    <span className="rounded-md bg-rose-50 px-2 py-0.5 text-xs font-semibold text-rose-700">
      disabled
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

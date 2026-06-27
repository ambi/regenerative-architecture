import {
  IconArrowLeft,
  IconDotsVertical,
  IconKey,
  IconPencil,
  IconPlayerStop,
  IconPlus,
  IconPower,
  IconRefresh,
  IconRobot,
  IconTrash,
  IconX,
} from '@tabler/icons-react'
import { type FormEvent, useEffect, useState } from 'react'
import {
  AuthenticationAPIError,
  bindAdminAgentCredential,
  deleteAdminAgent,
  disableAdminAgent,
  enableAdminAgent,
  getAdminAgent,
  killAdminAgent,
  listAdminAgents,
  registerAdminAgent,
  tenantURL,
  unbindAdminAgentCredential,
  updateAdminAgent,
} from '../../api'
import { AdminPaneActions } from '../../components/AdminPaneActions'
import { AdminShell } from '../../components/AdminShell'
import { Alert } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '../../components/ui/dropdown-menu'
import { Input } from '../../components/ui/input'
import { Label } from '../../components/ui/label'
import type { AdminAgent } from '../../types'

const KIND_LABELS: Record<AdminAgent['kind'], string> = {
  autonomous: '自律',
  supervised: '監督下',
}

const STATUS_LABELS: Record<AdminAgent['status'], string> = {
  active: '有効',
  disabled: '無効',
  killed: '緊急停止',
}

const STATUS_STYLES: Record<AdminAgent['status'], string> = {
  active: 'bg-emerald-100 text-emerald-700',
  disabled: 'bg-slate-200 text-slate-600',
  killed: 'bg-rose-100 text-rose-700',
}

function StatusBadge({ status }: { status: AdminAgent['status'] }) {
  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-semibold ${STATUS_STYLES[status]}`}
    >
      {STATUS_LABELS[status]}
    </span>
  )
}

export function AdminAgentsPage({
  csrfToken,
  actorUsername,
  agents: initial,
}: {
  csrfToken: string
  actorUsername?: string
  agents: AdminAgent[]
}) {
  const [agents, setAgents] = useState(initial)
  const initialID = new URLSearchParams(window.location.search).get('agent')
  const [selectedID, setSelectedID] = useState<string>(
    () => initial.find((a) => a.id === initialID)?.id ?? initial[0]?.id ?? '',
  )
  const [showCreate, setShowCreate] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  const selected = agents.find((a) => a.id === selectedID) ?? null

  async function refresh(preferredID = selectedID) {
    const next = await listAdminAgents()
    setAgents(next)
    setSelectedID(next.find((a) => a.id === preferredID)?.id ?? next[0]?.id ?? '')
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
          : 'エージェント操作を完了できませんでした。',
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
      const created = await registerAdminAgent(csrfToken, {
        name: String(data.get('name') ?? ''),
        description: optionalValue(data.get('description')),
        kind: (String(data.get('kind') ?? 'autonomous') as AdminAgent['kind']) || undefined,
        owner_sub: optionalValue(data.get('owner_sub')),
        roles: parseRoles(String(data.get('roles') ?? '')),
      })
      form.reset()
      setShowCreate(false)
      await refresh(created.id)
    }, 'エージェントを登録しました。')
  }

  return (
    <AdminShell
      active="agents"
      actorUsername={actorUsername}
      title="エージェント"
      description="非人間プリンシパルとしてのエージェントを登録し、ロールとクライアント資格情報を管理します。"
      actions={
        <>
          <Button
            variant="outline"
            className="size-9 px-0"
            aria-label="一覧を再読み込み"
            onClick={() => run(() => refresh(), '一覧を更新しました。')}
            disabled={busy}
          >
            <IconRefresh size={16} aria-hidden="true" />
          </Button>
          <Button onClick={() => setShowCreate(true)} disabled={busy}>
            <IconPlus size={16} aria-hidden="true" />
            登録
          </Button>
        </>
      }
    >
      {error ? <Alert variant="destructive">{error}</Alert> : null}
      {notice ? <Alert variant="success">{notice}</Alert> : null}

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_440px]">
        <Card className="overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
              <tr>
                <th className="px-4 py-3">エージェント</th>
                <th className="px-4 py-3">種別</th>
                <th className="px-4 py-3">所有者</th>
                <th className="px-4 py-3">状態</th>
                <th className="px-4 py-3 text-right">ロール / 資格情報</th>
              </tr>
            </thead>
            <tbody>
              {agents.map((agent) => (
                <tr
                  key={agent.id}
                  onClick={() => setSelectedID(agent.id)}
                  className={`cursor-pointer border-t border-slate-100 hover:bg-slate-50 ${
                    selectedID === agent.id ? 'bg-blue-50/60' : ''
                  }`}
                >
                  <td className="px-4 py-3">
                    <div className="font-semibold text-slate-900">{agent.name}</div>
                    {agent.description ? (
                      <div className="truncate text-xs text-slate-500">{agent.description}</div>
                    ) : null}
                  </td>
                  <td className="px-4 py-3 text-xs text-slate-600">{KIND_LABELS[agent.kind]}</td>
                  <td className="px-4 py-3 font-mono text-xs text-slate-600">
                    {agent.owner_sub || '—'}
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={agent.status} />
                  </td>
                  <td className="px-4 py-3 text-right text-xs text-slate-600">
                    {agent.roles.length} / {agent.client_ids.length}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {agents.length === 0 ? (
            <div className="flex min-h-40 flex-col items-center justify-center px-6 text-center text-sm text-slate-500">
              <IconRobot size={24} className="text-slate-400" aria-hidden="true" />
              <p className="mt-3">エージェントはまだありません。</p>
            </div>
          ) : null}
        </Card>

        <AgentDetailCard
          agent={selected}
          csrfToken={csrfToken}
          busy={busy}
          detailHref={
            selected ? tenantURL(`/admin/agents/${encodeURIComponent(selected.id)}`) : undefined
          }
          onChanged={(id) => run(() => refresh(id), 'エージェントを更新しました。')}
          onDeleted={() => run(() => refresh(), 'エージェントを削除しました。')}
        />
      </div>

      {showCreate ? (
        <div className="fixed inset-0 z-40 flex items-center justify-center bg-slate-900/40 px-4">
          <Card className="w-full max-w-md p-6">
            <div className="flex items-center justify-between">
              <h2 className="text-base font-semibold text-slate-900">エージェントを登録</h2>
              <Button
                variant="ghost"
                className="px-2.5"
                onClick={() => setShowCreate(false)}
                aria-label="閉じる"
              >
                <IconX size={18} aria-hidden="true" />
              </Button>
            </div>
            <form onSubmit={handleCreate} className="mt-4 grid gap-4">
              <div className="grid gap-1.5">
                <Label htmlFor="agent-name">エージェント名</Label>
                <Input id="agent-name" name="name" required placeholder="invoice-bot" />
                <p className="text-xs text-slate-500">テナント内で一意の表示名。</p>
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="agent-description">説明 (任意)</Label>
                <Input
                  id="agent-description"
                  name="description"
                  placeholder="請求書処理エージェント"
                />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="agent-kind">種別</Label>
                <select
                  id="agent-kind"
                  name="kind"
                  defaultValue="autonomous"
                  className="h-9 rounded-md border border-slate-300 bg-white px-2 text-sm"
                >
                  <option value="autonomous">自律 (autonomous)</option>
                  <option value="supervised">監督下 (supervised)</option>
                </select>
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="agent-owner">所有者 sub (任意)</Label>
                <Input id="agent-owner" name="owner_sub" placeholder="user-1234" />
                <p className="text-xs text-slate-500">省略時は操作した管理者が所有者になります。</p>
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="agent-roles">ロール</Label>
                <Input id="agent-roles" name="roles" placeholder="invoice:read, invoice:write" />
                <p className="text-xs text-slate-500">カンマ区切り。エージェントに付与されます。</p>
              </div>
              <div className="flex justify-end gap-2">
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => setShowCreate(false)}
                  disabled={busy}
                >
                  キャンセル
                </Button>
                <Button type="submit" disabled={busy}>
                  登録
                </Button>
              </div>
            </form>
          </Card>
        </div>
      ) : null}
    </AdminShell>
  )
}

// AdminAgentDetailPage はエージェントの編集・状態操作・資格情報管理を扱う詳細画面 (wi-49)。
export function AdminAgentDetailPage({
  csrfToken,
  actorUsername,
  agent: initialAgent,
}: {
  csrfToken: string
  actorUsername?: string
  agent: AdminAgent
}) {
  const [agent, setAgent] = useState(initialAgent)
  const [editing, setEditing] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [confirmKill, setConfirmKill] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function reload(id: string) {
    try {
      const next = await getAdminAgent(id)
      setAgent(next)
      setError('')
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : 'エージェントの再取得に失敗しました。',
      )
    }
  }

  async function run(action: () => Promise<void>) {
    setBusy(true)
    setError('')
    try {
      await action()
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : 'エージェント操作を完了できませんでした。',
      )
    } finally {
      setBusy(false)
    }
  }

  async function handleDelete() {
    setBusy(true)
    setError('')
    try {
      await deleteAdminAgent(csrfToken, agent.id)
      window.location.assign(tenantURL('/admin/agents'))
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : 'エージェントを削除できませんでした。',
      )
      setBusy(false)
    }
  }

  const killed = agent.status === 'killed'

  return (
    <>
      <AdminShell
        active="agents"
        actorUsername={actorUsername}
        title={agent.name}
        description={agent.description || agent.id}
        actions={
          <div className="flex items-center gap-2">
            <a
              href={tenantURL('/admin/agents')}
              className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 transition-colors hover:bg-slate-50"
            >
              <IconArrowLeft size={16} aria-hidden="true" />
              エージェント一覧
            </a>
            <Button type="button" disabled={busy || killed} onClick={() => setEditing(true)}>
              <IconPencil size={16} aria-hidden="true" />
              編集
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  type="button"
                  variant="outline"
                  className="size-9 px-0"
                  aria-label="エージェント操作"
                  disabled={busy}
                >
                  <IconDotsVertical size={18} aria-hidden="true" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                {!killed && agent.status === 'active' ? (
                  <DropdownMenuItem
                    onSelect={() =>
                      void run(async () => {
                        await disableAdminAgent(csrfToken, agent.id)
                        await reload(agent.id)
                      })
                    }
                  >
                    <IconPower size={17} aria-hidden="true" />
                    無効化
                  </DropdownMenuItem>
                ) : null}
                {!killed && agent.status === 'disabled' ? (
                  <DropdownMenuItem
                    onSelect={() =>
                      void run(async () => {
                        await enableAdminAgent(csrfToken, agent.id)
                        await reload(agent.id)
                      })
                    }
                  >
                    <IconPower size={17} aria-hidden="true" />
                    有効化
                  </DropdownMenuItem>
                ) : null}
                {!killed ? (
                  <DropdownMenuItem className="text-rose-700" onSelect={() => setConfirmKill(true)}>
                    <IconPlayerStop size={17} aria-hidden="true" />
                    緊急停止 (kill)
                  </DropdownMenuItem>
                ) : null}
                <DropdownMenuItem className="text-red-700" onSelect={() => setConfirmDelete(true)}>
                  <IconTrash size={17} aria-hidden="true" />
                  エージェントを削除
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        }
      >
        {error ? <Alert variant="destructive">{error}</Alert> : null}
        {confirmKill ? (
          <Alert
            variant="destructive"
            className="flex flex-wrap items-center justify-between gap-2"
          >
            <span>
              このエージェントを緊急停止しますか？緊急停止は<strong>一方向</strong>の操作で、
              取り消せません。停止後は再有効化できず、すべての資格情報が無効になります。
            </span>
            <div className="flex gap-2">
              <Button variant="outline" onClick={() => setConfirmKill(false)} disabled={busy}>
                取消
              </Button>
              <Button
                variant="destructive"
                disabled={busy}
                onClick={() =>
                  void run(async () => {
                    await killAdminAgent(csrfToken, agent.id)
                    setConfirmKill(false)
                    await reload(agent.id)
                  })
                }
              >
                <IconPlayerStop size={14} aria-hidden="true" />
                緊急停止を確定
              </Button>
            </div>
          </Alert>
        ) : null}
        {confirmDelete ? (
          <Alert
            variant="destructive"
            className="flex flex-wrap items-center justify-between gap-2"
          >
            <span>このエージェントを削除しますか？バインドされた資格情報も解除されます。</span>
            <div className="flex gap-2">
              <Button variant="outline" onClick={() => setConfirmDelete(false)} disabled={busy}>
                取消
              </Button>
              <Button variant="destructive" disabled={busy} onClick={() => void handleDelete()}>
                <IconTrash size={14} aria-hidden="true" />
                削除を確定
              </Button>
            </div>
          </Alert>
        ) : null}
        <div className="max-w-3xl">
          <AgentDetailCard
            agent={agent}
            csrfToken={csrfToken}
            busy={busy}
            showActions={false}
            onChanged={(id) => void reload(id)}
            onDeleted={() => window.location.assign(tenantURL('/admin/agents'))}
          />
        </div>
      </AdminShell>
      {editing ? (
        <AgentEditorDialog
          agent={agent}
          csrfToken={csrfToken}
          onClose={() => setEditing(false)}
          onSaved={(id) => {
            setEditing(false)
            void reload(id)
          }}
        />
      ) : null}
    </>
  )
}

function AgentDetailCard({
  agent,
  csrfToken,
  busy,
  detailHref,
  showActions = true,
  onChanged,
  onDeleted,
}: {
  agent: AdminAgent | null
  csrfToken: string
  busy: boolean
  detailHref?: string
  showActions?: boolean
  onChanged: (id: string) => void
  onDeleted: () => void
}) {
  const [clientIDs, setClientIDs] = useState<string[]>(agent?.client_ids ?? [])
  const [status, setStatus] = useState<AdminAgent['status']>(agent?.status ?? 'active')
  const [addClientID, setAddClientID] = useState('')
  const [localBusy, setLocalBusy] = useState(false)
  const [localError, setLocalError] = useState('')
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [editing, setEditing] = useState(false)

  useEffect(() => {
    setConfirmDelete(false)
    setEditing(false)
    setLocalError('')
    setAddClientID('')
    setClientIDs(agent?.client_ids ?? [])
    setStatus(agent?.status ?? 'active')
  }, [agent])

  if (!agent) {
    return (
      <Card className="p-5">
        <p className="text-sm text-slate-500">エージェントを選択してください。</p>
      </Card>
    )
  }
  const activeAgent = agent
  const killed = status === 'killed'

  async function withLocal(action: () => Promise<void>) {
    setLocalBusy(true)
    setLocalError('')
    try {
      await action()
    } catch (cause) {
      setLocalError(
        cause instanceof AuthenticationAPIError ? cause.message : '操作を完了できませんでした。',
      )
    } finally {
      setLocalBusy(false)
    }
  }

  async function reloadCredentials() {
    const next = await getAdminAgent(activeAgent.id)
    setClientIDs(next.client_ids)
    setStatus(next.status)
  }

  return (
    <>
      <Card className="overflow-hidden">
        <div className="border-b border-slate-200 bg-white p-5">
          <div className="flex items-start gap-3">
            <span className="flex size-11 shrink-0 items-center justify-center rounded-lg bg-slate-100 text-slate-700">
              <IconRobot size={22} aria-hidden="true" />
            </span>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <h2 className="truncate text-lg font-semibold text-slate-950">{agent.name}</h2>
                <StatusBadge status={status} />
              </div>
              <p className="mt-0.5 truncate font-mono text-sm text-slate-500">{agent.id}</p>
            </div>
          </div>

          {showActions ? (
            <div className="mt-4">
              <AdminPaneActions
                detailHref={detailHref}
                busy={busy || localBusy}
                onEdit={killed ? undefined : () => setEditing(true)}
                menu={
                  <DropdownMenuItem
                    className="text-red-700"
                    onSelect={() => setConfirmDelete(true)}
                  >
                    <IconTrash size={17} aria-hidden="true" />
                    エージェントを削除
                  </DropdownMenuItem>
                }
              />
            </div>
          ) : null}
        </div>

        {confirmDelete ? (
          <Alert
            variant="destructive"
            className="m-5 flex flex-wrap items-center justify-between gap-2"
          >
            <span>このエージェントを削除しますか？バインドされた資格情報も解除されます。</span>
            <div className="flex gap-2">
              <Button
                variant="outline"
                onClick={() => setConfirmDelete(false)}
                disabled={localBusy}
              >
                取消
              </Button>
              <Button
                variant="destructive"
                disabled={busy || localBusy}
                onClick={() =>
                  void withLocal(async () => {
                    await deleteAdminAgent(csrfToken, activeAgent.id)
                    onDeleted()
                  })
                }
              >
                <IconTrash size={14} aria-hidden="true" />
                削除を確定
              </Button>
            </div>
          </Alert>
        ) : null}

        {localError ? (
          <Alert variant="destructive" className="m-5">
            {localError}
          </Alert>
        ) : null}

        <dl className="grid gap-4 p-5">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <dt className="text-xs font-bold uppercase tracking-normal text-slate-400">種別</dt>
              <dd className="mt-1 text-sm text-slate-700">{KIND_LABELS[agent.kind]}</dd>
            </div>
            <div>
              <dt className="text-xs font-bold uppercase tracking-normal text-slate-400">所有者</dt>
              <dd className="mt-1 truncate font-mono text-sm text-slate-700">
                {agent.owner_sub || '—'}
              </dd>
            </div>
          </div>
          <div>
            <dt className="text-xs font-bold uppercase tracking-normal text-slate-400">説明</dt>
            <dd className="mt-1 text-sm text-slate-700">{agent.description || '—'}</dd>
          </div>
          <div>
            <dt className="text-xs font-bold uppercase tracking-normal text-slate-400">ロール</dt>
            <dd className="mt-1 flex flex-wrap gap-1.5">
              {agent.roles.length > 0 ? (
                agent.roles.map((role) => (
                  <span
                    key={role}
                    className="rounded-md bg-slate-100 px-2 py-1 font-mono text-xs text-slate-700"
                  >
                    {role}
                  </span>
                ))
              ) : (
                <span className="text-sm text-slate-400">なし</span>
              )}
            </dd>
          </div>
        </dl>

        <section className="border-t border-slate-100 p-5">
          <h3 className="text-xs font-bold uppercase tracking-normal text-slate-400">
            資格情報 ({clientIDs.length})
          </h3>
          <p className="mt-1 text-xs text-slate-500">
            エージェントが利用する OAuth クライアントをバインドします。
          </p>
          <ul className="mt-3 grid gap-2">
            {clientIDs.map((clientID) => (
              <li
                key={clientID}
                className="flex items-center justify-between rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm"
              >
                <span className="truncate font-mono text-slate-700">{clientID}</span>
                <Button
                  variant="ghost"
                  className="text-rose-700 hover:bg-rose-50"
                  disabled={localBusy || killed}
                  onClick={() =>
                    withLocal(async () => {
                      await unbindAdminAgentCredential(csrfToken, activeAgent.id, clientID)
                      await reloadCredentials()
                    })
                  }
                >
                  <IconX size={14} aria-hidden="true" />
                  解除
                </Button>
              </li>
            ))}
            {clientIDs.length === 0 ? (
              <li className="text-xs text-slate-400">バインドされた資格情報はありません。</li>
            ) : null}
          </ul>

          <div className="mt-3 flex items-center gap-2">
            <Input
              value={addClientID}
              onChange={(e) => setAddClientID(e.target.value)}
              placeholder="client_id"
              aria-label="バインドする client_id"
              disabled={killed}
            />
            <Button
              disabled={localBusy || killed || !addClientID.trim()}
              onClick={() =>
                withLocal(async () => {
                  await bindAdminAgentCredential(csrfToken, activeAgent.id, addClientID.trim())
                  setAddClientID('')
                  await reloadCredentials()
                })
              }
            >
              <IconKey size={14} aria-hidden="true" />
              バインド
            </Button>
          </div>
        </section>
      </Card>
      {editing ? (
        <AgentEditorDialog
          agent={activeAgent}
          csrfToken={csrfToken}
          onClose={() => setEditing(false)}
          onSaved={(id) => {
            setEditing(false)
            onChanged(id)
          }}
        />
      ) : null}
    </>
  )
}

function AgentEditorDialog({
  agent,
  csrfToken,
  onClose,
  onSaved,
}: {
  agent: AdminAgent
  csrfToken: string
  onClose: () => void
  onSaved: (id: string) => void
}) {
  const [name, setName] = useState(agent.name)
  const [description, setDescription] = useState(agent.description ?? '')
  const [kind, setKind] = useState<AdminAgent['kind']>(agent.kind)
  const [ownerSub, setOwnerSub] = useState(agent.owner_sub)
  const [roles, setRoles] = useState(agent.roles.join(', '))
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const trimmedName = name.trim()
  const nextRoles = parseRoles(roles)
  const nameInvalid = trimmedName === ''
  const changed =
    trimmedName !== agent.name ||
    description.trim() !== (agent.description ?? '') ||
    kind !== agent.kind ||
    ownerSub.trim() !== agent.owner_sub ||
    nextRoles.join(',') !== agent.roles.join(',')

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (nameInvalid || !changed) return
    setSaving(true)
    setError('')
    try {
      await updateAdminAgent(csrfToken, agent.id, {
        name: trimmedName !== agent.name ? trimmedName : undefined,
        description:
          description.trim() !== (agent.description ?? '') ? description.trim() : undefined,
        kind: kind !== agent.kind ? kind : undefined,
        owner_sub: ownerSub.trim() !== agent.owner_sub ? ownerSub.trim() : undefined,
        roles: nextRoles.join(',') !== agent.roles.join(',') ? nextRoles : undefined,
      })
      onSaved(agent.id)
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : 'エージェントを更新できませんでした。',
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/30 p-5 backdrop-blur-[2px]"
      role="dialog"
      aria-modal="true"
      aria-labelledby="agent-editor-title"
    >
      <button
        type="button"
        className="absolute inset-0 cursor-default"
        aria-label="閉じる"
        onClick={onClose}
      />
      <Card className="relative flex max-h-[88vh] w-full max-w-lg flex-col overflow-hidden shadow-2xl">
        <div className="flex items-start justify-between border-b border-slate-200 px-6 py-5">
          <div>
            <p className="text-xs font-bold uppercase tracking-normal text-blue-700">
              エージェント
            </p>
            <h2 id="agent-editor-title" className="mt-1 text-xl font-semibold">
              エージェントを編集
            </h2>
            <p className="mt-1 text-sm text-slate-500">{agent.name}</p>
          </div>
          <Button variant="ghost" className="px-2.5" onClick={onClose} aria-label="閉じる">
            <IconX size={18} aria-hidden="true" />
          </Button>
        </div>
        <form onSubmit={handleSubmit} className="flex min-h-0 flex-1 flex-col">
          <div className="min-h-0 flex-1 overflow-y-auto">
            {error ? (
              <Alert variant="destructive" className="mb-4">
                {error}
              </Alert>
            ) : null}
            <div className="grid gap-6 p-6">
              <section className="grid gap-4">
                <h3 className="text-xs font-bold uppercase tracking-normal text-slate-400">
                  基本情報
                </h3>
                <div className="grid gap-1.5">
                  <Label htmlFor="agent-editor-name">エージェント名</Label>
                  <Input
                    id="agent-editor-name"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    required
                    aria-invalid={nameInvalid}
                  />
                </div>
                <div className="grid gap-1.5">
                  <Label htmlFor="agent-editor-description">説明</Label>
                  <Input
                    id="agent-editor-description"
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                  />
                </div>
                <div className="grid gap-1.5">
                  <Label htmlFor="agent-editor-kind">種別</Label>
                  <select
                    id="agent-editor-kind"
                    value={kind}
                    onChange={(e) => setKind(e.target.value as AdminAgent['kind'])}
                    className="h-9 rounded-md border border-slate-300 bg-white px-2 text-sm"
                  >
                    <option value="autonomous">自律 (autonomous)</option>
                    <option value="supervised">監督下 (supervised)</option>
                  </select>
                </div>
                <div className="grid gap-1.5">
                  <Label htmlFor="agent-editor-owner">所有者 sub</Label>
                  <Input
                    id="agent-editor-owner"
                    value={ownerSub}
                    onChange={(e) => setOwnerSub(e.target.value)}
                  />
                </div>
              </section>
              <section className="grid gap-3 border-t border-slate-200 pt-5">
                <h3 className="text-xs font-bold uppercase tracking-normal text-slate-400">
                  ロール
                </h3>
                <div className="grid gap-1.5">
                  <Label htmlFor="agent-editor-roles">ロール</Label>
                  <Input
                    id="agent-editor-roles"
                    value={roles}
                    onChange={(e) => setRoles(e.target.value)}
                    placeholder="invoice:read, invoice:write"
                  />
                  <p className="text-xs text-slate-500">
                    カンマ区切り。エージェントに付与されます。
                  </p>
                </div>
              </section>
            </div>
          </div>
          <div className="flex justify-end gap-2 border-t border-slate-200 bg-slate-50 px-6 py-4">
            <Button type="button" variant="outline" onClick={onClose}>
              キャンセル
            </Button>
            <Button type="submit" disabled={saving || nameInvalid || !changed}>
              {saving ? '保存中…' : '保存'}
            </Button>
          </div>
        </form>
      </Card>
    </div>
  )
}

function parseRoles(value: string) {
  return [
    ...new Set(
      value
        .split(',')
        .map((role) => role.trim())
        .filter(Boolean),
    ),
  ]
}

function optionalValue(value: FormDataEntryValue | null) {
  const normalized = String(value ?? '').trim()
  return normalized || undefined
}

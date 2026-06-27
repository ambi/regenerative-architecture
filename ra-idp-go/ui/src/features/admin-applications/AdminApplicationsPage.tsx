import { IconApps, IconPlus, IconTrash } from '@tabler/icons-react'
import { type FormEvent, useState } from 'react'
import {
  assignApplication,
  attachProtocolBinding,
  AuthenticationAPIError,
  createAdminApplication,
  deleteAdminApplication,
  detachProtocolBinding,
  listApplicationAssignments,
  unassignApplication,
  updateAdminApplication,
} from '../../api'
import { AdminShell } from '../../components/AdminShell'
import { Alert } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import { Input } from '../../components/ui/input'
import { Label } from '../../components/ui/label'
import type {
  AdminApplication,
  ApplicationAssignment,
  ApplicationKind,
  ProtocolBinding,
  ProtocolBindingType,
} from '../../types'

const selectClass =
  'h-9 rounded-lg border border-slate-300 bg-white px-2 text-sm focus:border-blue-500 focus:outline-none'

function messageOf(cause: unknown, fallback: string): string {
  return cause instanceof AuthenticationAPIError ? cause.message : fallback
}

function StatusBadge({ status }: { status: AdminApplication['status'] }) {
  const active = status === 'active'
  return (
    <span
      className={`rounded-md px-2 py-0.5 text-xs font-medium ${
        active ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-100 text-slate-500'
      }`}
    >
      {active ? '有効' : '無効'}
    </span>
  )
}

function BindingsSection({
  app,
  csrfToken,
  onChange,
  onError,
}: {
  app: AdminApplication
  csrfToken: string
  onChange: (app: AdminApplication) => void
  onError: (msg: string) => void
}) {
  const [type, setType] = useState<ProtocolBindingType>('oidc')
  const [key, setKey] = useState('')
  const [pending, setPending] = useState(false)

  async function add(event: FormEvent) {
    event.preventDefault()
    setPending(true)
    try {
      const binding: ProtocolBinding =
        type === 'wsfed' ? { type, wtrealm: key } : { type, client_id: key }
      onChange(await attachProtocolBinding(csrfToken, app.application_id, binding))
      setKey('')
    } catch (cause) {
      onError(messageOf(cause, 'バインディングを追加できませんでした。'))
    } finally {
      setPending(false)
    }
  }

  async function remove(bindingType: ProtocolBindingType) {
    try {
      await detachProtocolBinding(csrfToken, app.application_id, bindingType)
      onChange({ ...app, bindings: app.bindings.filter((b) => b.type !== bindingType) })
    } catch (cause) {
      onError(messageOf(cause, 'バインディングを解除できませんでした。'))
    }
  }

  return (
    <div className="space-y-2">
      <p className="text-xs font-semibold text-slate-500">プロトコルバインディング</p>
      {app.bindings.length === 0 ? (
        <p className="text-xs text-slate-400">バインディングはありません。</p>
      ) : (
        <ul className="space-y-1">
          {app.bindings.map((b) => (
            <li
              key={b.type}
              className="flex items-center justify-between rounded-md bg-slate-50 px-2 py-1 text-xs"
            >
              <span className="font-mono text-slate-700">
                {b.type}: {b.client_id ?? b.wtrealm}
              </span>
              <button
                type="button"
                className="text-red-600 hover:underline"
                onClick={() => remove(b.type)}
              >
                解除
              </button>
            </li>
          ))}
        </ul>
      )}
      {app.kind === 'federated' ? (
        <form className="flex flex-wrap items-center gap-2" onSubmit={add}>
          <select
            className={selectClass}
            value={type}
            onChange={(e) => setType(e.target.value as ProtocolBindingType)}
            aria-label="バインディング種別"
          >
            <option value="oidc">oidc</option>
            <option value="wsfed">wsfed</option>
            <option value="saml">saml</option>
          </select>
          <Input
            value={key}
            onChange={(e) => setKey(e.target.value)}
            placeholder={type === 'wsfed' ? 'wtrealm' : 'client_id'}
            className="h-9 w-48"
          />
          <Button type="submit" variant="outline" disabled={pending || key.trim() === ''}>
            <IconPlus size={14} aria-hidden="true" />
            追加
          </Button>
        </form>
      ) : null}
    </div>
  )
}

function AssignmentsSection({
  app,
  csrfToken,
  onError,
}: {
  app: AdminApplication
  csrfToken: string
  onError: (msg: string) => void
}) {
  const [assignments, setAssignments] = useState<ApplicationAssignment[] | null>(null)
  const [subjectType, setSubjectType] = useState<'user' | 'group'>('user')
  const [subjectID, setSubjectID] = useState('')
  const [visibility, setVisibility] = useState<'visible' | 'hidden'>('visible')
  const [pending, setPending] = useState(false)

  async function load() {
    try {
      setAssignments(await listApplicationAssignments(app.application_id))
    } catch (cause) {
      onError(messageOf(cause, '割当を取得できませんでした。'))
    }
  }

  async function add(event: FormEvent) {
    event.preventDefault()
    setPending(true)
    try {
      await assignApplication(csrfToken, app.application_id, {
        subject_type: subjectType,
        subject_id: subjectID,
        visibility,
      })
      setSubjectID('')
      await load()
    } catch (cause) {
      onError(messageOf(cause, '割当を追加できませんでした。'))
    } finally {
      setPending(false)
    }
  }

  async function remove(a: ApplicationAssignment) {
    try {
      await unassignApplication(csrfToken, app.application_id, a.subject_type, a.subject_id)
      setAssignments((current) =>
        (current ?? []).filter(
          (x) => !(x.subject_type === a.subject_type && x.subject_id === a.subject_id),
        ),
      )
    } catch (cause) {
      onError(messageOf(cause, '割当を解除できませんでした。'))
    }
  }

  if (assignments === null) {
    return (
      <Button type="button" variant="outline" onClick={load}>
        割当を管理
      </Button>
    )
  }

  return (
    <div className="space-y-2">
      <p className="text-xs font-semibold text-slate-500">割当 (ユーザー / グループ)</p>
      {assignments.length === 0 ? (
        <p className="text-xs text-slate-400">
          割当はありません。未割当の利用者はログインできません。
        </p>
      ) : (
        <ul className="space-y-1">
          {assignments.map((a) => (
            <li
              key={`${a.subject_type}:${a.subject_id}`}
              className="flex items-center justify-between rounded-md bg-slate-50 px-2 py-1 text-xs"
            >
              <span className="text-slate-700">
                {a.subject_type}: {a.subject_id}
                {a.visibility === 'hidden' ? (
                  <span className="ml-2 rounded bg-amber-50 px-1.5 py-0.5 text-amber-700">
                    非表示
                  </span>
                ) : null}
              </span>
              <button
                type="button"
                className="text-red-600 hover:underline"
                onClick={() => remove(a)}
              >
                解除
              </button>
            </li>
          ))}
        </ul>
      )}
      <form className="flex flex-wrap items-center gap-2" onSubmit={add}>
        <select
          className={selectClass}
          value={subjectType}
          onChange={(e) => setSubjectType(e.target.value as 'user' | 'group')}
          aria-label="対象種別"
        >
          <option value="user">user</option>
          <option value="group">group</option>
        </select>
        <Input
          value={subjectID}
          onChange={(e) => setSubjectID(e.target.value)}
          placeholder={subjectType === 'user' ? 'sub' : 'group id'}
          className="h-9 w-44"
        />
        <select
          className={selectClass}
          value={visibility}
          onChange={(e) => setVisibility(e.target.value as 'visible' | 'hidden')}
          aria-label="可視性"
        >
          <option value="visible">表示</option>
          <option value="hidden">非表示</option>
        </select>
        <Button type="submit" variant="outline" disabled={pending || subjectID.trim() === ''}>
          <IconPlus size={14} aria-hidden="true" />
          割当
        </Button>
      </form>
    </div>
  )
}

function ApplicationCard({
  app,
  csrfToken,
  onChange,
  onRemove,
  onError,
}: {
  app: AdminApplication
  csrfToken: string
  onChange: (app: AdminApplication) => void
  onRemove: (id: string) => void
  onError: (msg: string) => void
}) {
  async function toggleStatus() {
    try {
      onChange(
        await updateAdminApplication(csrfToken, app.application_id, {
          status: app.status === 'active' ? 'disabled' : 'active',
        }),
      )
    } catch (cause) {
      onError(messageOf(cause, '状態を更新できませんでした。'))
    }
  }

  async function remove() {
    try {
      await deleteAdminApplication(csrfToken, app.application_id)
      onRemove(app.application_id)
    } catch (cause) {
      onError(messageOf(cause, 'アプリケーションを削除できませんでした。'))
    }
  }

  return (
    <Card className="space-y-4 p-5">
      <div className="flex flex-wrap items-start gap-3">
        {app.icon_url ? (
          <img src={app.icon_url} alt="" className="size-10 rounded-lg object-cover" />
        ) : (
          <span className="flex size-10 items-center justify-center rounded-lg border border-blue-100 bg-blue-50 text-xs font-bold text-blue-700">
            {app.name.slice(0, 2).toUpperCase()}
          </span>
        )}
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <p className="font-semibold text-slate-900">{app.name}</p>
            <StatusBadge status={app.status} />
            <span className="rounded-md bg-slate-100 px-2 py-0.5 text-xs text-slate-600">
              {app.kind === 'weblink' ? 'Web リンク' : 'フェデレーション'}
            </span>
          </div>
          {app.launch_url ? (
            <p className="mt-0.5 truncate text-xs text-slate-400">{app.launch_url}</p>
          ) : null}
        </div>
        <div className="flex gap-2">
          <Button type="button" variant="outline" onClick={toggleStatus}>
            {app.status === 'active' ? '無効化' : '有効化'}
          </Button>
          <Button
            type="button"
            variant="outline"
            className="text-red-700 hover:bg-red-50"
            onClick={remove}
          >
            <IconTrash size={16} aria-hidden="true" />
            削除
          </Button>
        </div>
      </div>
      {app.kind === 'federated' ? (
        <BindingsSection app={app} csrfToken={csrfToken} onChange={onChange} onError={onError} />
      ) : null}
      <AssignmentsSection app={app} csrfToken={csrfToken} onError={onError} />
    </Card>
  )
}

export function AdminApplicationsPage({
  csrfToken,
  actorUsername,
  applications: initial,
}: {
  csrfToken: string
  actorUsername?: string
  applications: AdminApplication[]
}) {
  const [applications, setApplications] = useState<AdminApplication[]>(initial)
  const [error, setError] = useState('')
  const [name, setName] = useState('')
  const [kind, setKind] = useState<ApplicationKind>('federated')
  const [launchURL, setLaunchURL] = useState('')
  const [iconURL, setIconURL] = useState('')
  const [pending, setPending] = useState(false)

  function replace(app: AdminApplication) {
    setApplications((current) =>
      current.map((x) => (x.application_id === app.application_id ? app : x)),
    )
  }

  async function create(event: FormEvent) {
    event.preventDefault()
    setPending(true)
    setError('')
    try {
      const created = await createAdminApplication(csrfToken, {
        name,
        kind,
        icon_url: iconURL || undefined,
        launch_url: launchURL || undefined,
      })
      setApplications((current) => [created, ...current])
      setName('')
      setLaunchURL('')
      setIconURL('')
    } catch (cause) {
      setError(messageOf(cause, 'アプリケーションを作成できませんでした。'))
    } finally {
      setPending(false)
    }
  }

  return (
    <AdminShell
      active="applications"
      actorUsername={actorUsername}
      title="アプリケーション"
      description="接続する業務アプリケーションを登録し、プロトコルバインディングと割当を管理します。"
    >
      {error ? <Alert variant="destructive">{error}</Alert> : null}

      <Card className="space-y-3 p-5">
        <p className="text-sm font-semibold text-slate-700">アプリケーションを追加</p>
        <form className="flex flex-wrap items-end gap-3" onSubmit={create}>
          <div className="space-y-1">
            <Label htmlFor="app-name">名前</Label>
            <Input
              id="app-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="w-56"
              placeholder="Payroll"
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="app-kind">種別</Label>
            <select
              id="app-kind"
              className={selectClass}
              value={kind}
              onChange={(e) => setKind(e.target.value as ApplicationKind)}
            >
              <option value="federated">フェデレーション</option>
              <option value="weblink">Web リンク</option>
            </select>
          </div>
          <div className="space-y-1">
            <Label htmlFor="app-icon">アイコン URL</Label>
            <Input
              id="app-icon"
              value={iconURL}
              onChange={(e) => setIconURL(e.target.value)}
              className="w-56"
              placeholder="https://…/icon.png"
            />
          </div>
          {kind === 'weblink' ? (
            <div className="space-y-1">
              <Label htmlFor="app-launch">起動 URL</Label>
              <Input
                id="app-launch"
                value={launchURL}
                onChange={(e) => setLaunchURL(e.target.value)}
                className="w-56"
                placeholder="https://app.example"
              />
            </div>
          ) : null}
          <Button type="submit" disabled={pending || name.trim() === ''}>
            <IconPlus size={16} aria-hidden="true" />
            追加
          </Button>
        </form>
      </Card>

      {applications.length === 0 ? (
        <Card className="flex flex-col items-center gap-2 p-10 text-center">
          <IconApps size={28} className="text-slate-300" aria-hidden="true" />
          <p className="text-sm text-slate-500">アプリケーションはまだありません。</p>
        </Card>
      ) : (
        <div className="grid gap-3">
          {applications.map((app) => (
            <ApplicationCard
              key={app.application_id}
              app={app}
              csrfToken={csrfToken}
              onChange={replace}
              onRemove={(id) =>
                setApplications((current) => current.filter((x) => x.application_id !== id))
              }
              onError={setError}
            />
          ))}
        </div>
      )}
    </AdminShell>
  )
}

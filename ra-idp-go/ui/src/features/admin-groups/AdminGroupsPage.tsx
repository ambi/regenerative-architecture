import {
  IconArrowLeft,
  IconDotsVertical,
  IconPencil,
  IconPlus,
  IconRefresh,
  IconTrash,
  IconUserMinus,
  IconUserPlus,
  IconUsersGroup,
  IconX,
} from '@tabler/icons-react'
import { type FormEvent, useEffect, useState } from 'react'
import {
  addAdminGroupMember,
  AuthenticationAPIError,
  createAdminGroup,
  deleteAdminGroup,
  getAdminGroup,
  listAdminGroups,
  listAdminUsers,
  removeAdminGroupMember,
  tenantURL,
  updateAdminGroup,
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
import type { AdminGroup, AdminGroupMember, AdminUser } from '../../types'

export function AdminGroupsPage({
  csrfToken,
  actorUsername,
  groups: initial,
}: {
  csrfToken: string
  actorUsername?: string
  groups: AdminGroup[]
}) {
  const [groups, setGroups] = useState(initial)
  const initialID = new URLSearchParams(window.location.search).get('group')
  const [selectedID, setSelectedID] = useState<string>(
    () => initial.find((g) => g.id === initialID)?.id ?? initial[0]?.id ?? '',
  )
  const [showCreate, setShowCreate] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  const selected = groups.find((g) => g.id === selectedID) ?? null

  async function refresh(preferredID = selectedID) {
    const next = await listAdminGroups()
    setGroups(next)
    setSelectedID(next.find((g) => g.id === preferredID)?.id ?? next[0]?.id ?? '')
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
          : 'グループ操作を完了できませんでした。',
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
      const created = await createAdminGroup(csrfToken, {
        name: String(data.get('name') ?? ''),
        description: optionalValue(data.get('description')),
        roles: parseRoles(String(data.get('roles') ?? '')),
      })
      form.reset()
      setShowCreate(false)
      await refresh(created.id)
    }, 'グループを作成しました。')
  }

  return (
    <AdminShell
      active="groups"
      actorUsername={actorUsername}
      title="グループ"
      description="複数のロールをまとめ、所属ユーザーに一括で付与します。"
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
            新規グループ
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
                <th className="px-4 py-3">グループ</th>
                <th className="px-4 py-3">ロール</th>
                <th className="px-4 py-3 text-right">メンバー</th>
              </tr>
            </thead>
            <tbody>
              {groups.map((group) => (
                <tr
                  key={group.id}
                  onClick={() => setSelectedID(group.id)}
                  className={`cursor-pointer border-t border-slate-100 hover:bg-slate-50 ${
                    selectedID === group.id ? 'bg-blue-50/60' : ''
                  }`}
                >
                  <td className="px-4 py-3">
                    <div className="font-semibold text-slate-900">{group.name}</div>
                    {group.description ? (
                      <div className="truncate text-xs text-slate-500">{group.description}</div>
                    ) : null}
                  </td>
                  <td className="px-4 py-3 text-xs text-slate-600">{group.roles.length} 個</td>
                  <td className="px-4 py-3 text-right text-xs text-slate-600">
                    {group.member_count}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {groups.length === 0 ? (
            <div className="flex min-h-40 flex-col items-center justify-center px-6 text-center text-sm text-slate-500">
              <IconUsersGroup size={24} className="text-slate-400" aria-hidden="true" />
              <p className="mt-3">グループはまだありません。</p>
            </div>
          ) : null}
        </Card>

        <GroupDetailCard
          group={selected}
          csrfToken={csrfToken}
          busy={busy}
          detailHref={
            selected ? tenantURL(`/admin/groups/${encodeURIComponent(selected.id)}`) : undefined
          }
          onChanged={(id) => run(() => refresh(id), 'グループを更新しました。')}
          onDeleted={() => run(() => refresh(), 'グループを削除しました。')}
        />
      </div>

      {showCreate ? (
        <div className="fixed inset-0 z-40 flex items-center justify-center bg-slate-900/40 px-4">
          <Card className="w-full max-w-md p-6">
            <div className="flex items-center justify-between">
              <h2 className="text-base font-semibold text-slate-900">新規グループ</h2>
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
                <Label htmlFor="group-name">グループ名</Label>
                <Input id="group-name" name="name" required placeholder="engineering" />
                <p className="text-xs text-slate-500">テナント内で一意の表示名。</p>
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="group-description">説明 (任意)</Label>
                <Input
                  id="group-description"
                  name="description"
                  placeholder="エンジニアリングチーム"
                />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="group-roles">ロール</Label>
                <Input id="group-roles" name="roles" placeholder="catalog:read, invoice:read" />
                <p className="text-xs text-slate-500">
                  カンマ区切り。所属ユーザーに一斉付与されます。
                </p>
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
                  作成
                </Button>
              </div>
            </form>
          </Card>
        </div>
      ) : null}
    </AdminShell>
  )
}

// AdminGroupDetailPage はグループの編集・メンバー管理を扱う専用詳細画面 (wi-39)。
export function AdminGroupDetailPage({
  csrfToken,
  actorUsername,
  group: initialGroup,
}: {
  csrfToken: string
  actorUsername?: string
  group: AdminGroup
}) {
  const [group, setGroup] = useState(initialGroup)
  const [editing, setEditing] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function reload(id: string) {
    try {
      const { group: next } = await getAdminGroup(id)
      setGroup(next)
      setError('')
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : 'グループの再取得に失敗しました。',
      )
    }
  }

  async function handleDelete() {
    setBusy(true)
    setError('')
    try {
      await deleteAdminGroup(csrfToken, group.id)
      window.location.assign(tenantURL('/admin/groups'))
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : 'グループを削除できませんでした。',
      )
      setBusy(false)
    }
  }

  return (
    <>
      <AdminShell
        active="groups"
        actorUsername={actorUsername}
        title={group.name}
        description={group.description || group.id}
        actions={
          <div className="flex items-center gap-2">
            <a
              href={tenantURL('/admin/groups')}
              className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 transition-colors hover:bg-slate-50"
            >
              <IconArrowLeft size={16} aria-hidden="true" />
              グループ一覧
            </a>
            <Button type="button" disabled={busy} onClick={() => setEditing(true)}>
              <IconPencil size={16} aria-hidden="true" />
              編集
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  type="button"
                  variant="outline"
                  className="size-9 px-0"
                  aria-label="グループ操作"
                  disabled={busy}
                >
                  <IconDotsVertical size={18} aria-hidden="true" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem className="text-red-700" onSelect={() => setConfirmDelete(true)}>
                  <IconTrash size={17} aria-hidden="true" />
                  グループを削除
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        }
      >
        {error ? <Alert variant="destructive">{error}</Alert> : null}
        {confirmDelete ? (
          <Alert
            variant="destructive"
            className="flex flex-wrap items-center justify-between gap-2"
          >
            <span>このグループを削除しますか？所属ユーザーからロールが外れます。</span>
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
          <GroupDetailCard
            group={group}
            csrfToken={csrfToken}
            busy={busy}
            showActions={false}
            onChanged={(id) => void reload(id)}
            onDeleted={() => window.location.assign(tenantURL('/admin/groups'))}
          />
        </div>
      </AdminShell>
      {editing ? (
        <GroupEditorDialog
          group={group}
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

function GroupDetailCard({
  group,
  csrfToken,
  busy,
  detailHref,
  showActions = true,
  onChanged,
  onDeleted,
}: {
  group: AdminGroup | null
  csrfToken: string
  busy: boolean
  detailHref?: string
  showActions?: boolean
  onChanged: (id: string) => void
  onDeleted: () => void
}) {
  const [members, setMembers] = useState<AdminGroupMember[]>([])
  const [allUsers, setAllUsers] = useState<AdminUser[]>([])
  const [addSub, setAddSub] = useState('')
  const [localBusy, setLocalBusy] = useState(false)
  const [localError, setLocalError] = useState('')
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [editing, setEditing] = useState(false)

  useEffect(() => {
    setConfirmDelete(false)
    setEditing(false)
    setLocalError('')
    if (!group) {
      setMembers([])
      return
    }
    let cancelled = false
    void Promise.all([getAdminGroup(group.id), listAdminUsers()]).then(([detail, users]) => {
      if (cancelled) return
      setMembers(detail.members)
      setAllUsers(users)
    })
    return () => {
      cancelled = true
    }
  }, [group])

  if (!group) {
    return (
      <Card className="p-5">
        <p className="text-sm text-slate-500">グループを選択してください。</p>
      </Card>
    )
  }
  const activeGroup = group

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

  async function reloadMembers() {
    const detail = await getAdminGroup(activeGroup.id)
    setMembers(detail.members)
  }

  const memberSubs = new Set(members.map((m) => m.user_sub))
  const addableUsers = allUsers.filter((u) => !memberSubs.has(u.sub))

  return (
    <>
      <Card className="overflow-hidden">
        <div className="border-b border-slate-200 bg-white p-5">
          <div className="flex items-start gap-3">
            <span className="flex size-11 shrink-0 items-center justify-center rounded-lg bg-slate-100 text-slate-700">
              <IconUsersGroup size={22} aria-hidden="true" />
            </span>
            <div className="min-w-0 flex-1">
              <h2 className="truncate text-lg font-semibold text-slate-950">{group.name}</h2>
              <p className="mt-0.5 truncate font-mono text-sm text-slate-500">{group.id}</p>
            </div>
          </div>

          {showActions ? (
            <div className="mt-4">
              <AdminPaneActions
                detailHref={detailHref}
                busy={busy || localBusy}
                onEdit={() => setEditing(true)}
                menu={
                  <DropdownMenuItem
                    className="text-red-700"
                    onSelect={() => setConfirmDelete(true)}
                  >
                    <IconTrash size={17} aria-hidden="true" />
                    グループを削除
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
            <span>このグループを削除しますか？所属ユーザーからロールが外れます。</span>
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
                    await deleteAdminGroup(csrfToken, activeGroup.id)
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
          <div>
            <dt className="text-xs font-bold uppercase tracking-normal text-slate-400">説明</dt>
            <dd className="mt-1 text-sm text-slate-700">{group.description || '—'}</dd>
          </div>
          <div>
            <dt className="text-xs font-bold uppercase tracking-normal text-slate-400">ロール</dt>
            <dd className="mt-1 flex flex-wrap gap-1.5">
              {group.roles.length > 0 ? (
                group.roles.map((role) => (
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
            メンバー ({members.length})
          </h3>
          <ul className="mt-3 grid gap-2">
            {members.map((member) => (
              <li
                key={member.user_sub}
                className="flex items-center justify-between rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm"
              >
                <a
                  className="font-medium text-blue-700 hover:underline"
                  href={tenantURL(
                    `/admin/users?role=${encodeURIComponent(member.preferred_username)}`,
                  )}
                >
                  {member.preferred_username}
                </a>
                <Button
                  variant="ghost"
                  className="text-rose-700 hover:bg-rose-50"
                  disabled={localBusy}
                  onClick={() =>
                    withLocal(async () => {
                      await removeAdminGroupMember(csrfToken, group.id, member.user_sub)
                      await reloadMembers()
                    })
                  }
                >
                  <IconUserMinus size={14} aria-hidden="true" />
                  除外
                </Button>
              </li>
            ))}
            {members.length === 0 ? (
              <li className="text-xs text-slate-400">メンバーはいません。</li>
            ) : null}
          </ul>

          <div className="mt-3 flex items-center gap-2">
            <select
              value={addSub}
              onChange={(e) => setAddSub(e.target.value)}
              className="h-9 flex-1 rounded-md border border-slate-300 bg-white px-2 text-sm"
              aria-label="追加するユーザー"
            >
              <option value="">ユーザーを選択…</option>
              {addableUsers.map((user) => (
                <option key={user.sub} value={user.sub}>
                  {user.preferred_username}
                </option>
              ))}
            </select>
            <Button
              disabled={localBusy || !addSub}
              onClick={() =>
                withLocal(async () => {
                  await addAdminGroupMember(csrfToken, group.id, addSub)
                  setAddSub('')
                  await reloadMembers()
                })
              }
            >
              <IconUserPlus size={14} aria-hidden="true" />
              追加
            </Button>
          </div>
        </section>
      </Card>
      {editing ? (
        <GroupEditorDialog
          group={activeGroup}
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

function GroupEditorDialog({
  group,
  csrfToken,
  onClose,
  onSaved,
}: {
  group: AdminGroup
  csrfToken: string
  onClose: () => void
  onSaved: (id: string) => void
}) {
  const [name, setName] = useState(group.name)
  const [description, setDescription] = useState(group.description ?? '')
  const [roles, setRoles] = useState(group.roles.join(', '))
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const trimmedName = name.trim()
  const nextRoles = parseRoles(roles)
  const nameInvalid = trimmedName === ''
  const changed =
    trimmedName !== group.name ||
    description.trim() !== (group.description ?? '') ||
    nextRoles.join(',') !== group.roles.join(',')

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (nameInvalid || !changed) return
    setSaving(true)
    setError('')
    try {
      await updateAdminGroup(csrfToken, group.id, {
        name: trimmedName !== group.name ? trimmedName : undefined,
        description:
          description.trim() !== (group.description ?? '') ? description.trim() : undefined,
        roles: nextRoles.join(',') !== group.roles.join(',') ? nextRoles : undefined,
      })
      onSaved(group.id)
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : 'グループを更新できませんでした。',
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
      aria-labelledby="group-editor-title"
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
            <p className="text-xs font-bold uppercase tracking-normal text-blue-700">グループ</p>
            <h2 id="group-editor-title" className="mt-1 text-xl font-semibold">
              グループを編集
            </h2>
            <p className="mt-1 text-sm text-slate-500">{group.name}</p>
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
                  <Label htmlFor="group-editor-name">グループ名</Label>
                  <Input
                    id="group-editor-name"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    required
                    aria-invalid={nameInvalid}
                  />
                </div>
                <div className="grid gap-1.5">
                  <Label htmlFor="group-editor-description">説明</Label>
                  <Input
                    id="group-editor-description"
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                  />
                </div>
              </section>
              <section className="grid gap-3 border-t border-slate-200 pt-5">
                <h3 className="text-xs font-bold uppercase tracking-normal text-slate-400">
                  ロール
                </h3>
                <div className="grid gap-1.5">
                  <Label htmlFor="group-editor-roles">ロール</Label>
                  <Input
                    id="group-editor-roles"
                    value={roles}
                    onChange={(e) => setRoles(e.target.value)}
                    placeholder="catalog:read, invoice:read"
                  />
                  <p className="text-xs text-slate-500">
                    カンマ区切り。所属ユーザーに一斉付与されます。
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

import {
  IconAdjustments,
  IconAlertTriangle,
  IconBan,
  IconCheck,
  IconCircleCheck,
  IconClock,
  IconDotsVertical,
  IconKey,
  IconMail,
  IconPencil,
  IconRefresh,
  IconSearch,
  IconShield,
  IconShieldCheck,
  IconTrash,
  IconUser,
  IconUserPlus,
  IconUsers,
  IconX,
} from '@tabler/icons-react'
import { type FormEvent, useMemo, useState } from 'react'
import {
  AuthenticationAPIError,
  createAdminUser,
  deleteAdminUser,
  listAdminUsers,
  setAdminUserDisabled,
  type UpdateAdminUserInput,
  updateAdminUser,
} from '../api'
import { AdminShell } from '../components/AdminShell'
import { Alert } from '../components/ui/alert'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '../components/ui/dropdown-menu'
import { cn } from '../lib/utils'
import type { AdminUser, AdminUsersPage as AdminUsersPageData } from '../types'

type StatusFilter = 'all' | 'active' | 'disabled'

export function AdminUsersPage({
  csrfToken,
  actorUsername,
  users: initialUsers,
}: AdminUsersPageData) {
  const [users, setUsers] = useState(initialUsers)
  const [selectedSub, setSelectedSub] = useState(initialUsers[0]?.sub ?? '')
  const [query, setQuery] = useState(
    () => new URLSearchParams(window.location.search).get('role') ?? '',
  )
  const [status, setStatus] = useState<StatusFilter>('all')
  const [showCreate, setShowCreate] = useState(false)
  const [showUserEditor, setShowUserEditor] = useState(false)
  const [showDelete, setShowDelete] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  const selected = users.find((user) => user.sub === selectedSub)
  const activeCount = users.filter((user) => !user.disabled_at).length
  const adminCount = users.filter((user) => user.roles.includes('admin')).length
  const mfaCount = users.filter((user) => user.mfa_enrolled).length
  const filteredUsers = useMemo(() => {
    const needle = query.trim().toLowerCase()
    return users.filter((user) => {
      const matchesStatus =
        status === 'all' ||
        (status === 'active' && !user.disabled_at) ||
        (status === 'disabled' && Boolean(user.disabled_at))
      const matchesQuery =
        !needle ||
        [user.preferred_username, user.name, user.email, user.sub, ...user.roles]
          .filter(Boolean)
          .some((value) => value?.toLowerCase().includes(needle))
      return matchesStatus && matchesQuery
    })
  }, [query, status, users])

  async function refresh(preferredSub = selectedSub) {
    const next = await listAdminUsers()
    setUsers(next)
    const nextSelected = next.find((user) => user.sub === preferredSub) ?? next[0]
    setSelectedSub(nextSelected?.sub ?? '')
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
          : '管理操作を完了できませんでした。',
      )
    } finally {
      setBusy(false)
    }
  }

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = event.currentTarget
    const data = new FormData(form)
    await run(async () => {
      const created = await createAdminUser(csrfToken, {
        preferred_username: String(data.get('preferred_username') ?? ''),
        password: String(data.get('password') ?? ''),
        name: optionalValue(data.get('name')),
        email: optionalValue(data.get('email')),
        email_verified: data.get('email_verified') === 'on',
        roles: parseRoles(String(data.get('roles') ?? '')),
      })
      form.reset()
      setShowCreate(false)
      await refresh(created.sub)
    }, 'ユーザーを作成しました。')
  }

  async function handleUpdate(input: UpdateAdminUserInput) {
    if (!selected) return
    await run(async () => {
      await updateAdminUser(csrfToken, selected.sub, input)
      setShowUserEditor(false)
      await refresh(selected.sub)
    }, 'ユーザーを更新しました。')
  }

  async function handleDisabled(user: AdminUser) {
    const disabled = !user.disabled_at
    await run(
      async () => {
        await setAdminUserDisabled(csrfToken, user.sub, disabled)
        await refresh(user.sub)
      },
      disabled ? 'ユーザーを無効化しました。' : 'ユーザーを再有効化しました。',
    )
  }

  async function handleDelete(user: AdminUser, reason: string) {
    await run(async () => {
      await deleteAdminUser(csrfToken, user.sub, reason)
      setShowDelete(false)
      await refresh()
    }, 'ユーザーを削除しました。')
  }

  function selectUser(user: AdminUser) {
    setSelectedSub(user.sub)
  }

  return (
    <>
      <AdminShell
        active="users"
        actorUsername={actorUsername}
        title="ユーザー"
        description="組織のID、アクセスロール、アカウント状態を一元管理します。"
        actions={
                <Button onClick={() => setShowCreate(true)}>
                  <IconUserPlus size={17} aria-hidden="true" />
                  ユーザーを追加
                </Button>
        }
      >
            <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4" aria-label="ユーザー概要">
              <Metric label="総ユーザー" value={users.length} icon={IconUsers} tone="blue" />
              <Metric
                label="有効なアカウント"
                value={activeCount}
                icon={IconCircleCheck}
                tone="green"
              />
              <Metric label="管理者" value={adminCount} icon={IconShield} tone="violet" />
              <Metric label="MFA 登録済み" value={mfaCount} icon={IconKey} tone="amber" />
            </section>

            {error && <Alert>{error}</Alert>}
            {notice && (
              <div
                role="status"
                className="flex items-center gap-2 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-900"
              >
                <IconCheck size={18} aria-hidden="true" />
                {notice}
              </div>
            )}

            <Card className="overflow-hidden shadow-[0_1px_2px_rgb(15_23_42/4%)]">
              <div className="flex flex-col gap-3 border-b border-slate-200 p-4 lg:flex-row lg:items-center lg:justify-between">
                <div className="relative w-full max-w-xl">
                  <IconSearch
                    size={18}
                    className="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400"
                    aria-hidden="true"
                  />
                  <Input
                    value={query}
                    onChange={(event) => setQuery(event.target.value)}
                    className="h-10 pl-10"
                    placeholder="名前、メール、ID、ロールで検索"
                    aria-label="ユーザーを検索"
                  />
                </div>
                <div className="flex items-center gap-2">
                  <IconAdjustments size={17} className="text-slate-400" aria-hidden="true" />
                  <div className="flex rounded-lg border border-slate-200 bg-slate-50 p-0.5">
                    {(['all', 'active', 'disabled'] as const).map((value) => (
                      <button
                        key={value}
                        type="button"
                        onClick={() => setStatus(value)}
                        className={cn(
                          'rounded-md px-3 py-1.5 text-xs font-semibold transition-colors',
                          status === value
                            ? 'bg-white text-slate-900 shadow-sm'
                            : 'text-slate-500 hover:text-slate-800',
                        )}
                      >
                        {{ all: 'すべて', active: '有効', disabled: '無効' }[value]}
                      </button>
                    ))}
                  </div>
                  <Button
                    variant="outline"
                    className="size-9 px-0"
                    disabled={busy}
                    aria-label="一覧を再読み込み"
                    onClick={() => void run(() => refresh(), '一覧を更新しました。')}
                  >
                    <IconRefresh size={16} aria-hidden="true" />
                  </Button>
                </div>
              </div>

              <div className="grid min-h-[520px] xl:grid-cols-[minmax(0,1.55fr)_400px]">
                <div className="min-w-0 overflow-x-auto">
                  <table className="w-full min-w-[760px] text-left text-sm">
                    <thead className="border-b border-slate-200 bg-slate-50/80 text-[0.68rem] font-bold uppercase tracking-[0.08em] text-slate-500">
                      <tr>
                        <th className="px-5 py-3.5">ユーザー</th>
                        <th className="px-5 py-3.5">アクセス</th>
                        <th className="px-5 py-3.5">セキュリティ</th>
                        <th className="px-5 py-3.5">状態</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100">
                      {filteredUsers.map((user) => (
                        <tr
                          key={user.sub}
                          onClick={() => selectUser(user)}
                          className={cn(
                            'cursor-pointer bg-white transition-colors hover:bg-slate-50',
                            selectedSub === user.sub && 'bg-blue-50/60 hover:bg-blue-50/80',
                          )}
                        >
                          <td className="px-5 py-4">
                            <div className="flex items-center gap-3">
                              <UserAvatar user={user} />
                              <div className="min-w-0">
                                <p className="truncate font-semibold text-slate-900">
                                  {user.name || user.preferred_username}
                                </p>
                                <p className="truncate text-xs text-slate-500">
                                  {user.email || `@${user.preferred_username}`}
                                </p>
                              </div>
                            </div>
                          </td>
                          <td className="px-5 py-4">
                            <RoleList roles={user.roles} />
                          </td>
                          <td className="px-5 py-4">
                            <div className="flex items-center gap-2 text-xs text-slate-600">
                              <span
                                className={cn(
                                  'flex size-6 items-center justify-center rounded-full',
                                  user.mfa_enrolled
                                    ? 'bg-emerald-50 text-emerald-700'
                                    : 'bg-slate-100 text-slate-400',
                                )}
                              >
                                <IconKey size={13} aria-hidden="true" />
                              </span>
                              {user.mfa_enrolled ? 'MFA' : 'Password'}
                            </div>
                          </td>
                          <td className="px-5 py-4">
                            <StatusBadge disabled={Boolean(user.disabled_at)} />
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                  {filteredUsers.length === 0 && (
                    <div className="flex min-h-64 flex-col items-center justify-center px-6 text-center">
                      <span className="flex size-12 items-center justify-center rounded-full bg-slate-100 text-slate-400">
                        <IconSearch size={22} aria-hidden="true" />
                      </span>
                      <p className="mt-4 font-semibold text-slate-800">ユーザーが見つかりません</p>
                      <p className="mt-1 text-sm text-slate-500">
                        検索語または状態フィルターを変更してください。
                      </p>
                    </div>
                  )}
                </div>

                <aside className="border-t border-slate-200 bg-slate-50/40 xl:border-l xl:border-t-0">
                  {selected ? (
                    <UserDetails
                      user={selected}
                      busy={busy}
                      onEdit={() => setShowUserEditor(true)}
                      onDisabled={() => void handleDisabled(selected)}
                      onDelete={() => setShowDelete(true)}
                    />
                  ) : (
                    <div className="flex h-full min-h-80 items-center justify-center p-8 text-center text-sm text-slate-500">
                      ユーザーを選択すると詳細が表示されます。
                    </div>
                  )}
                </aside>
              </div>
              <div className="flex items-center justify-between border-t border-slate-200 bg-slate-50/70 px-5 py-3 text-xs text-slate-500">
                <span>{filteredUsers.length} 件を表示</span>
                <span>最終更新: {formatDateTime(new Date().toISOString())}</span>
              </div>
            </Card>
      </AdminShell>

      {showCreate && (
        <CreateUserDialog
          busy={busy}
          onClose={() => setShowCreate(false)}
          onSubmit={handleCreate}
        />
      )}
      {showUserEditor && selected && (
        <UserEditorDialog
          user={selected}
          busy={busy}
          onSubmit={(input) => void handleUpdate(input)}
          onClose={() => setShowUserEditor(false)}
        />
      )}
      {showDelete && selected && (
        <DeleteUserDialog
          user={selected}
          busy={busy}
          onClose={() => setShowDelete(false)}
          onConfirm={(reason) => void handleDelete(selected, reason)}
        />
      )}
    </>
  )
}

function UserDetails({
  user,
  busy,
  onEdit,
  onDisabled,
  onDelete,
}: {
  user: AdminUser
  busy: boolean
  onEdit: () => void
  onDisabled: () => void
  onDelete: () => void
}) {
  return (
    <div className="flex h-full flex-col">
      <div className="border-b border-slate-200 bg-white p-5">
        <div className="flex items-start gap-3">
          <UserAvatar user={user} large />
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <h2 className="truncate text-lg font-semibold text-slate-950">
                {user.name || user.preferred_username}
              </h2>
              <StatusBadge disabled={Boolean(user.disabled_at)} compact />
            </div>
            <p className="mt-0.5 text-sm text-slate-500">@{user.preferred_username}</p>
          </div>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                type="button"
                variant="ghost"
                className="size-9 px-0"
                aria-label="ユーザー操作"
                disabled={busy}
              >
                <IconDotsVertical size={18} aria-hidden="true" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem
                className={user.disabled_at ? undefined : 'text-red-700'}
                onSelect={onDisabled}
              >
                {user.disabled_at ? (
                  <IconCheck size={17} aria-hidden="true" />
                ) : (
                  <IconBan size={17} aria-hidden="true" />
                )}
                {user.disabled_at ? 'アカウントを再有効化' : 'アカウントを無効化'}
              </DropdownMenuItem>
              <DropdownMenuSeparator className="my-1 h-px bg-slate-200" />
              <DropdownMenuItem className="text-red-700" onSelect={onDelete}>
                <IconTrash size={17} aria-hidden="true" />
                アカウントを削除
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      <div className="flex flex-1 flex-col gap-6 p-5">
        <section>
          <div className="flex items-center justify-between">
            <h3 className="text-xs font-bold uppercase tracking-[0.1em] text-slate-400">Profile</h3>
            <Button
              type="button"
              disabled={busy}
              onClick={onEdit}
            >
              <IconPencil size={16} aria-hidden="true" />
              編集
            </Button>
          </div>
          <dl className="mt-3 grid gap-3 text-sm">
            <DetailRow icon={IconMail} label="メール" value={user.email || '未設定'} />
            <DetailRow
              icon={IconShieldCheck}
              label="メール確認"
              value={user.email_verified ? '確認済み' : '未確認'}
            />
            <DetailRow
              icon={IconKey}
              label="認証方式"
              value={user.mfa_enrolled ? 'Password + MFA' : 'Password'}
            />
            <DetailRow icon={IconClock} label="作成日時" value={formatDateTime(user.created_at)} />
            <DetailRow icon={IconUser} label="Subject ID" value={user.sub} mono />
          </dl>
        </section>

        <section className="border-t border-slate-200 pt-5">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="text-sm font-semibold text-slate-900">ロール</h3>
              <p className="mt-0.5 text-xs text-slate-500">
                現在割り当てられているアクセス権限です。
              </p>
            </div>
            <IconShield size={18} className="text-slate-400" aria-hidden="true" />
          </div>
          <div className="mt-3 rounded-xl border border-slate-200 bg-white p-3">
            <RoleList roles={user.roles} />
          </div>
        </section>

      </div>
    </div>
  )
}

function RoleDiff({
  title,
  roles,
  tone,
}: {
  title: string
  roles: string[]
  tone: 'add' | 'remove'
}) {
  return (
    <div>
      <p className="text-xs font-semibold text-slate-500">{title}</p>
      <div className="mt-2 flex min-h-16 flex-wrap content-start gap-1.5 rounded-xl border border-slate-200 bg-white p-3">
        {roles.length === 0 ? (
          <span className="text-xs text-slate-400">なし</span>
        ) : (
          roles.map((role) => (
            <span
              key={role}
              className={cn(
                'rounded-md px-2 py-1 text-xs font-semibold',
                tone === 'add' ? 'bg-emerald-50 text-emerald-700' : 'bg-red-50 text-red-700',
              )}
            >
              {tone === 'add' ? '+' : '-'} {role}
            </span>
          ))
        )}
      </div>
    </div>
  )
}

function UserEditorDialog({
  user,
  busy,
  onSubmit,
  onClose,
}: {
  user: AdminUser
  busy: boolean
  onSubmit: (input: UpdateAdminUserInput) => void
  onClose: () => void
}) {
  const initialUsername = user.preferred_username
  const initialName = user.name ?? ''
  const initialEmail = user.email ?? ''
  const initialEmailVerified = user.email_verified

  const [username, setUsername] = useState(initialUsername)
  const [name, setName] = useState(initialName)
  const [email, setEmail] = useState(initialEmail)
  const [emailVerified, setEmailVerified] = useState(initialEmailVerified)
  const [emailVerifiedTouched, setEmailVerifiedTouched] = useState(false)
  const [roles, setRoles] = useState(user.roles.join(', '))
  const [confirming, setConfirming] = useState(false)

  const emailChanged = email !== initialEmail
  const effectiveEmailVerified = emailChanged && !emailVerifiedTouched ? false : emailVerified
  const trimmedUsername = username.trim()
  const usernameInvalid = trimmedUsername === ''
  const profileChanged =
    trimmedUsername !== initialUsername ||
    name !== initialName ||
    email !== initialEmail ||
    effectiveEmailVerified !== initialEmailVerified
  const nextRoles = parseRoles(roles)
  const addedRoles = nextRoles.filter((role) => !user.roles.includes(role))
  const removedRoles = user.roles.filter((role) => !nextRoles.includes(role))
  const rolesChanged = addedRoles.length > 0 || removedRoles.length > 0
  const changed = profileChanged || rolesChanged

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (usernameInvalid || !changed) return
    if (rolesChanged && !confirming) {
      setConfirming(true)
      return
    }
    const input: UpdateAdminUserInput = {}
    if (trimmedUsername !== initialUsername) input.preferred_username = trimmedUsername
    if (name !== initialName) input.name = name
    if (email !== initialEmail) input.email = email
    if (effectiveEmailVerified !== initialEmailVerified) {
      input.email_verified = effectiveEmailVerified
    }
    if (rolesChanged) input.roles = nextRoles
    onSubmit(input)
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/30 p-5 backdrop-blur-[2px]"
      role="dialog"
      aria-modal="true"
      aria-labelledby="user-editor-title"
    >
      <button
        type="button"
        className="absolute inset-0 cursor-default"
        aria-label="閉じる"
        onClick={onClose}
      />
      <Card className="relative w-full max-w-lg overflow-hidden shadow-2xl">
        <div className="flex items-start justify-between border-b border-slate-200 px-6 py-5">
          <div>
            <p className="text-xs font-bold uppercase tracking-[0.12em] text-blue-700">
              Profile and access
            </p>
            <h2 id="user-editor-title" className="mt-1 text-xl font-semibold">
              {confirming ? '変更内容を確認' : 'ユーザーを編集'}
            </h2>
            <p className="mt-1 text-sm text-slate-500">
              {user.name || user.preferred_username} (@{user.preferred_username})
            </p>
          </div>
          <Button variant="ghost" className="px-2.5" onClick={onClose} aria-label="閉じる">
            <IconX size={18} aria-hidden="true" />
          </Button>
        </div>

        <form onSubmit={handleSubmit}>
          {confirming ? (
            <div className="p-6">
              <div className="rounded-xl border border-amber-200 bg-amber-50 p-4">
                <div className="flex gap-3">
                  <IconShield
                    size={19}
                    className="mt-0.5 shrink-0 text-amber-700"
                    aria-hidden="true"
                  />
                  <div>
                    <p className="text-sm font-semibold text-amber-950">ロール変更を含む更新です</p>
                    <p className="mt-1 text-xs leading-5 text-amber-800">
                      管理者ロールの追加・削除は管理コンソールへのアクセス権に影響します。
                    </p>
                  </div>
                </div>
              </div>
              <div className="mt-5 grid gap-4 sm:grid-cols-2">
                <RoleDiff title="追加されるロール" roles={addedRoles} tone="add" />
                <RoleDiff title="削除されるロール" roles={removedRoles} tone="remove" />
              </div>
              {profileChanged && (
                <p className="mt-4 text-xs leading-5 text-slate-500">
                  プロフィールの変更も同じ更新リクエストで保存されます。
                </p>
              )}
            </div>
          ) : (
            <div className="grid gap-6 p-6">
              <section className="grid gap-4">
                <h3 className="text-xs font-bold uppercase tracking-[0.1em] text-slate-400">
                  Profile
                </h3>
                <div className="grid gap-2">
                  <Label htmlFor="user-editor-username">ユーザー名</Label>
                  <Input
                    id="user-editor-username"
                    value={username}
                    onChange={(event) => setUsername(event.target.value)}
                    autoFocus
                    required
                    aria-invalid={usernameInvalid}
                  />
                  <p className="text-xs leading-5 text-slate-500">
                    login 時に使われる識別子です。空にはできません。
                  </p>
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="user-editor-name">表示名</Label>
                  <Input
                    id="user-editor-name"
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="user-editor-email">メールアドレス</Label>
                  <Input
                    id="user-editor-email"
                    type="email"
                    value={email}
                    onChange={(event) => {
                      setEmail(event.target.value)
                      setEmailVerifiedTouched(false)
                    }}
                  />
                  {emailChanged && (
                    <p className="text-xs leading-5 text-amber-700">
                      メールを変更したため、確認済みフラグを既定で解除しています。
                    </p>
                  )}
                </div>
                <label className="flex items-start gap-3 rounded-xl border border-slate-200 bg-slate-50 p-4 text-sm text-slate-700">
                  <input
                    type="checkbox"
                    className="mt-0.5 size-4 rounded border-slate-300"
                    checked={effectiveEmailVerified}
                    onChange={(event) => {
                      setEmailVerified(event.target.checked)
                      setEmailVerifiedTouched(true)
                    }}
                  />
                  <span>
                    <span className="block font-semibold text-slate-900">
                      メール確認済みとして保存
                    </span>
                    <span className="mt-0.5 block text-xs leading-5 text-slate-500">
                      組織側でメールアドレスの所有確認が完了している場合のみ選択します。
                    </span>
                  </span>
                </label>
              </section>
              <section className="grid gap-2 border-t border-slate-200 pt-5">
                <h3 className="text-xs font-bold uppercase tracking-[0.1em] text-slate-400">
                  Roles
                </h3>
                <Label htmlFor="user-editor-roles">ロール</Label>
                <Input
                  id="user-editor-roles"
                  value={roles}
                  onChange={(event) => setRoles(event.target.value)}
                  placeholder="admin, support"
                />
                <p className="text-xs leading-5 text-slate-500">
                  複数指定する場合はカンマで区切ります。変更時は保存前に差分を確認します。
                </p>
              </section>
            </div>
          )}
          <div className="flex justify-end gap-2 border-t border-slate-200 bg-slate-50 px-6 py-4">
            <Button
              type="button"
              variant="outline"
              onClick={confirming ? () => setConfirming(false) : onClose}
            >
              {confirming ? '戻る' : 'キャンセル'}
            </Button>
            <Button type="submit" disabled={busy || usernameInvalid || !changed}>
              {confirming ? '変更を確定' : rolesChanged ? '変更内容を確認' : '保存'}
            </Button>
          </div>
        </form>
      </Card>
    </div>
  )
}

function DeleteUserDialog({
  user,
  busy,
  onClose,
  onConfirm,
}: {
  user: AdminUser
  busy: boolean
  onClose: () => void
  onConfirm: (reason: string) => void
}) {
  const [confirmName, setConfirmName] = useState('')
  const [reason, setReason] = useState('')
  const canConfirm = confirmName === user.preferred_username

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!canConfirm) return
    onConfirm(reason)
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/35 p-5 backdrop-blur-[2px]"
      role="dialog"
      aria-modal="true"
      aria-labelledby="delete-user-title"
    >
      <button
        type="button"
        className="absolute inset-0 cursor-default"
        aria-label="閉じる"
        onClick={onClose}
      />
      <Card className="relative w-full max-w-lg overflow-hidden shadow-2xl">
        <div className="flex items-start justify-between border-b border-slate-200 px-6 py-5">
          <div className="flex gap-3">
            <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-red-50 text-red-700">
              <IconAlertTriangle size={18} aria-hidden="true" />
            </span>
            <div>
              <p className="text-xs font-bold uppercase tracking-[0.12em] text-red-700">
                Irreversible action
              </p>
              <h2 id="delete-user-title" className="mt-1 text-xl font-semibold">
                ユーザーを削除
              </h2>
              <p className="mt-1 text-sm text-slate-500">
                {user.name || user.preferred_username} (@{user.preferred_username})
              </p>
            </div>
          </div>
          <Button variant="ghost" className="px-2.5" onClick={onClose} aria-label="閉じる">
            <IconX size={18} aria-hidden="true" />
          </Button>
        </div>

        <form onSubmit={handleSubmit}>
          <div className="grid gap-5 p-6">
            <div className="rounded-xl border border-red-200 bg-red-50 p-4 text-xs leading-5 text-red-900">
              <p className="font-semibold">同時に消えるもの</p>
              <ul className="mt-1.5 list-disc pl-5">
                <li>付与済みの同意 (Consent)</li>
                <li>リフレッシュトークンとアクティブなセッション</li>
                <li>MFA factor とパスワード履歴</li>
                <li>進行中の device authorization</li>
              </ul>
              <p className="mt-2">
                ユーザーの <code>sub</code> は監査ログのために残りますが、
                プロフィール情報は匿名化されます。
              </p>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="delete-user-confirm">
                確認のためユーザー名{' '}
                <span className="font-mono text-slate-700">{user.preferred_username}</span>{' '}
                を入力してください
              </Label>
              <Input
                id="delete-user-confirm"
                value={confirmName}
                onChange={(event) => setConfirmName(event.target.value)}
                autoFocus
                autoComplete="off"
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="delete-user-reason">削除理由 (任意)</Label>
              <Input
                id="delete-user-reason"
                value={reason}
                onChange={(event) => setReason(event.target.value)}
                placeholder="例: 退職処理 / 本人申請 (GDPR Art.17)"
              />
              <p className="text-xs leading-5 text-slate-500">
                監査イベントに同梱されます。空欄でも削除は実行できます。
              </p>
            </div>
          </div>

          <div className="flex justify-end gap-2 border-t border-slate-200 bg-slate-50 px-6 py-4">
            <Button type="button" variant="outline" onClick={onClose} disabled={busy}>
              キャンセル
            </Button>
            <Button type="submit" variant="destructive" disabled={busy || !canConfirm}>
              <IconTrash size={16} aria-hidden="true" />
              削除を確定
            </Button>
          </div>
        </form>
      </Card>
    </div>
  )
}

function CreateUserDialog({
  busy,
  onClose,
  onSubmit,
}: {
  busy: boolean
  onClose: () => void
  onSubmit: (event: FormEvent<HTMLFormElement>) => void
}) {
  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-end bg-slate-950/25 backdrop-blur-[2px]"
      role="dialog"
      aria-modal="true"
      aria-labelledby="create-user-title"
    >
      <button
        type="button"
        className="absolute inset-0 cursor-default"
        aria-label="閉じる"
        onClick={onClose}
      />
      <div className="relative flex h-full w-full max-w-md flex-col bg-white shadow-2xl">
        <div className="flex items-start justify-between border-b border-slate-200 px-6 py-5">
          <div>
            <p className="text-xs font-bold uppercase tracking-[0.12em] text-blue-700">Directory</p>
            <h2 id="create-user-title" className="mt-1 text-xl font-semibold">
              ユーザーを追加
            </h2>
            <p className="mt-1 text-sm text-slate-500">新しい組織アカウントを作成します。</p>
          </div>
          <Button variant="ghost" className="px-2.5" onClick={onClose} aria-label="閉じる">
            <IconX size={18} aria-hidden="true" />
          </Button>
        </div>
        <form className="flex flex-1 flex-col overflow-y-auto" onSubmit={onSubmit}>
          <div className="flex flex-col gap-5 p-6">
            <div className="grid grid-cols-2 gap-4">
              <Field id="preferred_username" label="ユーザー名" required />
              <Field id="name" label="表示名" />
            </div>
            <Field id="email" label="メールアドレス" type="email" />
            <Field
              id="password"
              label="初期パスワード"
              type="password"
              required
              minLength={12}
              description="12文字以上。一般的なパスワードやユーザー名を含む値は使用できません。"
            />
            <Field
              id="roles"
              label="初期ロール"
              placeholder="support, admin"
              description="権限を付与しない場合は空欄にします。"
            />
            <label className="flex items-start gap-3 rounded-xl border border-slate-200 bg-slate-50 p-4 text-sm text-slate-700">
              <input
                name="email_verified"
                type="checkbox"
                className="mt-0.5 size-4 rounded border-slate-300"
              />
              <span>
                <span className="block font-semibold text-slate-900">メール確認済みとして作成</span>
                <span className="mt-0.5 block text-xs leading-5 text-slate-500">
                  組織側でメールアドレスの所有確認が完了している場合のみ選択します。
                </span>
              </span>
            </label>
          </div>
          <div className="mt-auto flex justify-end gap-2 border-t border-slate-200 bg-slate-50 px-6 py-4">
            <Button type="button" variant="outline" onClick={onClose}>
              キャンセル
            </Button>
            <Button type="submit" disabled={busy}>
              <IconUserPlus size={16} aria-hidden="true" />
              作成
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}

function Metric({
  label,
  value,
  icon: Icon,
  tone,
}: {
  label: string
  value: number
  icon: typeof IconUsers
  tone: 'blue' | 'green' | 'violet' | 'amber'
}) {
  const tones = {
    blue: 'bg-blue-50 text-blue-700',
    green: 'bg-emerald-50 text-emerald-700',
    violet: 'bg-violet-50 text-violet-700',
    amber: 'bg-amber-50 text-amber-700',
  }
  return (
    <Card className="flex items-center gap-4 p-4">
      <span className={cn('flex size-10 items-center justify-center rounded-xl', tones[tone])}>
        <Icon size={20} stroke={1.8} aria-hidden="true" />
      </span>
      <div>
        <p className="text-2xl font-semibold tracking-tight text-slate-950">{value}</p>
        <p className="text-xs font-medium text-slate-500">{label}</p>
      </div>
    </Card>
  )
}

function UserAvatar({ user, large = false }: { user: AdminUser; large?: boolean }) {
  const label = (user.name || user.preferred_username).slice(0, 2).toUpperCase()
  return (
    <span
      className={cn(
        'flex shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-blue-100 to-indigo-100 font-bold text-blue-800 ring-1 ring-inset ring-blue-200/70',
        large ? 'size-11 text-sm' : 'size-9 text-xs',
      )}
    >
      {label}
    </span>
  )
}

function RoleList({ roles }: { roles: string[] }) {
  if (roles.length === 0) return <span className="text-xs text-slate-400">権限なし</span>
  return (
    <div className="flex flex-wrap gap-1.5">
      {roles.slice(0, 2).map((role) => (
        <span
          key={role}
          className="rounded-md border border-slate-200 bg-white px-2 py-1 text-[0.68rem] font-semibold text-slate-700"
        >
          {role}
        </span>
      ))}
      {roles.length > 2 && (
        <span className="rounded-md bg-slate-100 px-2 py-1 text-[0.68rem] font-semibold text-slate-500">
          +{roles.length - 2}
        </span>
      )}
    </div>
  )
}

function StatusBadge({ disabled, compact = false }: { disabled: boolean; compact?: boolean }) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full font-semibold',
        compact ? 'px-2 py-0.5 text-[0.65rem]' : 'px-2.5 py-1 text-xs',
        disabled ? 'bg-red-50 text-red-700' : 'bg-emerald-50 text-emerald-700',
      )}
    >
      <span className={cn('size-1.5 rounded-full', disabled ? 'bg-red-500' : 'bg-emerald-500')} />
      {disabled ? '無効' : '有効'}
    </span>
  )
}

function DetailRow({
  icon: Icon,
  label,
  value,
  mono = false,
}: {
  icon: typeof IconUser
  label: string
  value: string
  mono?: boolean
}) {
  return (
    <div className="grid grid-cols-[24px_90px_minmax(0,1fr)] items-start gap-2">
      <Icon size={16} className="mt-0.5 text-slate-400" aria-hidden="true" />
      <dt className="text-slate-500">{label}</dt>
      <dd className={cn('min-w-0 break-all text-slate-800', mono && 'font-mono text-xs')}>
        {value}
      </dd>
    </div>
  )
}

type FieldProps = {
  id: string
  label: string
  type?: string
  placeholder?: string
  required?: boolean
  minLength?: number
  description?: string
}

function Field({ id, label, type = 'text', description, ...props }: FieldProps) {
  return (
    <div className="grid gap-2">
      <Label htmlFor={id}>{label}</Label>
      <Input id={id} name={id} type={type} {...props} />
      {description && <p className="text-xs leading-5 text-slate-500">{description}</p>}
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

function formatDateTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('ja-JP', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

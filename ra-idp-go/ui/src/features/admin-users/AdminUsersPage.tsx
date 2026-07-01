import {
  IconAdjustments,
  IconAlertTriangle,
  IconArrowLeft,
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
  IconUsersGroup,
  IconX,
} from '@tabler/icons-react'
import { type FormEvent, useCallback, useEffect, useMemo, useState } from 'react'
import {
  addAdminGroupMember,
  AuthenticationAPIError,
  clearAdminUserRequiredAction,
  createAdminUser,
  deleteAdminUser,
  getAdminUser,
  getAdminUserGroups,
  listAdminGroups,
  listAdminUsers,
  restoreAdminUser,
  setAdminUserDisabled,
  setAdminUserRequiredAction,
  tenantURL,
  type UpdateAdminUserInput,
  updateAdminUser,
} from '../../api'
import { AdminPaneActions } from '../../components/AdminPaneActions'
import { AdminShell } from '../../components/AdminShell'
import { Alert } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import { Input } from '../../components/ui/input'
import { Label } from '../../components/ui/label'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '../../components/ui/dropdown-menu'
import { attributeGroupKey, attributeGroupTitle, attributeLabel, cn } from '../../lib/utils'
import {
  type AdminGroup,
  type AdminUser,
  type AdminUserGroups,
  type AttributeValue,
  REQUIRED_ACTIONS,
  requiredActionLabel,
  type TenantUserAttributeSchema,
  type UserAttributeDef,
} from '../../types'
import {
  daysUntil,
  DetailRow,
  Field,
  formatDateTime,
  Metric,
  optionalValue,
  parseRoles,
  RoleList,
  StatusBadge,
  UserAvatar,
  userLifecycleStatus,
} from './AdminUsersPrimitives'

type StatusFilter = 'all' | 'active' | 'disabled' | 'pending_deletion'

export function AdminUsersPage({
  csrfToken,
  actorUsername,
  users: initialUsers,
  attributeDefs,
}: {
  csrfToken: string
  actorUsername?: string
  users: AdminUser[]
  attributeDefs: UserAttributeDef[]
}) {
  const [users, setUsers] = useState(initialUsers)
  const [selectedSub, setSelectedSub] = useState(initialUsers[0]?.sub ?? '')
  const [query, setQuery] = useState(
    () => new URLSearchParams(window.location.search).get('role') ?? '',
  )
  const [status, setStatus] = useState<StatusFilter>('all')
  const [showCreate, setShowCreate] = useState(false)
  const [showUserEditor, setShowUserEditor] = useState(false)
  const [showDelete, setShowDelete] = useState(false)
  const [showPurge, setShowPurge] = useState(false)
  const [showDisable, setShowDisable] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  const selected = users.find((user) => user.sub === selectedSub)
  const activeCount = users.filter((user) => userLifecycleStatus(user) === 'active').length
  const adminCount = users.filter((user) => user.roles.includes('admin')).length
  const mfaCount = users.filter((user) => user.mfa_enrolled).length
  const filteredUsers = useMemo(() => {
    const needle = query.trim().toLowerCase()
    return users.filter((user) => {
      const matchesStatus = status === 'all' || userLifecycleStatus(user) === status
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
        setShowDisable(false)
        await refresh(user.sub)
      },
      disabled ? 'ユーザーを無効化しました。' : 'ユーザーを再有効化しました。',
    )
  }

  // 無効化は破壊的なので確認ダイアログを挟む。再有効化はアクセス回復のみで
  // 誤操作リスクが低いため即時実行する (片側非対称)。
  function requestDisable(user: AdminUser) {
    if (user.disabled_at) {
      void handleDisabled(user)
    } else {
      setShowDisable(true)
    }
  }

  async function handleRequiredAction(user: AdminUser, action: string, present: boolean) {
    await run(
      async () => {
        if (present) {
          await clearAdminUserRequiredAction(csrfToken, user.sub, action)
        } else {
          await setAdminUserRequiredAction(csrfToken, user.sub, action)
        }
        await refresh(user.sub)
      },
      present ? '強制アクションを解除しました。' : '強制アクションを付与しました。',
    )
  }

  async function handleDelete(user: AdminUser) {
    await run(async () => {
      await deleteAdminUser(csrfToken, user.sub)
      setShowDelete(false)
      await refresh(user.sub)
    }, 'ユーザーの削除を予約しました。30 日以内なら復元できます。')
  }

  async function handlePurge(user: AdminUser) {
    await run(async () => {
      await deleteAdminUser(csrfToken, user.sub, { purge: true })
      setShowPurge(false)
      await refresh()
    }, 'ユーザーを完全に削除しました。')
  }

  async function handleRestore(user: AdminUser) {
    await run(async () => {
      await restoreAdminUser(csrfToken, user.sub)
      await refresh(user.sub)
    }, 'ユーザーを復元しました。')
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
                {(['all', 'active', 'disabled', 'pending_deletion'] as const).map((value) => (
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
                    {
                      {
                        all: 'すべて',
                        active: '有効',
                        disabled: '無効',
                        pending_deletion: '削除予約',
                      }[value]
                    }
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
                        <StatusBadge status={userLifecycleStatus(user)} />
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
                  csrfToken={csrfToken}
                  busy={busy}
                  onEdit={() => setShowUserEditor(true)}
                  onDisabled={() => requestDisable(selected)}
                  onDelete={() => setShowDelete(true)}
                  onRestore={() => void handleRestore(selected)}
                  onPurge={() => setShowPurge(true)}
                  onRequiredAction={(action, present) =>
                    void handleRequiredAction(selected, action, present)
                  }
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
          attributeDefs={attributeDefs}
          busy={busy}
          onSubmit={(input) => void handleUpdate(input)}
          onClose={() => setShowUserEditor(false)}
        />
      )}
      {showDelete && selected && (
        <DeleteUserDialog
          user={selected}
          busy={busy}
          mode="soft"
          onClose={() => setShowDelete(false)}
          onConfirm={() => void handleDelete(selected)}
        />
      )}
      {showPurge && selected && (
        <DeleteUserDialog
          user={selected}
          busy={busy}
          mode="purge"
          onClose={() => setShowPurge(false)}
          onConfirm={() => void handlePurge(selected)}
        />
      )}
      {showDisable && selected && (
        <DisableUserDialog
          user={selected}
          busy={busy}
          onClose={() => setShowDisable(false)}
          onConfirm={() => void handleDisabled(selected)}
        />
      )}
    </>
  )
}

// AdminUserDetailPage はユーザーの全情報を扱う専用詳細画面 (wi-39)。右ペインの
// 簡易ビューに収まらない網羅情報 (全プロフィール / 全属性 / ライフサイクル /
// 強制アクション / ロールとグループ) をここに集約し、縦スクロール前提で見せる。
export function AdminUserDetailPage({
  csrfToken,
  actorUsername,
  user: initialUser,
  schema,
}: {
  csrfToken: string
  actorUsername?: string
  user: AdminUser
  schema: TenantUserAttributeSchema
}) {
  const [user, setUser] = useState(initialUser)
  const [showEditor, setShowEditor] = useState(false)
  const [showDelete, setShowDelete] = useState(false)
  const [showPurge, setShowPurge] = useState(false)
  const [showDisable, setShowDisable] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  const attributeDefs = [...schema.builtin, ...schema.attributes]

  async function reload() {
    setUser(await getAdminUser(user.sub))
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

  async function handleUpdate(input: UpdateAdminUserInput) {
    await run(async () => {
      await updateAdminUser(csrfToken, user.sub, input)
      setShowEditor(false)
      await reload()
    }, 'ユーザーを更新しました。')
  }

  async function handleDisabled() {
    const disabled = !user.disabled_at
    await run(
      async () => {
        await setAdminUserDisabled(csrfToken, user.sub, disabled)
        setShowDisable(false)
        await reload()
      },
      disabled ? 'ユーザーを無効化しました。' : 'ユーザーを再有効化しました。',
    )
  }

  // 無効化は確認ダイアログを挟み、再有効化は即時実行する (片側非対称)。
  function requestDisable() {
    if (user.disabled_at) {
      void handleDisabled()
    } else {
      setShowDisable(true)
    }
  }

  async function handleDelete() {
    await run(async () => {
      await deleteAdminUser(csrfToken, user.sub)
      setShowDelete(false)
      await reload()
    }, 'ユーザーの削除を予約しました。30 日以内なら復元できます。')
  }

  async function handlePurge() {
    await run(async () => {
      await deleteAdminUser(csrfToken, user.sub, { purge: true })
      window.location.assign(tenantURL('/admin/users'))
    }, 'ユーザーを完全に削除しました。')
  }

  async function handleRestore() {
    await run(async () => {
      await restoreAdminUser(csrfToken, user.sub)
      await reload()
    }, 'ユーザーを復元しました。')
  }

  async function handleRequiredAction(action: string, present: boolean) {
    await run(
      async () => {
        if (present) {
          await clearAdminUserRequiredAction(csrfToken, user.sub, action)
        } else {
          await setAdminUserRequiredAction(csrfToken, user.sub, action)
        }
        await reload()
      },
      present ? '強制アクションを解除しました。' : '強制アクションを付与しました。',
    )
  }

  return (
    <>
      <AdminShell
        active="users"
        actorUsername={actorUsername}
        title={user.name || user.preferred_username}
        description={`@${user.preferred_username}`}
        actions={
          <div className="flex items-center gap-2">
            <a
              href={tenantURL('/admin/users')}
              className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 transition-colors hover:bg-slate-50"
            >
              <IconArrowLeft size={16} aria-hidden="true" />
              ユーザー一覧
            </a>
            <Button type="button" disabled={busy} onClick={() => setShowEditor(true)}>
              <IconPencil size={16} aria-hidden="true" />
              編集
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  type="button"
                  variant="outline"
                  className="size-9 px-0"
                  aria-label="ユーザー操作"
                  disabled={busy}
                >
                  <IconDotsVertical size={18} aria-hidden="true" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                {userLifecycleStatus(user) === 'pending_deletion' ? (
                  <>
                    <DropdownMenuItem onSelect={() => void handleRestore()}>
                      <IconRefresh size={17} aria-hidden="true" />
                      アカウントを復元
                    </DropdownMenuItem>
                    <DropdownMenuSeparator className="my-1 h-px bg-slate-200" />
                    <DropdownMenuItem className="text-red-700" onSelect={() => setShowPurge(true)}>
                      <IconTrash size={17} aria-hidden="true" />
                      完全に削除する
                    </DropdownMenuItem>
                  </>
                ) : (
                  <>
                    <DropdownMenuItem
                      className={user.disabled_at ? undefined : 'text-red-700'}
                      onSelect={() => requestDisable()}
                    >
                      {user.disabled_at ? (
                        <IconCheck size={17} aria-hidden="true" />
                      ) : (
                        <IconBan size={17} aria-hidden="true" />
                      )}
                      {user.disabled_at ? 'アカウントを再有効化' : 'アカウントを無効化'}
                    </DropdownMenuItem>
                    <DropdownMenuSeparator className="my-1 h-px bg-slate-200" />
                    <DropdownMenuItem className="text-red-700" onSelect={() => setShowDelete(true)}>
                      <IconTrash size={17} aria-hidden="true" />
                      アカウントを削除
                    </DropdownMenuItem>
                  </>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        }
      >
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

        <div className="flex items-center gap-3">
          <UserAvatar user={user} large />
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h2 className="truncate text-lg font-semibold text-slate-950">
                {user.name || user.preferred_username}
              </h2>
              <StatusBadge status={userLifecycleStatus(user)} compact />
            </div>
            <p className="mt-0.5 text-sm text-slate-500">@{user.preferred_username}</p>
          </div>
        </div>

        <div className="grid gap-5 xl:grid-cols-3">
          <div className="flex flex-col gap-5 xl:col-span-2">
            <Card className="p-5">
              <h3 className="text-xs font-bold uppercase tracking-[0.1em] text-slate-400">
                プロフィール
              </h3>
              <dl className="mt-3 grid gap-3 text-sm sm:grid-cols-2">
                <DetailRow icon={IconUser} label="名前" value={user.name || '未設定'} />
                <DetailRow icon={IconUser} label="名" value={user.given_name || '未設定'} />
                <DetailRow icon={IconUser} label="姓" value={user.family_name || '未設定'} />
                <DetailRow
                  icon={IconUser}
                  label="ユーザー名"
                  value={user.preferred_username}
                  mono
                />
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
                <DetailRow icon={IconUser} label="Subject ID" value={user.sub} mono />
              </dl>
            </Card>

            <Card className="p-5">
              <h3 className="text-xs font-bold uppercase tracking-[0.1em] text-slate-400">属性</h3>
              <p className="mt-1 text-xs text-slate-500">
                値が設定されている属性を区分ごとに表示します。編集は「編集」から行います。
              </p>
              <div className="mt-4 flex flex-col gap-5">
                {groupedAttributeDefs(attributeDefs).map((group) => (
                  <AttributeGroup
                    key={group.key}
                    title={group.title}
                    defs={group.defs}
                    user={user}
                  />
                ))}
              </div>
            </Card>
          </div>

          <div className="flex flex-col gap-5">
            <Card className="p-5">
              <h3 className="text-xs font-bold uppercase tracking-[0.1em] text-slate-400">
                ライフサイクル
              </h3>
              <dl className="mt-3 grid gap-3 text-sm">
                <DetailRow
                  icon={IconCircleCheck}
                  label="状態"
                  value={
                    { active: '有効', disabled: '無効', pending_deletion: '削除予約' }[
                      userLifecycleStatus(user)
                    ]
                  }
                />
                {userLifecycleStatus(user) === 'pending_deletion' && user.purge_after && (
                  <DetailRow
                    icon={IconClock}
                    label="完全削除予定"
                    value={`${formatDateTime(user.purge_after)}${
                      daysUntil(user.purge_after) !== null
                        ? ` (あと ${daysUntil(user.purge_after)} 日)`
                        : ''
                    }`}
                  />
                )}
                <DetailRow
                  icon={IconClock}
                  label="作成日時"
                  value={formatDateTime(user.created_at)}
                />
                <DetailRow
                  icon={IconClock}
                  label="更新日時"
                  value={formatDateTime(user.updated_at)}
                />
                <DetailRow
                  icon={IconClock}
                  label="最終ログイン"
                  value={user.last_login_at ? formatDateTime(user.last_login_at) : '未ログイン'}
                />
                <DetailRow
                  icon={IconKey}
                  label="パスワード変更"
                  value={
                    user.password_changed_at ? formatDateTime(user.password_changed_at) : '記録なし'
                  }
                />
              </dl>
              <div className="mt-5 border-t border-slate-200 pt-5">
                <UserRequiredActionsSection
                  user={user}
                  busy={busy}
                  onToggle={handleRequiredAction}
                />
              </div>
            </Card>

            <Card className="p-5">
              <UserGroupsSection user={user} csrfToken={csrfToken} />
            </Card>
          </div>
        </div>
      </AdminShell>

      {showEditor && (
        <UserEditorDialog
          user={user}
          attributeDefs={attributeDefs}
          busy={busy}
          onSubmit={(input) => void handleUpdate(input)}
          onClose={() => setShowEditor(false)}
        />
      )}
      {showDelete && (
        <DeleteUserDialog
          user={user}
          busy={busy}
          mode="soft"
          onClose={() => setShowDelete(false)}
          onConfirm={() => void handleDelete()}
        />
      )}
      {showPurge && (
        <DeleteUserDialog
          user={user}
          busy={busy}
          mode="purge"
          onClose={() => setShowPurge(false)}
          onConfirm={() => void handlePurge()}
        />
      )}
      {showDisable && (
        <DisableUserDialog
          user={user}
          busy={busy}
          onClose={() => setShowDisable(false)}
          onConfirm={() => void handleDisabled()}
        />
      )}
    </>
  )
}

// AttributeGroup は区分内で「値が設定されている」属性だけを読み取り表示する。
// 全 def を出すと未設定行で埋もれるため、設定済みのみを示し、無ければその旨を出す。
function AttributeGroup({
  title,
  defs,
  user,
}: {
  title: string
  defs: UserAttributeDef[]
  user: AdminUser
}) {
  const rows = defs
    .map((def) => ({ def, value: user.attributes?.[def.key] }))
    .filter((row): row is { def: UserAttributeDef; value: AttributeValue } => Boolean(row.value))
  return (
    <div>
      <h4 className="text-[0.68rem] font-bold uppercase tracking-wide text-slate-400">{title}</h4>
      {rows.length === 0 ? (
        <p className="mt-2 text-xs text-slate-400">設定された項目はありません</p>
      ) : (
        <dl className="mt-2 grid gap-2 text-sm sm:grid-cols-2">
          {rows.map(({ def, value }) => (
            <div key={def.key} className="grid gap-0.5">
              <dt className="text-xs text-slate-500">{attributeLabel(def)}</dt>
              <dd className="min-w-0 break-all text-slate-800">{attributeValueToText(value)}</dd>
            </div>
          ))}
        </dl>
      )}
    </div>
  )
}

// UserDetails は右ペインの詳細ビュー。同一画面で複数ユーザーを見比べられるよう
// 情報量は厚めに残しつつ、上部に「詳細 / 編集」を置いて専用詳細ページや
// 編集モーダルへすぐ飛べるようにする (wi-39)。
function UserDetails({
  user,
  csrfToken,
  busy,
  onEdit,
  onDisabled,
  onDelete,
  onRestore,
  onPurge,
  onRequiredAction,
}: {
  user: AdminUser
  csrfToken: string
  busy: boolean
  onEdit: () => void
  onDisabled: () => void
  onDelete: () => void
  onRestore: () => void
  onPurge: () => void
  onRequiredAction: (action: string, present: boolean) => void
}) {
  const pending = userLifecycleStatus(user) === 'pending_deletion'
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
              <StatusBadge status={userLifecycleStatus(user)} compact />
            </div>
            <p className="mt-0.5 text-sm text-slate-500">@{user.preferred_username}</p>
          </div>
        </div>

        {pending && <PendingDeletionNotice user={user} />}

        <div className="mt-4">
          <AdminPaneActions
            detailHref={tenantURL(`/admin/users/${encodeURIComponent(user.sub)}`)}
            busy={busy}
            onEdit={onEdit}
            menu={
              pending ? (
                <>
                  <DropdownMenuItem onSelect={onRestore}>
                    <IconRefresh size={17} aria-hidden="true" />
                    アカウントを復元
                  </DropdownMenuItem>
                  <DropdownMenuSeparator className="my-1 h-px bg-slate-200" />
                  <DropdownMenuItem className="text-red-700" onSelect={onPurge}>
                    <IconTrash size={17} aria-hidden="true" />
                    完全に削除する
                  </DropdownMenuItem>
                </>
              ) : (
                <>
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
                </>
              )
            }
          />
        </div>
      </div>

      <div className="flex flex-1 flex-col gap-6 p-5">
        <section>
          <h3 className="text-xs font-bold uppercase tracking-[0.1em] text-slate-400">
            プロフィール
          </h3>
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
            <DetailRow
              icon={IconClock}
              label="最終ログイン"
              value={user.last_login_at ? formatDateTime(user.last_login_at) : '未ログイン'}
            />
            <DetailRow
              icon={IconKey}
              label="パスワード変更"
              value={
                user.password_changed_at ? formatDateTime(user.password_changed_at) : '記録なし'
              }
            />
            <DetailRow icon={IconUser} label="Subject ID" value={user.sub} mono />
          </dl>
        </section>

        <UserRequiredActionsSection user={user} busy={busy} onToggle={onRequiredAction} />

        <UserGroupsSection user={user} csrfToken={csrfToken} />
      </div>
    </div>
  )
}

// PendingDeletionNotice は削除予約中のユーザーに、猶予残日数と自動完全削除の
// 予定を伝える amber バナー。復元動線 (メニューの「アカウントを復元」) を促す。
function PendingDeletionNotice({ user }: { user: AdminUser }) {
  const remaining = daysUntil(user.purge_after)
  return (
    <div className="mt-4 rounded-xl border border-amber-200 bg-amber-50 p-3 text-xs leading-5 text-amber-900">
      <p className="font-semibold">削除予約中</p>
      <p className="mt-1">
        {remaining !== null
          ? `あと ${remaining} 日で自動的に完全削除 (匿名化) されます。`
          : '猶予期間の経過後に自動的に完全削除 (匿名化) されます。'}
        復元期間中もログインとトークン更新は拒否されます。
      </p>
      {user.purge_after && (
        <p className="mt-1 text-amber-700">完全削除予定: {formatDateTime(user.purge_after)}</p>
      )}
    </div>
  )
}

function UserRequiredActionsSection({
  user,
  busy,
  onToggle,
}: {
  user: AdminUser
  busy: boolean
  onToggle: (action: string, present: boolean) => void
}) {
  const active = new Set(user.required_actions ?? [])
  return (
    <section>
      <h3 className="text-xs font-bold uppercase tracking-[0.1em] text-slate-400">
        強制アクション
      </h3>
      <p className="mt-1 text-xs text-slate-500">
        付与すると、次回ログイン時にユーザーへ対応を求めます。
      </p>
      <div className="mt-3 flex flex-wrap gap-2">
        {REQUIRED_ACTIONS.map((action) => {
          const present = active.has(action)
          return (
            <button
              key={action}
              type="button"
              disabled={busy}
              onClick={() => onToggle(action, present)}
              aria-pressed={present}
              className={cn(
                'rounded-full border px-3 py-1 text-xs font-medium transition-colors disabled:opacity-50',
                present
                  ? 'border-amber-300 bg-amber-50 text-amber-800 hover:bg-amber-100'
                  : 'border-slate-200 bg-white text-slate-500 hover:bg-slate-50',
              )}
            >
              {present ? '✓ ' : '+ '}
              {requiredActionLabel(action)}
            </button>
          )
        })}
      </div>
    </section>
  )
}

function UserGroupsSection({ user, csrfToken }: { user: AdminUser; csrfToken: string }) {
  const [data, setData] = useState<AdminUserGroups | null>(null)
  const [allGroups, setAllGroups] = useState<AdminGroup[]>([])
  const [error, setError] = useState('')
  const [adding, setAdding] = useState(false)
  const [selectedGroup, setSelectedGroup] = useState('')
  const { sub } = user

  const load = useCallback(async () => {
    try {
      const [groups, all] = await Promise.all([getAdminUserGroups(sub), listAdminGroups()])
      setData(groups)
      setAllGroups(all)
      setError('')
    } catch (err) {
      setError(
        err instanceof AuthenticationAPIError ? err.message : 'グループの取得に失敗しました。',
      )
    }
  }, [sub])

  useEffect(() => {
    setData(null)
    setSelectedGroup('')
    void load()
  }, [load])

  async function handleAdd() {
    if (!selectedGroup) return
    setAdding(true)
    try {
      await addAdminGroupMember(csrfToken, selectedGroup, user.sub)
      setSelectedGroup('')
      await load()
    } catch (err) {
      setError(
        err instanceof AuthenticationAPIError ? err.message : 'グループへの追加に失敗しました。',
      )
    } finally {
      setAdding(false)
    }
  }

  const memberIDs = new Set(data?.groups.map((g) => g.id) ?? [])
  const addable = allGroups.filter((g) => !memberIDs.has(g.id))

  return (
    <section className="border-t border-slate-200 pt-5">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold text-slate-900">ロールとグループ</h3>
          <p className="mt-0.5 text-xs text-slate-500">
            実効ロールは明示ロールとグループ由来ロールの和集合です。
          </p>
        </div>
        <IconShield size={18} className="text-slate-400" aria-hidden="true" />
      </div>

      {error && (
        <Alert variant="destructive" className="mt-3">
          {error}
        </Alert>
      )}

      <div className="mt-3 space-y-3">
        <RoleGroup label="明示ロール" roles={user.roles} />
        <RoleGroup label="グループ由来ロール" roles={data?.group_roles ?? []} />
        <RoleGroup label="実効ロール" roles={data?.effective_roles ?? user.roles} emphasis />
      </div>

      <div className="mt-4">
        <div className="flex items-center gap-2">
          <IconUsersGroup size={16} className="text-slate-400" aria-hidden="true" />
          <h4 className="text-xs font-bold uppercase tracking-[0.1em] text-slate-400">
            所属グループ
          </h4>
        </div>
        <div className="mt-2 rounded-xl border border-slate-200 bg-white p-3">
          {data && data.groups.length === 0 ? (
            <span className="text-xs text-slate-400">グループ未所属</span>
          ) : (
            <ul className="flex flex-col gap-2">
              {data?.groups.map((group) => (
                <li key={group.id} className="flex items-center justify-between gap-2">
                  <a
                    href={tenantURL(`/admin/groups?group=${encodeURIComponent(group.id)}`)}
                    className="text-sm font-medium text-indigo-700 hover:underline"
                  >
                    {group.name}
                  </a>
                  <RoleList roles={group.roles} />
                </li>
              ))}
            </ul>
          )}
        </div>

        {addable.length > 0 && (
          <div className="mt-2 flex items-center gap-2">
            <select
              value={selectedGroup}
              onChange={(event) => setSelectedGroup(event.target.value)}
              disabled={adding}
              className="h-9 flex-1 rounded-lg border border-slate-200 bg-white px-2 text-sm text-slate-700"
            >
              <option value="">グループを選択して追加…</option>
              {addable.map((group) => (
                <option key={group.id} value={group.id}>
                  {group.name}
                </option>
              ))}
            </select>
            <Button
              type="button"
              disabled={adding || !selectedGroup}
              onClick={() => void handleAdd()}
            >
              <IconUserPlus size={16} aria-hidden="true" />
              追加
            </Button>
          </div>
        )}
      </div>
    </section>
  )
}

function RoleGroup({
  label,
  roles,
  emphasis = false,
}: {
  label: string
  roles: string[]
  emphasis?: boolean
}) {
  return (
    <div
      className={cn(
        'rounded-xl border p-3',
        emphasis ? 'border-indigo-200 bg-indigo-50/50' : 'border-slate-200 bg-white',
      )}
    >
      <p className="mb-2 text-[0.68rem] font-semibold uppercase tracking-wide text-slate-400">
        {label}
      </p>
      <RoleList roles={roles} />
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

function attributeValueToText(value: AttributeValue): string {
  switch (value.type) {
    case 'string':
      return value.string ?? ''
    case 'date':
      return value.date ?? ''
    case 'number':
      return value.number?.toString() ?? ''
    case 'boolean':
      return value.boolean ? 'true' : 'false'
    case 'string_array':
      return (value.string_array ?? []).join(', ')
    default:
      return ''
  }
}

function textToAttributeValue(def: UserAttributeDef, text: string): AttributeValue | undefined {
  const trimmed = text.trim()
  switch (def.type) {
    case 'boolean':
      return { type: 'boolean', boolean: text === 'true' }
    case 'number':
      return trimmed ? { type: 'number', number: Number(trimmed) } : undefined
    case 'date':
      return trimmed ? { type: 'date', date: trimmed } : undefined
    case 'string_array': {
      const items = trimmed
        .split(',')
        .map((item) => item.trim())
        .filter((item) => item.length > 0)
      return items.length ? { type: 'string_array', string_array: items } : undefined
    }
    default:
      return trimmed ? { type: 'string', string: trimmed } : undefined
  }
}

function attributeDraftFromUser(user: AdminUser, defs: UserAttributeDef[]): Record<string, string> {
  const draft: Record<string, string> = {}
  for (const def of defs) {
    const value = user.attributes?.[def.key]
    draft[def.key] = value ? attributeValueToText(value) : ''
  }
  return draft
}

function attributeMapFromDraft(
  draft: Record<string, string>,
  defs: UserAttributeDef[],
): Record<string, AttributeValue> {
  const map: Record<string, AttributeValue> = {}
  for (const def of defs) {
    const value = textToAttributeValue(def, draft[def.key] ?? '')
    if (value) {
      map[def.key] = value
    }
  }
  return map
}

function AdminAttributeField({
  def,
  value,
  onChange,
}: {
  def: UserAttributeDef
  value: string
  onChange: (next: string) => void
}) {
  const id = `user-editor-attr-${def.key}`
  const label = attributeLabel(def)
  if (def.type === 'boolean') {
    return (
      <label htmlFor={id} className="inline-flex items-center gap-2 text-sm text-slate-700">
        <input
          id={id}
          type="checkbox"
          checked={value === 'true'}
          onChange={(event) => onChange(event.target.checked ? 'true' : 'false')}
          className="size-4 rounded border-slate-300"
        />
        <span className="font-mono">{label}</span>
      </label>
    )
  }
  return (
    <div className="grid gap-1.5">
      <Label htmlFor={id} className="font-mono text-xs">
        {label}
      </Label>
      <Input
        id={id}
        type={def.type === 'number' ? 'number' : def.type === 'date' ? 'date' : 'text'}
        value={value}
        placeholder={def.type === 'string_array' ? 'カンマ区切り' : undefined}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  )
}

function groupedAttributeDefs(defs: UserAttributeDef[]) {
  const groups = new Map<ReturnType<typeof attributeGroupKey>, UserAttributeDef[]>()
  for (const def of defs) {
    const key = attributeGroupKey(def)
    groups.set(key, [...(groups.get(key) ?? []), def])
  }
  return (['profile', 'organization', 'custom'] as const)
    .map((key) => ({ key, title: attributeGroupTitle(key), defs: groups.get(key) ?? [] }))
    .filter((group) => group.defs.length > 0)
}

function AdminAttributeEditorGroups({
  defs,
  values,
  onChange,
}: {
  defs: UserAttributeDef[]
  values: Record<string, string>
  onChange: (key: string, next: string) => void
}) {
  const groups = groupedAttributeDefs(defs)
  if (groups.length === 0) return null
  return (
    <section className="grid gap-4 border-t border-slate-200 pt-5">
      <div>
        <h3 className="text-xs font-bold uppercase tracking-normal text-slate-400">
          アカウント情報
        </h3>
        <p className="mt-1 text-xs leading-5 text-slate-500">保存時に属性全体が置換されます。</p>
      </div>
      {groups.map((group) => (
        <fieldset key={group.key} className="grid gap-3 rounded-lg border border-slate-200 p-4">
          <legend className="px-1 text-xs font-bold uppercase tracking-normal text-slate-500">
            {group.title}
          </legend>
          {group.defs.map((def) => (
            <AdminAttributeField
              key={def.key}
              def={def}
              value={values[def.key] ?? ''}
              onChange={(next) => onChange(def.key, next)}
            />
          ))}
        </fieldset>
      ))}
    </section>
  )
}

function UserEditorDialog({
  user,
  attributeDefs,
  busy,
  onSubmit,
  onClose,
}: {
  user: AdminUser
  attributeDefs: UserAttributeDef[]
  busy: boolean
  onSubmit: (input: UpdateAdminUserInput) => void
  onClose: () => void
}) {
  const initialUsername = user.preferred_username
  const initialName = user.name ?? ''
  const initialGivenName = user.given_name ?? ''
  const initialFamilyName = user.family_name ?? ''
  const initialEmail = user.email ?? ''
  const initialEmailVerified = user.email_verified
  const initialAttrDraft = attributeDraftFromUser(user, attributeDefs)

  const [username, setUsername] = useState(initialUsername)
  const [name, setName] = useState(initialName)
  const [givenName, setGivenName] = useState(initialGivenName)
  const [familyName, setFamilyName] = useState(initialFamilyName)
  const [email, setEmail] = useState(initialEmail)
  const [emailVerified, setEmailVerified] = useState(initialEmailVerified)
  const [emailVerifiedTouched, setEmailVerifiedTouched] = useState(false)
  const [roles, setRoles] = useState(user.roles.join(', '))
  const [attrDraft, setAttrDraft] = useState<Record<string, string>>(initialAttrDraft)
  const [confirming, setConfirming] = useState(false)

  const emailChanged = email !== initialEmail
  const effectiveEmailVerified = emailChanged && !emailVerifiedTouched ? false : emailVerified
  const trimmedUsername = username.trim()
  const usernameInvalid = trimmedUsername === ''
  const attributesChanged = attributeDefs.some(
    (def) => (attrDraft[def.key] ?? '') !== (initialAttrDraft[def.key] ?? ''),
  )
  const profileChanged =
    trimmedUsername !== initialUsername ||
    name !== initialName ||
    givenName !== initialGivenName ||
    familyName !== initialFamilyName ||
    email !== initialEmail ||
    effectiveEmailVerified !== initialEmailVerified ||
    attributesChanged
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
    if (givenName !== initialGivenName) input.given_name = givenName
    if (familyName !== initialFamilyName) input.family_name = familyName
    if (email !== initialEmail) input.email = email
    if (effectiveEmailVerified !== initialEmailVerified) {
      input.email_verified = effectiveEmailVerified
    }
    if (rolesChanged) input.roles = nextRoles
    // admin は属性バッグ全体を置換するため、ドラフトから完全な map を再構成する。
    if (attributesChanged) input.attributes = attributeMapFromDraft(attrDraft, attributeDefs)
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
      <Card className="relative flex max-h-[88vh] w-full max-w-lg flex-col overflow-hidden shadow-2xl">
        <div className="flex items-start justify-between border-b border-slate-200 px-6 py-5">
          <div>
            <p className="text-xs font-bold uppercase tracking-[0.12em] text-blue-700">
              プロフィールとアクセス
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

        <form onSubmit={handleSubmit} className="flex min-h-0 flex-1 flex-col">
          <div className="min-h-0 flex-1 overflow-y-auto">
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
                      <p className="text-sm font-semibold text-amber-950">
                        ロール変更を含む更新です
                      </p>
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
                    プロフィール
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
                  <div className="grid gap-4 sm:grid-cols-2">
                    <div className="grid gap-2">
                      <Label htmlFor="user-editor-given-name">名 (given_name)</Label>
                      <Input
                        id="user-editor-given-name"
                        value={givenName}
                        onChange={(event) => setGivenName(event.target.value)}
                      />
                    </div>
                    <div className="grid gap-2">
                      <Label htmlFor="user-editor-family-name">姓 (family_name)</Label>
                      <Input
                        id="user-editor-family-name"
                        value={familyName}
                        onChange={(event) => setFamilyName(event.target.value)}
                      />
                    </div>
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
                    ロール
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
                <AdminAttributeEditorGroups
                  defs={attributeDefs}
                  values={attrDraft}
                  onChange={(key, next) => setAttrDraft((current) => ({ ...current, [key]: next }))}
                />
              </div>
            )}
          </div>
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

// REQUIRE_USERNAME_CONFIRMATION は削除確認としてユーザー名の再入力を求める
// オプション機能のスイッチ。既定では無効。誤操作の最終防御を強めたい運用では
// true にすると、削除前に対象のユーザー名タイピングを要求する。
const REQUIRE_USERNAME_CONFIRMATION: boolean = false

// DeleteUserDialog は削除前の確認ダイアログ。mode='soft' は削除予約 (復元可能)、
// mode='purge' は完全削除 (匿名化・不可逆)。ユーザー名の再入力確認は
// REQUIRE_USERNAME_CONFIRMATION が true のときだけ求める (既定は無効)。
function DeleteUserDialog({
  user,
  busy,
  mode,
  onClose,
  onConfirm,
}: {
  user: AdminUser
  busy: boolean
  mode: 'soft' | 'purge'
  onClose: () => void
  onConfirm: () => void
}) {
  const [confirmName, setConfirmName] = useState('')
  const canConfirm = !REQUIRE_USERNAME_CONFIRMATION || confirmName === user.preferred_username
  const purge = mode === 'purge'

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!canConfirm) return
    onConfirm()
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
            <span
              className={cn(
                'flex size-9 shrink-0 items-center justify-center rounded-full',
                purge ? 'bg-red-50 text-red-700' : 'bg-amber-50 text-amber-700',
              )}
            >
              <IconAlertTriangle size={18} aria-hidden="true" />
            </span>
            <div>
              <p
                className={cn(
                  'text-xs font-bold uppercase tracking-[0.12em]',
                  purge ? 'text-red-700' : 'text-amber-700',
                )}
              >
                {purge ? 'Irreversible action' : 'Reversible for 30 days'}
              </p>
              <h2 id="delete-user-title" className="mt-1 text-xl font-semibold">
                {purge ? 'ユーザーを完全に削除' : 'ユーザーを削除'}
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
            {purge ? (
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
                  プロフィール情報は匿名化されます。<strong>元に戻せません。</strong>
                </p>
              </div>
            ) : (
              <div className="rounded-xl border border-amber-200 bg-amber-50 p-4 text-xs leading-5 text-amber-900">
                <p className="font-semibold">30 日以内なら復元できます。</p>
                <p className="mt-1.5">
                  削除予約中もログインとトークン更新は拒否されますが、プロフィール・同意・
                  セッションなどは温存されます。30 日を過ぎると自動的に完全削除 (匿名化)
                  され、元に戻せなくなります。
                </p>
              </div>
            )}

            {REQUIRE_USERNAME_CONFIRMATION && (
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
            )}
          </div>

          <div className="flex justify-end gap-2 border-t border-slate-200 bg-slate-50 px-6 py-4">
            <Button type="button" variant="outline" onClick={onClose} disabled={busy}>
              キャンセル
            </Button>
            <Button type="submit" variant="destructive" disabled={busy || !canConfirm}>
              <IconTrash size={16} aria-hidden="true" />
              {purge ? '完全に削除' : '削除を確定'}
            </Button>
          </div>
        </form>
      </Card>
    </div>
  )
}

// DisableUserDialog は無効化 (disable) 前に挟む軽い確認ダイアログ。削除と違い
// 復元可能なため username typing は求めず、影響と復元動線の説明だけで確定させる
// (enable 方向はダイアログ無しで即時実行する)。
function DisableUserDialog({
  user,
  busy,
  onClose,
  onConfirm,
}: {
  user: AdminUser
  busy: boolean
  onClose: () => void
  onConfirm: () => void
}) {
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/35 p-5 backdrop-blur-[2px]"
      role="dialog"
      aria-modal="true"
      aria-labelledby="disable-user-title"
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
              <IconBan size={18} aria-hidden="true" />
            </span>
            <div>
              <p className="text-xs font-bold uppercase tracking-[0.12em] text-red-700">
                Account access
              </p>
              <h2 id="disable-user-title" className="mt-1 text-xl font-semibold">
                アカウントを無効化
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

        <div className="grid gap-5 p-6">
          <div className="rounded-xl border border-red-200 bg-red-50 p-4 text-xs leading-5 text-red-900">
            <p className="font-semibold">無効化すると</p>
            <ul className="mt-1.5 list-disc pl-5">
              <li>新規ログインが拒否されます。</li>
              <li>既存のセッションが無効になります。</li>
              <li>リフレッシュトークンによる更新が拒否されます。</li>
            </ul>
            <p className="mt-2">
              この操作は <span className="font-semibold">アカウント状態 → 再有効化</span>{' '}
              から元に戻せます。
            </p>
          </div>
        </div>

        <div className="flex justify-end gap-2 border-t border-slate-200 bg-slate-50 px-6 py-4">
          <Button type="button" variant="outline" onClick={onClose} disabled={busy}>
            キャンセル
          </Button>
          <Button type="button" variant="destructive" disabled={busy} onClick={onConfirm}>
            <IconBan size={16} aria-hidden="true" />
            無効化を確定
          </Button>
        </div>
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
            <p className="text-xs font-bold uppercase tracking-[0.12em] text-blue-700">ユーザー</p>
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

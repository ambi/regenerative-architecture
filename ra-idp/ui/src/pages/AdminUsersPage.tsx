/**
 * /admin/users の SPA ページ。
 *
 * バックエンドは GET /admin/users で session + admin ロールを検証してから shell
 * を返す。本コンポーネントは /api/admin/users 系を呼んで一覧と CRUD を回す。
 * すべての変更系は CSRF token を `X-CSRF-Token` ヘッダで送る。
 */

import { IconAlertCircle, IconCheck, IconLoader2 } from '@tabler/icons-react'
import { type FormEvent, useCallback, useEffect, useId, useState } from 'react'
import { AdminLayout } from '@/components/layout/AdminLayout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { readAdminUsersContext } from '@/lib/page-context'

interface AdminUser {
  sub: string
  preferred_username: string
  name?: string
  email?: string
  email_verified: boolean
  mfa_enrolled: boolean
  roles: string[]
  disabled_at?: string
  created_at: string
  updated_at: string
}

interface ErrorResponse {
  error?: string
  message?: string
  violations?: string[]
}

export function AdminUsersPage() {
  const ctx = readAdminUsersContext()
  const errorId = useId()
  const [users, setUsers] = useState<AdminUser[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [showCreate, setShowCreate] = useState(false)

  const refresh = useCallback(async (): Promise<void> => {
    setError(null)
    setLoading(true)
    try {
      const res = await fetch(`${ctx.basePath}/api/admin/users`, { credentials: 'same-origin' })
      if (!res.ok) {
        setError(await describeError(res, 'ユーザー一覧を取得できませんでした。'))
        return
      }
      const body = (await res.json()) as { users: AdminUser[] }
      setUsers(body.users ?? [])
    } catch {
      setError('通信エラーが発生しました。')
    } finally {
      setLoading(false)
    }
  }, [ctx.basePath])

  useEffect(() => {
    void refresh()
  }, [refresh])

  async function setDisabled(user: AdminUser, disabled: boolean): Promise<void> {
    await mutate(
      `${ctx.basePath}/api/admin/users/${encodeURIComponent(user.sub)}/${disabled ? 'disable' : 'enable'}`,
      'POST',
      null,
      disabled
        ? `${user.preferred_username} を無効化しました。`
        : `${user.preferred_username} を再有効化しました。`,
    )
  }

  async function updateRoles(user: AdminUser, raw: string): Promise<void> {
    const roles = parseRoles(raw)
    await mutate(
      `${ctx.basePath}/api/admin/users/${encodeURIComponent(user.sub)}`,
      'PATCH',
      { roles },
      `${user.preferred_username} のロールを更新しました。`,
    )
  }

  async function createUser(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault()
    const data = new FormData(event.currentTarget)
    const payload = {
      preferred_username: String(data.get('preferred_username') ?? '').trim(),
      password: String(data.get('password') ?? ''),
      name: optional(data.get('name')),
      email: optional(data.get('email')),
      email_verified: data.get('email_verified') === 'on',
      roles: parseRoles(String(data.get('roles') ?? '')),
    }
    const ok = await mutate(
      `${ctx.basePath}/api/admin/users`,
      'POST',
      payload,
      'ユーザーを作成しました。',
    )
    if (ok) {
      event.currentTarget.reset()
      setShowCreate(false)
    }
  }

  async function mutate(
    path: string,
    method: 'POST' | 'PATCH',
    body: object | null,
    successMessage: string,
  ): Promise<boolean> {
    setError(null)
    setNotice(null)
    setSubmitting(true)
    try {
      const res = await fetch(path, {
        method,
        headers: {
          'content-type': 'application/json',
          'X-CSRF-Token': ctx.csrf,
        },
        credentials: 'same-origin',
        body: body == null ? undefined : JSON.stringify(body),
      })
      if (!res.ok) {
        setError(await describeError(res, '管理操作を完了できませんでした。'))
        return false
      }
      setNotice(successMessage)
      await refresh()
      return true
    } catch {
      setError('通信エラーが発生しました。')
      return false
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <AdminLayout
      title="ユーザー管理"
      description="所属テナントのユーザー、ロール、有効状態を管理します。"
      active="users"
      basePath={ctx.basePath}
      actorUsername={ctx.actorUsername}
    >
      <Card>
        <CardContent className="pt-6 space-y-5">
          {error ? (
            <Alert variant="destructive" id={errorId}>
              <IconAlertCircle className="h-4 w-4" aria-hidden />
              <AlertTitle>失敗しました</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}
          {notice ? (
            <Alert>
              <IconCheck className="h-4 w-4" aria-hidden />
              <AlertTitle>完了しました</AlertTitle>
              <AlertDescription>{notice}</AlertDescription>
            </Alert>
          ) : null}

          <div className="flex items-center justify-between gap-3">
            <p className="text-sm text-muted-foreground">
              {loading ? '読み込み中…' : `${users.length} 件のユーザー`}
            </p>
            <div className="flex gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => void refresh()}
                disabled={submitting || loading}
              >
                更新
              </Button>
              <Button
                type="button"
                onClick={() => setShowCreate((prev) => !prev)}
                disabled={submitting}
              >
                {showCreate ? '作成を閉じる' : '新規ユーザー'}
              </Button>
            </div>
          </div>

          {showCreate ? <CreateUserForm onSubmit={createUser} submitting={submitting} /> : null}

          <UserTable
            users={users}
            submitting={submitting}
            onSetDisabled={setDisabled}
            onUpdateRoles={updateRoles}
          />
        </CardContent>
      </Card>
    </AdminLayout>
  )
}

function CreateUserForm({
  onSubmit,
  submitting,
}: {
  onSubmit: (e: FormEvent<HTMLFormElement>) => Promise<void> | void
  submitting: boolean
}) {
  return (
    <form
      className="space-y-3 rounded-md border border-border/60 bg-muted/30 p-4"
      onSubmit={(e) => void onSubmit(e)}
      noValidate
    >
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <div className="space-y-1.5">
          <Label htmlFor="preferred_username">ユーザー名</Label>
          <Input id="preferred_username" name="preferred_username" required autoFocus />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="name">表示名</Label>
          <Input id="name" name="name" />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="email">メール</Label>
          <Input id="email" name="email" type="email" />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="password">初期パスワード</Label>
          <Input id="password" name="password" type="password" required minLength={12} />
        </div>
        <div className="space-y-1.5 sm:col-span-2">
          <Label htmlFor="roles">ロール (カンマ区切り)</Label>
          <Input id="roles" name="roles" placeholder="admin, support" />
        </div>
      </div>
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" name="email_verified" className="h-4 w-4" />
        メール確認済みとして作成
      </label>
      <Button type="submit" className="w-full" disabled={submitting}>
        {submitting ? (
          <>
            <IconLoader2 className="h-4 w-4 motion-safe:animate-spin" aria-hidden />
            送信中…
          </>
        ) : (
          '作成'
        )}
      </Button>
    </form>
  )
}

function UserTable({
  users,
  submitting,
  onSetDisabled,
  onUpdateRoles,
}: {
  users: AdminUser[]
  submitting: boolean
  onSetDisabled: (user: AdminUser, disabled: boolean) => Promise<void>
  onUpdateRoles: (user: AdminUser, raw: string) => Promise<void>
}) {
  if (users.length === 0) {
    return <p className="text-sm text-muted-foreground">ユーザーがありません。</p>
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse text-left text-sm">
        <thead className="text-xs uppercase tracking-wider text-muted-foreground">
          <tr>
            <th className="py-2 pr-3">ユーザー</th>
            <th className="py-2 pr-3">メール</th>
            <th className="py-2 pr-3">ロール</th>
            <th className="py-2 pr-3">状態</th>
            <th className="py-2 pr-3 text-right">操作</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border/60">
          {users.map((user) => (
            <UserRow
              key={user.sub}
              user={user}
              submitting={submitting}
              onSetDisabled={onSetDisabled}
              onUpdateRoles={onUpdateRoles}
            />
          ))}
        </tbody>
      </table>
    </div>
  )
}

function UserRow({
  user,
  submitting,
  onSetDisabled,
  onUpdateRoles,
}: {
  user: AdminUser
  submitting: boolean
  onSetDisabled: (user: AdminUser, disabled: boolean) => Promise<void>
  onUpdateRoles: (user: AdminUser, raw: string) => Promise<void>
}) {
  const [editingRoles, setEditingRoles] = useState(false)
  const [rolesDraft, setRolesDraft] = useState(user.roles.join(', '))
  const disabled = Boolean(user.disabled_at)

  useEffect(() => {
    setRolesDraft(user.roles.join(', '))
  }, [user.roles])

  return (
    <tr>
      <td className="py-3 pr-3">
        <div className="font-medium">{user.name || user.preferred_username}</div>
        <div className="text-xs text-muted-foreground font-mono">{user.sub}</div>
      </td>
      <td className="py-3 pr-3 text-sm text-muted-foreground">
        {user.email ?? '—'}
        {user.email && !user.email_verified ? (
          <span className="ml-2 text-amber-600">(未確認)</span>
        ) : null}
      </td>
      <td className="py-3 pr-3">
        {editingRoles ? (
          <div className="flex items-center gap-2">
            <Input
              value={rolesDraft}
              onChange={(e) => setRolesDraft(e.target.value)}
              className="h-8 w-44"
            />
            <Button
              type="button"
              variant="outline"
              disabled={submitting}
              onClick={() => {
                setEditingRoles(false)
                void onUpdateRoles(user, rolesDraft)
              }}
            >
              保存
            </Button>
            <Button
              type="button"
              variant="ghost"
              onClick={() => {
                setEditingRoles(false)
                setRolesDraft(user.roles.join(', '))
              }}
            >
              取消
            </Button>
          </div>
        ) : (
          <button
            type="button"
            className="text-left text-sm hover:underline"
            onClick={() => setEditingRoles(true)}
            disabled={submitting}
          >
            {user.roles.length === 0 ? (
              <span className="text-muted-foreground">権限なし</span>
            ) : (
              user.roles.join(', ')
            )}
          </button>
        )}
      </td>
      <td className="py-3 pr-3">
        <span
          className={
            disabled
              ? 'rounded-full bg-destructive/10 px-2 py-0.5 text-xs text-destructive'
              : 'rounded-full bg-emerald-500/10 px-2 py-0.5 text-xs text-emerald-700'
          }
        >
          {disabled ? '無効' : '有効'}
        </span>
      </td>
      <td className="py-3 pr-3 text-right">
        <Button
          type="button"
          variant={disabled ? 'outline' : 'destructive'}
          disabled={submitting}
          onClick={() => void onSetDisabled(user, !disabled)}
        >
          {disabled ? '再有効化' : '無効化'}
        </Button>
      </td>
    </tr>
  )
}

async function describeError(res: Response, fallback: string): Promise<string> {
  const body = (await res.json().catch(() => null)) as ErrorResponse | null
  if (res.status === 401) return '認証セッションが切れています。/login からやり直してください。'
  if (res.status === 403) return body?.message ?? '管理者権限が必要です。'
  if (body?.error === 'username_conflict')
    return body.message ?? 'ユーザー名は既に使用されています。'
  if (body?.error === 'password_policy' && body.violations) {
    return `パスワードポリシー違反: ${body.violations.join(', ')}`
  }
  if (body?.error === 'invalid_request') return body.message ?? 'リクエストの形式が不正です。'
  return body?.message ?? fallback
}

function parseRoles(raw: string): string[] {
  return [
    ...new Set(
      raw
        .split(',')
        .map((r) => r.trim())
        .filter(Boolean),
    ),
  ]
}

function optional(value: FormDataEntryValue | null): string | undefined {
  const v = String(value ?? '').trim()
  return v ? v : undefined
}

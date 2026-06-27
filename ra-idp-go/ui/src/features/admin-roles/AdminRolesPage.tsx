import {
  IconArrowLeft,
  IconChevronRight,
  IconPencil,
  IconShieldCheck,
  IconTrash,
  IconUsers,
} from '@tabler/icons-react'
import { useMemo, useState } from 'react'
import { tenantURL } from '../../api'
import { AdminPaneActions } from '../../components/AdminPaneActions'
import { AdminShell } from '../../components/AdminShell'
import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import { cn } from '../../lib/utils'
import type { AdminRole, AdminUser } from '../../types'

export function AdminRolesPage({
  actorUsername,
  roles,
  users,
}: {
  actorUsername?: string
  roles: AdminRole[]
  users: AdminUser[]
}) {
  const [selectedName, setSelectedName] = useState(roles[0]?.name ?? '')
  const selected = roles.find((role) => role.name === selectedName)
  const roleCounts = useMemo(
    () =>
      Object.fromEntries(
        roles.map((role) => [
          role.name,
          users.filter((user) => user.roles.includes(role.name)).length,
        ]),
      ),
    [roles, users],
  )

  return (
    <AdminShell
      active="roles"
      actorUsername={actorUsername}
      title="ロール"
      description="管理ロールと、各ロールに許可される操作を確認します。"
    >
      <section className="grid gap-3 sm:grid-cols-2" aria-label="ロール概要">
        {roles.map((role) => (
          <MetricCard
            key={role.name}
            label={role.name}
            value={roleCounts[role.name] ?? 0}
            hint={`${role.permissions.length} 件の操作`}
          />
        ))}
      </section>

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(420px,0.8fr)]">
        <Card className="overflow-hidden">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-slate-200 bg-slate-50 text-xs font-semibold uppercase tracking-wide text-slate-500">
              <tr>
                <th className="px-5 py-3.5">ロール</th>
                <th className="px-5 py-3.5">付与人数</th>
                <th className="px-5 py-3.5">操作数</th>
                <th className="px-5 py-3.5" />
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {roles.map((role) => (
                <tr
                  key={role.name}
                  onClick={() => setSelectedName(role.name)}
                  className={cn(
                    'cursor-pointer transition-colors hover:bg-slate-50',
                    role.name === selectedName && 'bg-blue-50/60 hover:bg-blue-50/80',
                  )}
                >
                  <td className="px-5 py-4">
                    <p className="font-mono font-semibold text-slate-900">{role.name}</p>
                    <p className="mt-1 line-clamp-2 text-xs leading-5 text-slate-500">
                      {role.description}
                    </p>
                  </td>
                  <td className="px-5 py-4 font-semibold">{roleCounts[role.name] ?? 0}</td>
                  <td className="px-5 py-4">{role.permissions.length}</td>
                  <td className="px-5 py-4 text-right">
                    <IconChevronRight size={16} className="text-slate-400" aria-hidden="true" />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>

        <Card className="overflow-hidden">
          {selected ? (
            <div>
              <div className="border-b border-slate-200 bg-white p-4">
                <div className="flex flex-wrap items-center gap-2">
                  <AdminPaneActions
                    detailHref={tenantURL(`/admin/roles/${encodeURIComponent(selected.name)}`)}
                  />
                  <RoleUnavailableActions />
                </div>
              </div>
              <RoleDetails
                role={selected}
                count={roleCounts[selected.name] ?? 0}
                usernames={users
                  .filter((user) => user.roles.includes(selected.name))
                  .map((user) => user.preferred_username)}
              />
            </div>
          ) : (
            <div className="p-8 text-sm text-slate-500">ロールを選択してください。</div>
          )}
        </Card>
      </div>
    </AdminShell>
  )
}

// AdminRoleDetailPage はロールの全操作と付与ユーザーを扱う専用詳細画面 (wi-39)。
export function AdminRoleDetailPage({
  actorUsername,
  role,
  count,
  usernames,
}: {
  actorUsername?: string
  role: AdminRole
  count: number
  usernames: string[]
}) {
  return (
    <AdminShell
      active="roles"
      actorUsername={actorUsername}
      title={role.name}
      description={role.description}
      actions={
        <div className="flex items-center gap-2">
          <a
            href={tenantURL('/admin/roles')}
            className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 transition-colors hover:bg-slate-50"
          >
            <IconArrowLeft size={16} aria-hidden="true" />
            ロール一覧
          </a>
          <RoleUnavailableActions />
        </div>
      }
    >
      <Card className="overflow-hidden">
        <RoleDetails role={role} count={count} usernames={usernames} />
      </Card>
    </AdminShell>
  )
}

function RoleUnavailableActions() {
  return (
    <>
      <Button type="button" variant="outline" disabled title="標準ロールは編集できません">
        <IconPencil size={16} aria-hidden="true" />
        編集
      </Button>
      <Button type="button" variant="outline" disabled title="標準ロールは削除できません">
        <IconTrash size={16} aria-hidden="true" />
        削除
      </Button>
    </>
  )
}

function RoleDetails({
  role,
  count,
  usernames,
}: {
  role: AdminRole
  count: number
  usernames: string[]
}) {
  return (
    <div>
      <div className="border-b border-slate-200 p-5">
        <div className="flex items-start gap-3">
          <span className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-blue-50 text-blue-700">
            <IconShieldCheck size={20} aria-hidden="true" />
          </span>
          <div>
            <h2 className="font-mono text-lg font-semibold text-slate-950">{role.name}</h2>
            <p className="mt-1 text-sm leading-6 text-slate-600">{role.description}</p>
            <div className="mt-3 flex flex-wrap gap-1.5">
              {role.aliases.map((alias) => (
                <span
                  key={alias}
                  className="rounded-md bg-slate-100 px-2 py-1 text-xs text-slate-600"
                >
                  {alias}
                </span>
              ))}
            </div>
          </div>
        </div>
      </div>

      <section className="border-b border-slate-200 p-5">
        <h3 className="text-sm font-semibold text-slate-900">許可される操作</h3>
        {role.permissions.length === 0 ? (
          <p className="mt-3 text-sm text-slate-500">
            現在の管理者には、このロールの詳細を表示する権限がありません。
          </p>
        ) : (
          <div className="mt-3 grid gap-3">
            {role.permissions.map((permission) => (
              <div key={permission.name} className="rounded-xl border border-slate-200 p-4">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <p className="font-semibold text-slate-900">{permission.name}</p>
                  <code className="rounded bg-slate-100 px-2 py-1 text-xs text-slate-600">
                    {permission.action}
                  </code>
                </div>
                <p className="mt-2 text-xs leading-5 text-slate-600">{permission.description}</p>
                <div className="mt-3 grid gap-1.5">
                  {permission.interfaces.map((iface) => (
                    <div
                      key={iface.name}
                      className="flex items-center gap-2 rounded-lg bg-slate-50 px-3 py-2 text-xs"
                    >
                      <span className="w-14 font-bold text-blue-700">{iface.method}</span>
                      <code className="min-w-0 break-all text-slate-700">{iface.path}</code>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </section>

      <section className="p-5">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-sm font-semibold text-slate-900">このロールを持つユーザー</h3>
            <p className="mt-1 text-xs text-slate-500">{count} 人</p>
          </div>
          <IconUsers size={18} className="text-slate-400" aria-hidden="true" />
        </div>
        {usernames.length > 0 ? (
          <div className="mt-3 flex flex-wrap gap-2">
            {usernames.map((username) => (
              <a
                key={username}
                href={`${tenantURL('/admin/users')}?role=${encodeURIComponent(role.name)}`}
                className="rounded-lg border border-slate-200 px-2.5 py-1.5 text-xs font-medium text-blue-700 hover:bg-blue-50"
              >
                @{username}
              </a>
            ))}
          </div>
        ) : (
          <p className="mt-3 text-sm text-slate-500">該当するユーザーはいません。</p>
        )}
      </section>
    </div>
  )
}

function MetricCard({ label, value, hint }: { label: string; value: number; hint: string }) {
  return (
    <Card className="p-5">
      <p className="font-mono text-xs font-semibold text-slate-500">{label}</p>
      <p className="mt-2 text-3xl font-semibold text-slate-950">{value}</p>
      <p className="mt-1 text-xs text-slate-500">{hint}</p>
    </Card>
  )
}

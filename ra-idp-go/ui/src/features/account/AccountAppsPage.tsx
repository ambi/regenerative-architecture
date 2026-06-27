import { IconLayoutGrid } from '@tabler/icons-react'
import { AccountShell } from '../../components/AccountShell'
import { Card } from '../../components/ui/card'
import type { MyApplication } from '../../types'

function initials(name: string): string {
  return name.trim().slice(0, 2).toUpperCase() || '??'
}

function AppTile({ app }: { app: MyApplication }) {
  const icon = app.icon_url ? (
    <img src={app.icon_url} alt="" className="size-12 rounded-xl object-cover" aria-hidden="true" />
  ) : (
    <span className="flex size-12 items-center justify-center rounded-xl border border-blue-100 bg-blue-50 text-sm font-bold text-blue-700">
      {initials(app.name)}
    </span>
  )
  const body = (
    <Card className="flex h-full flex-col items-center gap-3 p-5 text-center transition hover:border-blue-300 hover:shadow-md">
      {icon}
      <span className="text-sm font-semibold text-slate-900">{app.name}</span>
    </Card>
  )
  if (app.launch_url) {
    return (
      <a
        href={app.launch_url}
        target="_blank"
        rel="noreferrer"
        className="block focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
      >
        {body}
      </a>
    )
  }
  return body
}

export function AccountAppsPage({
  username,
  applications,
  isAdmin,
}: {
  username: string
  applications: MyApplication[]
  isAdmin: boolean
}) {
  return (
    <AccountShell
      active="apps"
      username={username}
      isAdmin={isAdmin}
      title="アプリ"
      description="あなたが利用できるアプリケーションです。タイルから起動できます。"
    >
      {applications.length === 0 ? (
        <Card className="flex flex-col items-center gap-2 p-10 text-center">
          <IconLayoutGrid size={28} className="text-slate-300" aria-hidden="true" />
          <p className="text-sm text-slate-500">利用できるアプリはまだありません。</p>
        </Card>
      ) : (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4">
          {applications.map((app) => (
            <AppTile key={app.application_id} app={app} />
          ))}
        </div>
      )}
    </AccountShell>
  )
}

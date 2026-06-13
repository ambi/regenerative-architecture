import { IconKey, IconLogout, IconShieldLock, IconUsers } from '@tabler/icons-react'
import type { ReactNode } from 'react'
import { useBranding } from '@/branding/context'

export function AdminLayout({
  title,
  description,
  active,
  basePath,
  actorUsername,
  children,
}: {
  title: string
  description: string
  active: 'users' | 'clients'
  basePath: string
  actorUsername: string
  children: ReactNode
}) {
  const brand = useBranding()
  return (
    <div className="min-h-full bg-muted/30">
      <header className="border-b border-border bg-card">
        <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-5">
          <a href={`${basePath}/admin/users`} className="flex items-center gap-2.5 font-semibold">
            <span className="grid h-8 w-8 place-items-center rounded-md bg-primary text-primary-foreground">
              <IconShieldLock className="h-4 w-4" aria-hidden />
            </span>
            <span>{brand.name} Administration</span>
          </a>
          <div className="flex items-center gap-4 text-sm">
            <span className="hidden text-muted-foreground sm:inline">
              {actorUsername || 'administrator'}
            </span>
            <a
              href={`${basePath}/end_session`}
              className="inline-flex items-center gap-2 rounded-md px-3 py-2 hover:bg-muted"
            >
              <IconLogout className="h-4 w-4" aria-hidden />
              ログアウト
            </a>
          </div>
        </div>
      </header>
      <div className="mx-auto grid max-w-7xl gap-6 px-5 py-6 lg:grid-cols-[220px_minmax(0,1fr)]">
        <nav aria-label="管理メニュー" className="space-y-1">
          <AdminLink
            href={`${basePath}/admin/users`}
            active={active === 'users'}
            icon={<IconUsers className="h-4 w-4" />}
          >
            ユーザー
          </AdminLink>
          <AdminLink
            href={`${basePath}/admin/clients`}
            active={active === 'clients'}
            icon={<IconKey className="h-4 w-4" />}
          >
            クライアント
          </AdminLink>
        </nav>
        <main className="min-w-0">
          <div className="mb-6">
            <h1 className="font-serif text-3xl font-semibold tracking-tight">{title}</h1>
            <p className="mt-2 text-sm text-muted-foreground">{description}</p>
          </div>
          {children}
        </main>
      </div>
    </div>
  )
}

function AdminLink({
  href,
  active,
  icon,
  children,
}: {
  href: string
  active: boolean
  icon: ReactNode
  children: ReactNode
}) {
  return (
    <a
      href={href}
      aria-current={active ? 'page' : undefined}
      className={`flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium ${
        active ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'
      }`}
    >
      {icon}
      {children}
    </a>
  )
}

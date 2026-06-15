import { IconLogout } from '@tabler/icons-react'
import type { ReactNode } from 'react'
import { tenantURL } from '../api'
import { adminNavItems, type AdminNavKey } from '../lib/adminNav'
import { cn } from '../lib/utils'
import { Brand } from './Brand'
import { Button } from './ui/button'

type AdminShellProps = {
  active: AdminNavKey
  actorUsername?: string
  title: string
  description?: string
  actions?: ReactNode
  children: ReactNode
}

export function AdminShell({
  active,
  actorUsername,
  title,
  description,
  actions,
  children,
}: AdminShellProps) {
  const items = adminNavItems(active)
  const currentItem = items.find((item) => item.active)
  return (
    <div className="min-h-screen bg-[#f5f7fa] text-slate-950">
      <header className="sticky top-0 z-30 border-b border-slate-200 bg-white/95 backdrop-blur">
        <div className="flex h-16 items-center justify-between px-5 lg:px-7">
          <div className="flex items-center gap-5">
            <Brand compact />
            <div className="hidden h-6 w-px bg-slate-200 sm:block" />
            <div className="hidden items-center gap-2 px-2 py-1.5 text-sm font-medium text-slate-700 sm:flex">
              <span className="flex size-7 items-center justify-center rounded-md bg-blue-50 text-xs font-bold text-blue-700">
                RA
              </span>
              Default organization
            </div>
          </div>
          <div className="flex items-center gap-3">
            <div className="hidden text-right sm:block">
              <p className="text-sm font-semibold text-slate-800">
                {actorUsername ?? 'administrator'}
              </p>
              <p className="text-xs text-slate-500">Organization administrator</p>
            </div>
            <span className="flex size-9 items-center justify-center rounded-full bg-slate-900 text-sm font-semibold text-white">
              {(actorUsername ?? 'A').slice(0, 1).toUpperCase()}
            </span>
            <Button asChild variant="ghost" className="px-2.5" aria-label="ログアウト">
              <a href={tenantURL('/end_session')}>
                <IconLogout size={18} aria-hidden="true" />
              </a>
            </Button>
          </div>
        </div>
      </header>

      <div className="grid min-h-[calc(100vh-4rem)] lg:grid-cols-[248px_minmax(0,1fr)]">
        <aside className="hidden border-r border-slate-200 bg-white lg:flex lg:flex-col">
          <nav className="flex flex-1 flex-col gap-1 p-4" aria-label="管理メニュー">
            <p className="mb-2 px-3 text-[0.67rem] font-bold uppercase tracking-[0.14em] text-slate-400">
              Identity management
            </p>
            {items.map((item) => (
              <a
                key={item.key}
                href={item.href}
                className={cn(
                  'flex h-10 w-full items-center gap-3 rounded-lg px-3 text-left text-sm font-medium',
                  item.active
                    ? 'bg-blue-50 text-blue-800'
                    : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900',
                )}
                aria-current={item.active ? 'page' : undefined}
              >
                <item.icon size={18} stroke={1.8} aria-hidden="true" />
                {item.label}
              </a>
            ))}
          </nav>
        </aside>

        <main className="min-w-0 p-6 lg:p-10">
          <div className="mx-auto flex max-w-[1500px] flex-col gap-6">
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div>
                <nav aria-label="breadcrumb">
                  <ol className="flex items-center gap-2 text-xs font-semibold text-slate-500">
                    <li>Identity management</li>
                    <li aria-hidden="true">/</li>
                    <li aria-current="page">{currentItem?.label ?? title}</li>
                  </ol>
                </nav>
                <h1 className="mt-2 text-3xl font-semibold tracking-[-0.03em] text-slate-900">
                  {title}
                </h1>
                {description ? (
                  <p className="mt-2 max-w-[70ch] text-sm text-slate-600">{description}</p>
                ) : null}
              </div>
              {actions ? <div className="flex items-center gap-2">{actions}</div> : null}
            </div>
            {children}
          </div>
        </main>
      </div>
    </div>
  )
}

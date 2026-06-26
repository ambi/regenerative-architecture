import { IconChevronDown, IconLogout, IconUserCircle } from '@tabler/icons-react'
import type { ReactNode } from 'react'
import { logout, tenantURL } from '../api'
import { adminNavItems, type AdminNavKey } from '../lib/adminNav'
import { cn } from '../lib/utils'
import { Brand } from './Brand'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from './ui/dropdown-menu'

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
    <div className="app-surface">
      <header className="app-header">
        <div className="flex h-16 items-center justify-between px-5 lg:px-7">
          <div className="flex items-center gap-5">
            <a
              href={tenantURL('/admin')}
              aria-current={active === 'dashboard' ? 'page' : undefined}
              aria-label="管理コンソール"
              className="rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600/30"
            >
              <Brand compact />
            </a>
            <div className="hidden h-6 w-px bg-slate-200/80 sm:block" />
            <div className="hidden items-center gap-2 rounded-lg border border-slate-200/80 bg-white/70 px-2.5 py-1.5 text-sm font-medium text-slate-700 shadow-xs sm:flex">
              <span className="flex size-7 items-center justify-center rounded-md bg-slate-950 text-xs font-bold text-white">
                RA
              </span>
              Default organization
            </div>
          </div>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                className="flex items-center gap-3 rounded-lg px-2 py-1.5 text-left transition-colors hover:bg-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600/30"
                aria-label="アカウントメニュー"
              >
                <div className="hidden text-right sm:block">
                  <p className="text-sm font-semibold text-slate-800">
              {actorUsername ?? 'administrator'}
                  </p>
                  <p className="text-xs text-slate-500">Organization administrator</p>
                </div>
                <span className="flex size-9 items-center justify-center rounded-lg bg-slate-950 text-sm font-semibold text-white shadow-sm">
                  {(actorUsername ?? 'A').slice(0, 1).toUpperCase()}
                </span>
                <IconChevronDown size={15} className="text-slate-400" aria-hidden="true" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuLabel>
                <p className="text-xs font-medium text-slate-500">ログイン中</p>
                <p className="mt-0.5 text-sm font-semibold text-slate-900">
                  {actorUsername ?? 'administrator'}
                </p>
              </DropdownMenuLabel>
              <DropdownMenuSeparator className="my-1 h-px bg-slate-200" />
              <DropdownMenuItem asChild>
                <a href={tenantURL('/account')}>
                  <IconUserCircle size={17} aria-hidden="true" />
                  マイページ
                </a>
              </DropdownMenuItem>
              <DropdownMenuSeparator className="my-1 h-px bg-slate-200" />
              <DropdownMenuItem asChild>
                <button
                  type="button"
                  onClick={() => {
                    void logout('admin')
                  }}
                  className="w-full text-left text-red-700"
                >
                  <IconLogout size={17} aria-hidden="true" />
                  ログアウト
                </button>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </header>

      <div className="grid min-h-[calc(100vh-4rem)] lg:grid-cols-[248px_minmax(0,1fr)]">
        <aside className="app-sidebar">
          <nav className="flex flex-1 flex-col gap-1 p-4" aria-label="管理メニュー">
            {items.map((item) => (
              <a
                key={item.key}
                href={item.href}
                className={cn(
                  'flex h-10 w-full items-center gap-3 rounded-lg px-3 text-left text-sm font-medium transition-[background-color,color,box-shadow]',
                  item.active
                    ? 'bg-slate-950 text-white shadow-sm'
                    : 'text-slate-600 hover:bg-white hover:text-slate-950 hover:shadow-xs',
                )}
                aria-current={item.active ? 'page' : undefined}
              >
                <item.icon size={18} stroke={1.8} aria-hidden="true" />
                {item.label}
              </a>
            ))}
          </nav>
        </aside>

        <main className="app-main">
          <div className="app-content max-w-[1500px]">
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div>
                <nav aria-label="breadcrumb">
                  <ol className="flex items-center gap-2 text-xs font-semibold text-slate-500">
                    {active === 'dashboard' ? (
                      <li aria-current="page">管理コンソール</li>
                    ) : (
                      <>
                        <li>
                          <a href={tenantURL('/admin')} className="hover:text-blue-700 hover:underline">
                            管理コンソール
                          </a>
                        </li>
                        <li aria-hidden="true">/</li>
                        <li aria-current="page">{currentItem?.label ?? title}</li>
                      </>
                    )}
                  </ol>
                </nav>
                <h1 className="app-page-title mt-2">
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

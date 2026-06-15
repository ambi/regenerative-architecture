import { IconChevronDown, IconKey, IconLogout, IconUserCircle } from '@tabler/icons-react'
import type { ReactNode } from 'react'
import { tenantURL } from '../api'
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
    <div className="min-h-screen bg-[#f5f7fa] text-slate-950">
      <header className="sticky top-0 z-30 border-b border-slate-200 bg-white/95 backdrop-blur">
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
            <div className="hidden h-6 w-px bg-slate-200 sm:block" />
            <div className="hidden items-center gap-2 px-2 py-1.5 text-sm font-medium text-slate-700 sm:flex">
              <span className="flex size-7 items-center justify-center rounded-md bg-blue-50 text-xs font-bold text-blue-700">
                RA
              </span>
              Default organization
            </div>
          </div>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                className="flex items-center gap-3 rounded-lg px-2 py-1.5 text-left hover:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600/30"
                aria-label="アカウントメニュー"
              >
                <div className="hidden text-right sm:block">
                  <p className="text-sm font-semibold text-slate-800">
                    {actorUsername ?? 'administrator'}
                  </p>
                  <p className="text-xs text-slate-500">Organization administrator</p>
                </div>
                <span className="flex size-9 items-center justify-center rounded-full bg-slate-900 text-sm font-semibold text-white">
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
                <a href={tenantURL('/account/password')}>
                  <IconUserCircle size={17} aria-hidden="true" />
                  アカウント概要
                </a>
              </DropdownMenuItem>
              <DropdownMenuItem asChild>
                <a href={tenantURL('/account/password')}>
                  <IconKey size={17} aria-hidden="true" />
                  パスワードを変更
                </a>
              </DropdownMenuItem>
              <DropdownMenuSeparator className="my-1 h-px bg-slate-200" />
              <DropdownMenuItem asChild>
                <a href={tenantURL('/end_session')} className="text-red-700">
                  <IconLogout size={17} aria-hidden="true" />
                  ログアウト
                </a>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </header>

      <div className="grid min-h-[calc(100vh-4rem)] lg:grid-cols-[248px_minmax(0,1fr)]">
        <aside className="hidden border-r border-slate-200 bg-white lg:flex lg:flex-col">
          <nav className="flex flex-1 flex-col gap-1 p-4" aria-label="管理メニュー">
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

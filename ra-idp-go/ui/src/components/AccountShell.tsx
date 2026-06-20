import {
  IconApps,
  IconChevronDown,
  IconDatabaseCog,
  IconKey,
  IconLogout,
  IconMail,
  IconShieldLock,
  IconUser,
  IconUserCircle,
} from '@tabler/icons-react'
import type { ReactNode } from 'react'
import { tenantURL } from '../api'
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

// AccountShell は end-user 向け「マイページ」(account portal) の外枠 (wi-21)。
// admin shell とは別の trust boundary で、admin 機能への導線は持たない。
// 未実装のタブ (アクティビティ / 接続済みアプリ / データとプライバシー) は
// 「未実装機能を操作可能に見せない」方針に従い、実装済みのものだけ出す。
export type AccountNavKey =
  | 'home'
  | 'profile'
  | 'emails'
  | 'security'
  | 'applications'
  | 'data'

const navItems: { key: AccountNavKey; label: string; href: string; icon: typeof IconUser }[] = [
  { key: 'home', label: 'アカウント概要', href: '/account', icon: IconUserCircle },
  { key: 'profile', label: '個人情報', href: '/account/profile', icon: IconUser },
  { key: 'emails', label: 'メールアドレス', href: '/account/emails', icon: IconMail },
  { key: 'security', label: 'パスワード', href: '/account/password', icon: IconKey },
  { key: 'applications', label: '接続済みアプリ', href: '/account/applications', icon: IconApps },
  { key: 'data', label: 'データとプライバシー', href: '/account/data', icon: IconDatabaseCog },
]

type AccountShellProps = {
  active: AccountNavKey
  username: string
  isAdmin?: boolean
  title: string
  description?: string
  children: ReactNode
}

export function AccountShell({
  active,
  username,
  isAdmin = false,
  title,
  description,
  children,
}: AccountShellProps) {
  return (
    <div className="min-h-screen bg-[#f5f7fa] text-slate-950">
      <header className="sticky top-0 z-30 border-b border-slate-200 bg-white/95 backdrop-blur">
        <div className="flex h-16 items-center justify-between px-5 lg:px-7">
          <div className="flex items-center gap-5">
            <a
              href={tenantURL('/account')}
              aria-label="アカウント"
              className="rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600/30"
            >
              <Brand compact />
            </a>
            <div className="hidden h-6 w-px bg-slate-200 sm:block" />
            <span className="hidden items-center gap-2 px-2 py-1.5 text-sm font-medium text-slate-600 sm:flex">
              <IconShieldLock size={16} className="text-slate-400" aria-hidden="true" />
              マイページ
            </span>
          </div>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                className="flex items-center gap-3 rounded-lg px-2 py-1.5 text-left hover:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600/30"
                aria-label="アカウントメニュー"
              >
                <div className="hidden text-right sm:block">
                  <p className="text-sm font-semibold text-slate-800">{username}</p>
                  <p className="text-xs text-slate-500">サインイン中</p>
                </div>
                <span className="flex size-9 items-center justify-center rounded-full bg-slate-900 text-sm font-semibold text-white">
                  {username.slice(0, 1).toUpperCase()}
                </span>
                <IconChevronDown size={15} className="text-slate-400" aria-hidden="true" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuLabel>
                <p className="text-xs font-medium text-slate-500">ログイン中</p>
                <p className="mt-0.5 text-sm font-semibold text-slate-900">{username}</p>
              </DropdownMenuLabel>
              <DropdownMenuSeparator className="my-1 h-px bg-slate-200" />
              {isAdmin ? (
                <DropdownMenuItem asChild>
                  <a href={tenantURL('/admin')}>
                    <IconShieldLock size={17} aria-hidden="true" />
                    管理コンソール
                  </a>
                </DropdownMenuItem>
              ) : null}
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
          <nav className="flex flex-1 flex-col gap-1 p-4" aria-label="マイページメニュー">
            {navItems.map((item) => (
              <a
                key={item.key}
                href={tenantURL(item.href)}
                className={cn(
                  'flex h-10 w-full items-center gap-3 rounded-lg px-3 text-left text-sm font-medium',
                  item.key === active
                    ? 'bg-blue-50 text-blue-800'
                    : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900',
                )}
                aria-current={item.key === active ? 'page' : undefined}
              >
                <item.icon size={18} stroke={1.8} aria-hidden="true" />
                {item.label}
              </a>
            ))}
          </nav>
        </aside>

        <main className="min-w-0 p-6 lg:p-10">
          <div className="mx-auto flex max-w-[920px] flex-col gap-6">
            <div>
              <h1 className="text-3xl font-semibold tracking-[-0.03em] text-slate-900">{title}</h1>
              {description ? (
                <p className="mt-2 max-w-[70ch] text-sm text-slate-600">{description}</p>
              ) : null}
            </div>
            {children}
          </div>
        </main>
      </div>
    </div>
  )
}

import {
  IconApps,
  IconChevronDown,
  IconDatabaseCog,
  IconHistory,
  IconLayoutGrid,
  IconLogout,
  IconMail,
  IconShieldLock,
  IconUser,
  IconUserCircle,
} from '@tabler/icons-react'
import { Link } from '@tanstack/react-router'
import { useEffect, type ReactNode } from 'react'
import { logout } from '../api'
import { cn } from '../lib/utils'
import { preloadPageChunks } from '../router'
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
  | 'apps'
  | 'profile'
  | 'emails'
  | 'security'
  | 'activity'
  | 'applications'
  | 'data'

const navItems: { key: AccountNavKey; label: string; href: string; icon: typeof IconUser }[] = [
  { key: 'home', label: 'ホーム', href: '/account', icon: IconUserCircle },
  { key: 'apps', label: 'アプリ', href: '/account/apps', icon: IconLayoutGrid },
  { key: 'profile', label: 'アカウント情報', href: '/account/profile', icon: IconUser },
  { key: 'emails', label: 'メールアドレス', href: '/account/emails', icon: IconMail },
  { key: 'security', label: 'セキュリティ', href: '/account/security', icon: IconShieldLock },
  { key: 'activity', label: 'アクティビティ', href: '/account/activity', icon: IconHistory },
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
  // アカウントポータルに入ったら全ページ chunk をバックグラウンド先読みし、遷移の空白を防ぐ。
  useEffect(() => {
    preloadPageChunks()
  }, [])
  return (
    <div className="app-surface">
      <header className="app-header">
        <div className="flex h-16 items-center justify-between px-5 lg:px-7">
          <div className="flex items-center gap-5">
            <Link
              to="/account"
              aria-label="アカウント"
              className="rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600/30"
            >
              <Brand compact />
            </Link>
            <div className="hidden h-6 w-px bg-slate-200/80 sm:block" />
            <span className="hidden items-center gap-2 rounded-lg border border-slate-200/80 bg-white/70 px-2.5 py-1.5 text-sm font-medium text-slate-600 shadow-xs sm:flex">
              <IconShieldLock size={16} className="text-slate-400" aria-hidden="true" />
              マイページ
            </span>
          </div>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                className="flex items-center gap-3 rounded-lg px-2 py-1.5 text-left transition-colors hover:bg-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600/30"
                aria-label="アカウントメニュー"
              >
                <div className="hidden text-right sm:block">
                  <p className="text-sm font-semibold text-slate-800">{username}</p>
                  <p className="text-xs text-slate-500">サインイン中</p>
                </div>
                <span className="flex size-9 items-center justify-center rounded-lg bg-slate-950 text-sm font-semibold text-white shadow-sm">
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
                  {/* 認証オーディエンス境界をまたぐため preload を無効化する。
                      intent preload で /admin loader が走ると admin セッション未確立時に
                      OIDC ログインへ画面遷移してしまう (hover だけで遷移する不具合)。 */}
                  <Link to="/admin" preload={false}>
                    <IconShieldLock size={17} aria-hidden="true" />
                    管理コンソール
                  </Link>
                </DropdownMenuItem>
              ) : null}
              <DropdownMenuItem asChild>
                <button
                  type="button"
                  onClick={() => {
                    void logout('account')
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
          <nav className="flex flex-1 flex-col gap-1 p-4" aria-label="マイページメニュー">
            {navItems.map((item) => (
              <Link
                key={item.key}
                to={item.href}
                className={cn(
                  'flex h-10 w-full items-center gap-3 rounded-lg px-3 text-left text-sm font-medium transition-[background-color,color,box-shadow]',
                  item.key === active
                    ? 'bg-slate-950 text-white shadow-sm'
                    : 'text-slate-600 hover:bg-white hover:text-slate-950 hover:shadow-xs',
                )}
                aria-current={item.key === active ? 'page' : undefined}
              >
                <item.icon size={18} stroke={1.8} aria-hidden="true" />
                {item.label}
              </Link>
            ))}
          </nav>
        </aside>

        <main className="app-main">
          <div className="app-content max-w-[920px]">
            <div>
              <h1 className="app-page-title">{title}</h1>
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

import {
  IconCircleCheckFilled,
  IconHelpCircle,
  IconLock,
  IconShieldCheck,
} from '@tabler/icons-react'
import type { ReactNode } from 'react'
import { Brand } from './Brand'

type AuthShellProps = {
  children: ReactNode
  asideTitle?: string
  asideText?: string
}

const assuranceItems = [
  '標準準拠のシングルサインオン',
  'アカウント情報の安全な取り扱い',
  '組織ポリシーに基づくアクセス制御',
]

export function AuthShell({
  children,
  asideTitle = '組織のすべてのサービスへ、安全なひとつの入口から。',
  asideText = 'RA Identity は、利用者とアプリケーションを保護するエンタープライズ認証基盤です。',
}: AuthShellProps) {
  return (
    <div className="auth-background">
      <div className="auth-container">
        <div className="auth-frame">
          <aside className="auth-aside">
            <Brand inverse />

            <div className="auth-aside-copy">
              <div className="flex w-fit items-center gap-2 rounded-full border border-blue-300/20 bg-blue-300/10 px-3 py-1.5 text-xs font-semibold text-blue-100">
                <IconShieldCheck size={15} aria-hidden="true" />
                Enterprise identity platform
              </div>
              <div className="flex flex-col gap-4">
                <h1 className="aside-title">{asideTitle}</h1>
                <p className="aside-text">{asideText}</p>
              </div>
              <ul className="flex flex-col gap-3" aria-label="セキュリティ機能">
                {assuranceItems.map((item) => (
                  <li key={item} className="flex items-center gap-3 text-sm text-slate-200">
                    <IconCircleCheckFilled
                      size={17}
                      className="shrink-0 text-blue-300"
                      aria-hidden="true"
                    />
                    {item}
                  </li>
                ))}
              </ul>
            </div>

            <div className="flex items-center justify-between border-t border-white/10 pt-5 text-xs text-slate-400">
              <span className="flex items-center gap-2">
                <IconLock size={14} aria-hidden="true" />
                保護された接続
              </span>
              <span>OpenID Connect / OAuth 2.0</span>
            </div>
          </aside>

          <main className="auth-main">
            <div className="mobile-brand text-slate-950">
              <Brand />
            </div>
            <div className="mb-7 flex items-center justify-between border-b border-slate-100 pb-4">
              <span className="flex items-center gap-2 text-xs font-semibold text-slate-600">
                <IconLock size={14} className="text-emerald-600" aria-hidden="true" />
                セキュアな認証
              </span>
              <span className="rounded-full bg-emerald-50 px-2.5 py-1 text-[0.68rem] font-bold uppercase tracking-[0.08em] text-emerald-700">
                Protected
              </span>
            </div>

            {children}

            <footer className="mt-9 flex flex-wrap items-center justify-center gap-x-3 gap-y-2 border-t border-slate-100 pt-5 text-xs text-slate-500">
              <a
                href="/.well-known/openid-configuration"
                className="rounded-sm font-medium transition-colors hover:text-slate-950 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600/30"
              >
                Provider information
              </a>
              <span className="text-slate-300" aria-hidden="true">
                •
              </span>
              <span className="flex items-center gap-1.5">
                <IconHelpCircle size={14} aria-hidden="true" />
                問題がある場合は管理者へお問い合わせください
              </span>
            </footer>
          </main>
        </div>
      </div>
    </div>
  )
}

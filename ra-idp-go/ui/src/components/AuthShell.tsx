import {
  IconCircleCheckFilled,
  IconHelpCircle,
  IconLock,
  IconSparkles,
} from '@tabler/icons-react'
import type { ReactNode } from 'react'
import { Brand } from './Brand'

type AuthShellProps = {
  children: ReactNode
  asideTitle?: string
  asideText?: string
  // aside=false で左のプロモ枠を出さない (認証済み self-service 画面向け)。
  aside?: boolean
}

const assuranceItems = [
  '継続的に検証されるサインイン',
  '最小権限に基づくアクセス制御',
  '監査可能なアイデンティティ操作',
]

export function AuthShell({
  children,
  asideTitle = '組織のすべてのサービスへ、安全なひとつの入口から。',
  asideText = 'RA Identity は、利用者とアプリケーションを保護するエンタープライズ認証基盤です。',
  aside = true,
}: AuthShellProps) {
  return (
    <div className="auth-background">
      <div className="auth-container">
        <div className={aside ? 'auth-frame' : 'auth-frame auth-frame--solo'}>
          {aside ? (
            <aside className="auth-aside">
              <Brand inverse />

              <div className="auth-aside-copy">
                <div className="flex w-fit items-center gap-2 rounded-lg border border-white/12 bg-white/8 px-3 py-1.5 text-xs font-semibold text-blue-100 shadow-sm backdrop-blur-sm">
                  <IconSparkles size={15} aria-hidden="true" />
                  AI-era identity control
                </div>
                <div className="flex flex-col gap-4">
                  <h1 className="aside-title">{asideTitle}</h1>
                  <p className="aside-text">{asideText}</p>
                </div>
                <ul className="grid gap-3" aria-label="セキュリティ機能">
                  {assuranceItems.map((item) => (
                    <li
                      key={item}
                      className="flex items-center gap-3 rounded-lg border border-white/8 bg-white/6 px-3 py-2.5 text-sm text-slate-200 backdrop-blur-sm"
                    >
                      <IconCircleCheckFilled
                        size={17}
                        className="shrink-0 text-teal-300"
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
          ) : null}

          <main className="auth-main">
            <div className="mobile-brand text-slate-950">
              <Brand />
            </div>
            <div className="mb-7 flex items-center justify-between border-b border-slate-100 pb-4">
              <span className="flex items-center gap-2 text-xs font-semibold text-slate-600">
                <IconLock size={14} className="text-emerald-600" aria-hidden="true" />
                セキュアな認証
              </span>
              <span className="rounded-md bg-emerald-50 px-2.5 py-1 text-[0.68rem] font-bold uppercase tracking-normal text-emerald-700">
                Protected
              </span>
            </div>

            {children}

            <footer className="mt-9 flex flex-wrap items-center justify-center gap-x-3 gap-y-2 border-t border-slate-100 pt-5 text-xs text-slate-500">
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

import { IconLock } from '@tabler/icons-react'
import type { ReactNode } from 'react'
import { Brand } from './Brand'

type AuthShellProps = {
  children: ReactNode
  asideTitle?: string
  asideText?: string
}

export function AuthShell({
  children,
  asideTitle = 'ひとつの安全な入口から、すべてのサービスへ。',
  asideText = '標準準拠の認証基盤が、アカウントとアプリケーションを保護します。',
}: AuthShellProps) {
  return (
    <div className="auth-background">
      <div className="auth-container">
        <div className="auth-frame">
          <aside className="auth-aside">
            <Brand />
            <div className="auth-aside-copy">
              <p className="eyebrow !text-cyan-200">Regenerative Architecture</p>
              <h1 className="aside-title">{asideTitle}</h1>
              <p className="aside-text">{asideText}</p>
            </div>
            <div className="flex items-center gap-2 text-white/70">
              <IconLock size={16} />
              <span className="text-sm">OpenID Connect / OAuth 2.0</span>
            </div>
          </aside>

          <main className="auth-main">
            <div className="mobile-brand">
              <Brand />
            </div>
            {children}
            <div className="mt-8 flex items-center justify-center gap-2 text-xs text-slate-400">
              <a
                href="/.well-known/openid-configuration"
                className="transition-colors hover:text-slate-700"
              >
                Provider information
              </a>
              <span aria-hidden="true">•</span>
              <span>Privacy protected</span>
            </div>
          </main>
        </div>
      </div>
    </div>
  )
}

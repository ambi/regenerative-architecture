import { IconArrowRight, IconInfoCircle } from '@tabler/icons-react'
import { startDemoAuthorization } from '../api'
import { AuthShell } from '../components/AuthShell'
import { Button } from '../components/ui/button'
import type { HomePage as HomePageData } from '../types'

export function HomePage({ demoEnabled }: HomePageData) {
  return (
    <AuthShell>
      <div className="flex flex-col gap-7 py-4">
        <header className="flex flex-col gap-2.5">
          <p className="eyebrow">IDプロバイダー</p>
          <h2 className="page-title">RA Identity は起動しています</h2>
          <p className="page-description">
            ログイン画面は、接続するアプリケーションから認証要求を受けたときに表示されます。
          </p>
        </header>

        <div className="flex items-start gap-3 rounded-xl border border-blue-100 bg-blue-50 p-4 text-sm leading-6 text-blue-950">
          <IconInfoCircle className="mt-0.5 shrink-0 text-blue-700" size={18} aria-hidden="true" />
          <p>
            <code>/login</code> を直接開くことはできません。OAuth 2.0 / OpenID Connect
            クライアントから <code>/authorize</code> を開始してください。
          </p>
        </div>

        {demoEnabled ? (
          <Button size="lg" onClick={() => void startDemoAuthorization()}>
            ローカルデモ認証を開始
            <IconArrowRight size={18} aria-hidden="true" />
          </Button>
        ) : (
          <p className="text-sm leading-6 text-slate-600">
            利用するアプリケーションからログインを開始してください。
          </p>
        )}

        {demoEnabled && (
          <p className="text-center text-xs leading-5 text-slate-500">
            デモユーザー: <code>alice</code> / <code>demo-password-1234</code>
          </p>
        )}
      </div>
    </AuthShell>
  )
}

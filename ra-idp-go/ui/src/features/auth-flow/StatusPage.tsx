import {
  IconCheck,
  IconInfoCircle,
  IconLogin,
  IconLogout,
  IconX,
} from '@tabler/icons-react'
import { tenantURL } from '../../api'
import { AuthShell } from '../../components/AuthShell'
import { Button } from '../../components/ui/button'
import { cn } from '../../lib/utils'

const content = {
  approved: {
    eyebrow: 'Connection approved',
    title: 'デバイスを承認しました',
    text: '認証が完了しました。このウィンドウを閉じて、接続したデバイスに戻ってください。',
    note: 'この操作に心当たりがない場合は、すぐに管理者へ連絡してください。',
    icon: IconCheck,
    color: 'border-emerald-100 bg-emerald-50 text-emerald-700',
  },
  denied: {
    eyebrow: 'Connection denied',
    title: '接続を拒否しました',
    text: 'デバイスへの接続を拒否しました。アカウント情報は共有されていません。',
    note: '同じ要求が繰り返される場合は、管理者へ連絡してください。',
    icon: IconX,
    color: 'border-slate-200 bg-slate-100 text-slate-700',
  },
  'signed-out': {
    eyebrow: 'Signed out',
    title: 'ログアウトしました',
    text: 'RA Identity のセッションを安全に終了しました。',
    note: '共有端末では、このブラウザを閉じることをおすすめします。',
    icon: IconLogout,
    color: 'border-blue-100 bg-blue-50 text-blue-700',
  },
  'authentication-required': {
    eyebrow: 'Authentication required',
    title: 'ログインが必要です',
    text: 'デバイスを承認するには、先に認証フローからログインしてください。',
    note: '元のアプリケーションまたはデバイスに戻り、接続操作を最初からやり直してください。',
    icon: IconLogin,
    color: 'border-amber-200 bg-amber-50 text-amber-700',
  },
} as const

type StatusKey = keyof typeof content

export function StatusPage({ status }: { status: StatusKey }) {
  const state = content[status]
  const StatusIcon = state.icon

  return (
    <AuthShell>
      <div className="flex flex-col items-center gap-7 py-4 text-center">
        <div
          className={cn(
            'flex size-16 items-center justify-center rounded-2xl border shadow-xs',
            state.color,
          )}
        >
          <StatusIcon size={30} stroke={2} aria-hidden="true" />
        </div>

        <header className="flex max-w-md flex-col items-center gap-2.5">
          <p className="eyebrow">{state.eyebrow}</p>
          <h2 className="page-title">{state.title}</h2>
          <p className="page-description">{state.text}</p>
        </header>

        <div className="flex w-full items-start gap-3 rounded-xl bg-slate-50 p-3.5 text-left text-xs leading-5 text-slate-600">
          <IconInfoCircle className="mt-0.5 shrink-0 text-slate-500" size={17} aria-hidden="true" />
          <p>{state.note}</p>
        </div>

        {status === 'signed-out' ? (
          <div className="grid w-full gap-2">
            <Button asChild className="w-full">
              <a href={tenantURL('/account')}>マイページにログイン</a>
            </Button>
            <Button asChild variant="outline" className="w-full">
              <a href={tenantURL('/admin')}>管理コンソールにログイン</a>
            </Button>
          </div>
        ) : null}
      </div>
    </AuthShell>
  )
}

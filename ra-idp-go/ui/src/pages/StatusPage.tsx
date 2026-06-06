import { IconCheck, IconLogin, IconLogout, IconX } from '@tabler/icons-react'
import { AuthShell } from '../components/AuthShell'
import { Button } from '../components/ui/button'
import { cn } from '../lib/utils'
import type { StatusPage as StatusPageData } from '../types'

const content = {
  approved: {
    title: 'デバイスを承認しました',
    text: 'この画面を閉じて、デバイスに戻ることができます。',
    icon: IconCheck,
    color: 'bg-teal-50 text-teal-600',
  },
  denied: {
    title: '接続を拒否しました',
    text: 'デバイスにはアカウント情報は共有されていません。',
    icon: IconX,
    color: 'bg-slate-100 text-slate-600',
  },
  'signed-out': {
    title: 'ログアウトしました',
    text: 'セッションは安全に終了しました。',
    icon: IconLogout,
    color: 'bg-indigo-50 text-indigo-600',
  },
  'authentication-required': {
    title: 'ログインが必要です',
    text: 'デバイスを承認する前に、認証フローからログインしてください。',
    icon: IconLogin,
    color: 'bg-orange-50 text-orange-600',
  },
} as const

export function StatusPage({ status }: StatusPageData) {
  const state = content[status]
  const StatusIcon = state.icon

  return (
    <AuthShell>
      <div className="flex flex-col items-center gap-8 py-8 text-center">
        <div
          className={cn('flex size-[68px] items-center justify-center rounded-full', state.color)}
        >
          <StatusIcon size={34} />
        </div>
        <div>
          <h2 className="text-2xl font-bold tracking-tight">{state.title}</h2>
          <p className="mt-2 text-slate-500">{state.text}</p>
        </div>
        <Button asChild variant="secondary">
          <a href="/.well-known/openid-configuration">プロバイダー情報</a>
        </Button>
      </div>
    </AuthShell>
  )
}

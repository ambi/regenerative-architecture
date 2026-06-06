import {
  IconArrowRight,
  IconCheck,
  IconId,
  IconMail,
  IconShieldCheck,
  IconUser,
  IconX,
} from '@tabler/icons-react'
import { AuthShell } from '../components/AuthShell'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import type { ConsentPage as ConsentPageData } from '../types'

const scopeDetails: Record<string, { label: string; description: string; icon: typeof IconId }> = {
  openid: { label: '本人確認', description: '一意のアカウントIDを確認します', icon: IconId },
  profile: {
    label: '基本プロフィール',
    description: '名前とプロフィール情報を参照します',
    icon: IconUser,
  },
  email: {
    label: 'メールアドレス',
    description: '登録済みメールアドレスを参照します',
    icon: IconMail,
  },
}

export function ConsentPage({ requestId, clientName, scope }: ConsentPageData) {
  const scopes = scope.split(/\s+/).filter(Boolean)

  return (
    <AuthShell
      asideTitle="共有する情報は、いつでもあなたが決められます。"
      asideText="アプリケーションには、許可した情報だけが安全に共有されます。"
    >
      <div className="flex flex-col gap-6">
        <div className="flex flex-col items-center gap-3">
          <div className="flex size-[58px] items-center justify-center rounded-2xl bg-indigo-50 font-bold text-indigo-700">
            {clientName.slice(0, 2).toUpperCase()}
          </div>
          <div className="text-center">
            <h2 className="text-2xl font-bold tracking-tight">{clientName}</h2>
            <p className="mt-1 text-slate-500">このアプリがアカウントへのアクセスを求めています</p>
          </div>
        </div>

        <Card className="p-4">
          <div className="flex flex-col gap-4">
            <div className="flex items-center gap-3">
              <div className="flex size-9 items-center justify-center rounded-full bg-teal-50 text-teal-600">
                <IconShieldCheck size={18} />
              </div>
              <p className="font-semibold">許可する内容</p>
            </div>
            {scopes.map((scopeName) => {
              const detail = scopeDetails[scopeName] ?? {
                label: scopeName,
                description: 'このアプリが要求する追加の権限です',
                icon: IconCheck,
              }
              const ScopeIcon = detail.icon
              return (
                <div key={scopeName} className="flex items-start gap-3">
                  <div className="flex size-9 shrink-0 items-center justify-center text-slate-500">
                    <ScopeIcon size={19} />
                  </div>
                  <div>
                    <p className="text-sm font-semibold">{detail.label}</p>
                    <p className="text-xs text-slate-500">{detail.description}</p>
                  </div>
                </div>
              )
            })}
          </div>
        </Card>

        <form method="POST" action="/consent">
          <input type="hidden" name="request_id" value={requestId} />
          <div className="flex flex-col gap-2">
            <Button type="submit" name="action" value="allow" size="lg">
              アクセスを許可
              <IconArrowRight size={18} />
            </Button>
            <Button type="submit" name="action" value="deny" size="lg" variant="ghost">
              <IconX size={17} />
              キャンセル
            </Button>
          </div>
        </form>
      </div>
    </AuthShell>
  )
}

import {
  IconArrowRight,
  IconCheck,
  IconClock,
  IconId,
  IconMail,
  IconRefresh,
  IconShieldCheck,
  IconUser,
  IconX,
} from '@tabler/icons-react'
import { useState } from 'react'
import { AuthenticationAPIError, continueBrowserFlow, submitConsent } from '../../api'
import { AuthShell } from '../../components/AuthShell'
import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import type { ConsentDetailView } from '../../types'

const scopeDetails: Record<string, { label: string; description: string; icon: typeof IconId }> = {
  openid: { label: 'アカウント識別子', description: '本人確認に必要な一意のID', icon: IconId },
  profile: {
    label: '基本プロフィール',
    description: '名前、表示名などのプロフィール情報',
    icon: IconUser,
  },
  email: {
    label: 'メールアドレス',
    description: '登録済みメールアドレスと確認状態',
    icon: IconMail,
  },
  offline_access: {
    label: '継続的なアクセス',
    description: 'サインインしていない間も許可した範囲でアクセス',
    icon: IconRefresh,
  },
}

export function ConsentPage({
  csrfToken,
  clientName,
  scopes,
  authorizationDetails = [],
}: {
  csrfToken: string
  clientName: string
  scopes: string[]
  authorizationDetails?: ConsentDetailView[]
}) {
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  async function handleConsent(action: 'allow' | 'deny') {
    setSubmitting(true)
    setError('')
    try {
      continueBrowserFlow(await submitConsent(csrfToken, action))
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : 'アクセス要求を処理できませんでした。',
      )
      setSubmitting(false)
    }
  }

  return (
    <AuthShell
      asideTitle="データ共有は、必要な範囲だけを明確に。"
      asideText="アクセス要求の内容を確認し、信頼できるアプリケーションにだけ権限を付与してください。"
    >
      <div className="flex flex-col gap-6">
        <header className="flex flex-col gap-2">
          <p className="eyebrow">アクセス要求</p>
          <h2 className="page-title">アクセスを許可しますか？</h2>
          <p className="page-description">要求元と共有される情報を確認してください。</p>
        </header>

        <Card className="overflow-hidden">
          <div className="flex items-center gap-4 border-b border-slate-200 bg-slate-50/70 p-4">
            <div className="flex size-12 shrink-0 items-center justify-center rounded-xl border border-blue-100 bg-blue-50 text-sm font-bold text-blue-700">
              {clientName.slice(0, 2).toUpperCase()}
            </div>
            <div className="min-w-0">
              <p className="text-xs font-medium text-slate-500">要求元アプリケーション</p>
              <p className="truncate font-semibold text-slate-950">{clientName}</p>
            </div>
            <span className="ml-auto flex shrink-0 items-center gap-1.5 rounded-full bg-emerald-50 px-2.5 py-1 text-[0.68rem] font-bold text-emerald-700">
              <IconShieldCheck size={13} aria-hidden="true" />
              登録済み
            </span>
          </div>

          <div className="p-4">
            <p className="mb-3 text-xs font-bold uppercase tracking-[0.09em] text-slate-500">
              共有される情報
            </p>
            <div className="divide-y divide-slate-100">
              {scopes.map((scopeName) => {
                const detail = scopeDetails[scopeName] ?? {
                  label: scopeName,
                  description: 'このアプリケーションが要求する追加権限',
                  icon: IconCheck,
                }
                const ScopeIcon = detail.icon
                return (
                  <div key={scopeName} className="flex items-start gap-3 py-3 first:pt-1 last:pb-1">
                    <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-slate-100 text-slate-600">
                      <ScopeIcon size={18} aria-hidden="true" />
                    </div>
                    <div className="pt-0.5">
                      <p className="text-sm font-semibold text-slate-900">{detail.label}</p>
                      <p className="mt-0.5 text-xs leading-5 text-slate-500">
                        {detail.description}
                      </p>
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        </Card>

        {authorizationDetails.length > 0 ? (
          <Card className="overflow-hidden border-blue-200/70">
            <div className="border-b border-slate-200 bg-blue-50/50 p-4">
              <p className="text-xs font-bold uppercase tracking-[0.09em] text-blue-700">
                細粒度の権限
              </p>
              <p className="mt-0.5 text-xs leading-5 text-slate-500">
                以下の対象・上限に限定して許可します。
              </p>
            </div>
            <div className="divide-y divide-slate-100 p-4">
              {authorizationDetails.map((detail) => (
                <div
                  key={`${detail.type}-${detail.summary}`}
                  className="flex items-start gap-3 py-3 first:pt-1 last:pb-1"
                >
                  <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-blue-100 text-blue-700">
                    <IconShieldCheck size={18} aria-hidden="true" />
                  </div>
                  <div className="min-w-0 pt-0.5">
                    <p className="text-sm font-semibold text-slate-900">
                      {detail.description || detail.type}
                    </p>
                    <p className="mt-0.5 text-xs leading-5 text-slate-700">{detail.summary}</p>
                    {detail.lines && detail.lines.length > 0 ? (
                      <ul className="mt-1.5 flex flex-col gap-0.5 text-[0.7rem] leading-5 text-slate-500">
                        {detail.lines.map((line) => (
                          <li key={line} className="font-mono">
                            {line}
                          </li>
                        ))}
                      </ul>
                    ) : null}
                  </div>
                </div>
              ))}
            </div>
          </Card>
        ) : null}

        <div className="flex gap-3 rounded-xl border border-amber-200/80 bg-amber-50/70 p-3.5 text-xs leading-5 text-amber-950">
          <IconClock className="mt-0.5 shrink-0 text-amber-700" size={17} aria-hidden="true" />
          <p>許可は組織のポリシーに従って保存され、後から管理者またはアプリ側で取り消せます。</p>
        </div>

        {error ? (
          <p role="alert" className="text-sm font-medium text-red-700">
            {error}
          </p>
        ) : null}

        <div className="flex flex-col gap-2.5">
          <Button
            type="button"
            size="lg"
            disabled={submitting}
            onClick={() => handleConsent('allow')}
          >
            {submitting ? '処理しています…' : '許可して続行'}
            <IconArrowRight size={18} aria-hidden="true" />
          </Button>
          <Button
            type="button"
            size="lg"
            variant="ghost"
            disabled={submitting}
            onClick={() => handleConsent('deny')}
          >
            <IconX size={17} aria-hidden="true" />
            許可しない
          </Button>
        </div>
      </div>
    </AuthShell>
  )
}

import {
  IconAlertTriangle,
  IconArrowRight,
  IconClockHour4,
  IconKey,
  IconShieldCheck,
  IconShieldOff,
  IconUser,
} from '@tabler/icons-react'
import type { ReactNode } from 'react'
import { tenantURL } from '../api'
import { AccountShell } from '../components/AccountShell'
import { Card } from '../components/ui/card'
import { type AccountHomePage as PageProps, requiredActionLabel } from '../types'

function formatDateTime(value: string | undefined): string {
  if (!value) {
    return '記録なし'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return '記録なし'
  }
  return date.toLocaleString('ja-JP', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function AccountHomePage({ summary, isAdmin }: PageProps) {
  const displayName = summary.name?.trim() || summary.preferred_username
  return (
    <AccountShell
      active="home"
      username={summary.preferred_username}
      isAdmin={isAdmin}
      title={`こんにちは、${displayName} さん`}
      description="アカウントの状態を確認し、個人情報やパスワードを管理できます。"
    >
      {summary.required_actions.length > 0 ? (
        <Card className="flex items-start gap-3 border-amber-200 bg-amber-50/70 p-4">
          <IconAlertTriangle className="mt-0.5 shrink-0 text-amber-600" size={20} aria-hidden="true" />
          <div>
            <p className="text-sm font-semibold text-amber-900">対応が必要な項目があります</p>
            <ul className="mt-1.5 flex flex-wrap gap-2">
              {summary.required_actions.map((action) => (
                <li
                  key={action}
                  className="rounded-md bg-amber-100 px-2 py-1 text-xs font-medium text-amber-900"
                >
                  {requiredActionLabel(action)}
                </li>
              ))}
            </ul>
          </div>
        </Card>
      ) : null}

      <section className="grid gap-4 sm:grid-cols-2" aria-label="アカウント概要">
        <SummaryCard
          icon={summary.mfa_enrolled ? <IconShieldCheck size={20} /> : <IconShieldOff size={20} />}
          tone={summary.mfa_enrolled ? 'ok' : 'warn'}
          label="二要素認証 (MFA)"
          value={summary.mfa_enrolled ? '登録済み' : '未登録'}
        />
        <SummaryCard
          icon={<IconUser size={20} />}
          tone="neutral"
          label="メールアドレス"
          value={summary.email ?? '未設定'}
          hint={summary.email ? (summary.email_verified ? '確認済み' : '未確認') : undefined}
        />
        <SummaryCard
          icon={<IconClockHour4 size={20} />}
          tone="neutral"
          label="最終ログイン"
          value={formatDateTime(summary.last_login_at)}
        />
        <SummaryCard
          icon={<IconKey size={20} />}
          tone="neutral"
          label="パスワード最終変更"
          value={formatDateTime(summary.password_changed_at)}
        />
      </section>

      <section className="grid gap-3 sm:grid-cols-2" aria-label="操作">
        <ActionLink
          href={tenantURL('/account/profile')}
          title="個人情報を編集"
          description="表示名やプロフィール属性を更新します。"
        />
        <ActionLink
          href={tenantURL('/account/password')}
          title="パスワードを変更"
          description="サインインに使うパスワードを変更します。"
        />
      </section>
    </AccountShell>
  )
}

function SummaryCard({
  icon,
  tone,
  label,
  value,
  hint,
}: {
  icon: ReactNode
  tone: 'ok' | 'warn' | 'neutral'
  label: string
  value: string
  hint?: string
}) {
  const toneClass =
    tone === 'ok'
      ? 'bg-emerald-50 text-emerald-700'
      : tone === 'warn'
        ? 'bg-amber-50 text-amber-700'
        : 'bg-slate-100 text-slate-600'
  return (
    <Card className="flex items-start gap-3 p-5">
      <span className={`flex size-10 shrink-0 items-center justify-center rounded-lg ${toneClass}`}>
        {icon}
      </span>
      <div className="min-w-0">
        <p className="text-xs font-medium text-slate-500">{label}</p>
        <p className="mt-1 truncate text-sm font-semibold text-slate-900">{value}</p>
        {hint ? <p className="mt-0.5 text-xs text-slate-500">{hint}</p> : null}
      </div>
    </Card>
  )
}

function ActionLink({
  href,
  title,
  description,
}: {
  href: string
  title: string
  description: string
}) {
  return (
    <a
      href={href}
      className="group flex items-center justify-between gap-3 rounded-xl border border-slate-200 bg-white p-4 transition-colors hover:border-blue-300 hover:bg-blue-50/40"
    >
      <div>
        <p className="text-sm font-semibold text-slate-900">{title}</p>
        <p className="mt-0.5 text-xs text-slate-500">{description}</p>
      </div>
      <IconArrowRight
        size={18}
        className="shrink-0 text-slate-400 transition-colors group-hover:text-blue-600"
        aria-hidden="true"
      />
    </a>
  )
}

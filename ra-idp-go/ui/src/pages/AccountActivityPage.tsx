import { IconDeviceLaptop, IconInfoCircle, IconLogin2 } from '@tabler/icons-react'
import { AccountShell } from '../components/AccountShell'
import { Card } from '../components/ui/card'
import type { AccountActivityPage as PageProps, AccountSignInActivity } from '../types'

function formatDateTime(value: string): string {
  return new Date(value).toLocaleString('ja-JP', { dateStyle: 'medium', timeStyle: 'short' })
}

const amrLabels: Record<string, string> = {
  pwd: 'パスワード',
  otp: '認証アプリ (TOTP)',
  mfa: '多要素認証',
  hwk: 'ハードウェアキー',
  swk: 'ソフトウェアキー',
}

function methodSummary(amr: string[]): string {
  if (amr.length === 0) return '不明な手段'
  return amr.map((code) => amrLabels[code] ?? code).join(' + ')
}

function ActivityRow({ activity }: { activity: AccountSignInActivity }) {
  return (
    <li className="flex items-start gap-3 px-5 py-4">
      <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-slate-100 text-slate-600">
        <IconLogin2 size={18} aria-hidden="true" />
      </span>
      <div className="min-w-0">
        <p className="text-sm font-semibold text-slate-900">サインイン</p>
        <p className="mt-0.5 text-sm text-slate-600">{methodSummary(activity.amr)}</p>
        <p className="mt-1 text-xs text-slate-500">{formatDateTime(activity.occurred_at)}</p>
      </div>
    </li>
  )
}

export function AccountActivityPage({ username, isAdmin, activities }: PageProps) {
  return (
    <AccountShell
      active="activity"
      username={username}
      isAdmin={isAdmin}
      title="アクティビティ"
      description="最近のサインイン履歴を確認できます。"
    >
      <Card className="overflow-hidden p-0">
        {activities.length === 0 ? (
          <div className="flex items-center gap-3 px-5 py-8 text-sm text-slate-600">
            <IconDeviceLaptop size={20} className="text-slate-400" aria-hidden="true" />
            まだサインイン履歴がありません。
          </div>
        ) : (
          <ul className="divide-y divide-slate-100">
            {activities.map((activity) => (
              <ActivityRow key={`${activity.occurred_at}-${activity.amr.join('-')}`} activity={activity} />
            ))}
          </ul>
        )}
      </Card>

      <div className="flex items-start gap-3 rounded-xl bg-slate-50 p-3.5 text-xs leading-5 text-slate-600">
        <IconInfoCircle className="mt-0.5 shrink-0 text-slate-500" size={17} aria-hidden="true" />
        <p>
          現在は直近のサインイン日時と認証手段を表示します。IP アドレス・デバイス・場所の表示と、
          有効なセッションの一覧・終了は今後のステージで追加します。
        </p>
      </div>
    </AccountShell>
  )
}

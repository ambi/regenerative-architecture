import {
  IconDeviceDesktop,
  IconDeviceLaptop,
  IconInfoCircle,
  IconLogin2,
} from '@tabler/icons-react'
import { useState } from 'react'
import { AuthenticationAPIError, revokeAccountSession, revokeOtherAccountSessions } from '../../api'
import { AccountShell } from '../../components/AccountShell'
import { StepUpCancelledError, useStepUpGuard } from '../../components/StepUpDialog'
import { Alert } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import type { AccountSession, AccountSignInActivity } from '../../types'

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

function errorMessage(cause: unknown, fallback: string): string {
  return cause instanceof AuthenticationAPIError ? cause.message : fallback
}

function SessionRow({
  session,
  busy,
  onRevoke,
}: {
  session: AccountSession
  busy: boolean
  onRevoke: () => void
}) {
  return (
    <li className="flex items-start justify-between gap-3 px-5 py-4">
      <div className="flex min-w-0 items-start gap-3">
        <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-slate-100 text-slate-600">
          <IconDeviceDesktop size={18} aria-hidden="true" />
        </span>
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <p className="text-sm font-semibold text-slate-900">セッション</p>
            {session.current ? (
              <span className="inline-flex items-center rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700">
                現在のセッション
              </span>
            ) : null}
          </div>
          <p className="mt-0.5 text-sm text-slate-600">{methodSummary(session.amr)}</p>
          <p className="mt-1 text-xs text-slate-500">開始: {formatDateTime(session.started_at)}</p>
        </div>
      </div>
      {session.current ? (
        <span className="shrink-0 self-center text-xs text-slate-400">このデバイス</span>
      ) : (
        <Button
          type="button"
          variant="outline"
          className="h-9 shrink-0 self-center px-3 text-xs"
          disabled={busy}
          onClick={onRevoke}
        >
          {busy ? '終了中…' : '終了'}
        </Button>
      )}
    </li>
  )
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

export function AccountActivityPage({
  csrfToken,
  username,
  isAdmin,
  activities,
  sessions: initialSessions,
}: {
  csrfToken: string
  username: string
  activities: AccountSignInActivity[]
  sessions: AccountSession[]
  isAdmin: boolean
}) {
  const [sessions, setSessions] = useState(initialSessions)
  const [busyId, setBusyId] = useState<string | null>(null)
  const [busyOthers, setBusyOthers] = useState(false)
  const [error, setError] = useState('')
  const { guard, dialog } = useStepUpGuard(csrfToken)

  const otherCount = sessions.filter((session) => !session.current).length

  async function handleRevoke(id: string) {
    setBusyId(id)
    setError('')
    try {
      await revokeAccountSession(csrfToken, id)
      setSessions((current) => current.filter((session) => session.id !== id))
    } catch (cause) {
      setError(errorMessage(cause, 'セッションを終了できませんでした。'))
    } finally {
      setBusyId(null)
    }
  }

  async function handleRevokeOthers() {
    setBusyOthers(true)
    setError('')
    try {
      await guard(() => revokeOtherAccountSessions(csrfToken))
      setSessions((current) => current.filter((session) => session.current))
    } catch (cause) {
      if (cause instanceof StepUpCancelledError) return
      setError(errorMessage(cause, '他のセッションを終了できませんでした。'))
    } finally {
      setBusyOthers(false)
    }
  }

  return (
    <AccountShell
      active="activity"
      username={username}
      isAdmin={isAdmin}
      title="アクティビティ"
      description="有効なセッションと最近のサインイン履歴を確認できます。"
    >
      {error ? <Alert variant="destructive">{error}</Alert> : null}

      <section className="flex flex-col gap-3">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-semibold text-slate-900">有効なセッション</h2>
          {otherCount > 0 ? (
            <Button
              type="button"
              variant="outline"
              className="h-9 px-3 text-xs"
              disabled={busyOthers}
              onClick={handleRevokeOthers}
            >
              {busyOthers ? '終了中…' : '他のセッションを終了'}
            </Button>
          ) : null}
        </div>
        <Card className="overflow-hidden p-0">
          {sessions.length === 0 ? (
            <div className="flex items-center gap-3 px-5 py-8 text-sm text-slate-600">
              <IconDeviceLaptop size={20} className="text-slate-400" aria-hidden="true" />
              有効なセッションがありません。
            </div>
          ) : (
            <ul className="divide-y divide-slate-100">
              {sessions.map((session) => (
                <SessionRow
                  key={session.id}
                  session={session}
                  busy={busyId === session.id}
                  onRevoke={() => handleRevoke(session.id)}
                />
              ))}
            </ul>
          )}
        </Card>
      </section>

      <section className="flex flex-col gap-3">
        <h2 className="text-sm font-semibold text-slate-900">サインイン履歴</h2>
        <Card className="overflow-hidden p-0">
          {activities.length === 0 ? (
            <div className="flex items-center gap-3 px-5 py-8 text-sm text-slate-600">
              <IconDeviceLaptop size={20} className="text-slate-400" aria-hidden="true" />
              まだサインイン履歴がありません。
            </div>
          ) : (
            <ul className="divide-y divide-slate-100">
              {activities.map((activity) => (
                <ActivityRow
                  key={`${activity.occurred_at}-${activity.amr.join('-')}`}
                  activity={activity}
                />
              ))}
            </ul>
          )}
        </Card>
      </section>

      <div className="flex items-start gap-3 rounded-xl bg-slate-50 p-3.5 text-xs leading-5 text-slate-600">
        <IconInfoCircle className="mt-0.5 shrink-0 text-slate-500" size={17} aria-hidden="true" />
        <p>
          「終了」したセッションのブラウザは次回アクセス時に再ログインが必要になります。 IP
          アドレス・デバイス・場所の表示は今後のステージで追加します。
        </p>
      </div>
      {dialog}
    </AccountShell>
  )
}

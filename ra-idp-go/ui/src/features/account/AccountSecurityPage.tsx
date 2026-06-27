import {
  IconArrowRight,
  IconCircleCheck,
  IconDeviceMobile,
  IconKey,
  IconShieldLock,
} from '@tabler/icons-react'
import { QRCodeSVG } from 'qrcode.react'
import { type FormEvent, useState } from 'react'
import {
  AuthenticationAPIError,
  confirmTotpEnrollment,
  removeTotpFactor,
  startTotpEnrollment,
  tenantURL,
} from '../../api'
import { AccountShell } from '../../components/AccountShell'
import { StepUpCancelledError, useStepUpGuard } from '../../components/StepUpDialog'
import { Alert } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import { Input } from '../../components/ui/input'
import { Label } from '../../components/ui/label'
import type { AccountSecurity, TotpEnrollmentStart } from '../../types'

function formatDateTime(value?: string): string {
  if (!value) return '記録なし'
  return new Date(value).toLocaleString('ja-JP', { dateStyle: 'medium', timeStyle: 'short' })
}

function errorMessage(cause: unknown, fallback: string): string {
  return cause instanceof AuthenticationAPIError ? cause.message : fallback
}

export function AccountSecurityPage({
  csrfToken,
  username,
  isAdmin,
  security,
}: {
  csrfToken: string
  username: string
  isAdmin: boolean
  security: AccountSecurity
}) {
  const [enrolled, setEnrolled] = useState(security.totp_enrolled)
  const [enrollment, setEnrollment] = useState<TotpEnrollmentStart | null>(null)
  const [enrollCode, setEnrollCode] = useState('')
  const [removeCode, setRemoveCode] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const { guard, dialog } = useStepUpGuard(csrfToken)

  async function handleStart() {
    setBusy(true)
    setError('')
    setNotice('')
    try {
      setEnrollment(await startTotpEnrollment(csrfToken))
      setEnrollCode('')
    } catch (cause) {
      setError(errorMessage(cause, '認証アプリの登録を開始できませんでした。'))
    } finally {
      setBusy(false)
    }
  }

  async function handleConfirm(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!enrollment) return
    setBusy(true)
    setError('')
    try {
      await confirmTotpEnrollment(csrfToken, enrollment.secret, enrollCode.trim())
      setEnrolled(true)
      setEnrollment(null)
      setEnrollCode('')
      setNotice('認証アプリを登録しました。次回サインインから確認コードが必要になります。')
    } catch (cause) {
      setError(errorMessage(cause, '認証アプリを登録できませんでした。'))
    } finally {
      setBusy(false)
    }
  }

  async function handleRemove(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setBusy(true)
    setError('')
    try {
      await guard(() => removeTotpFactor(csrfToken, removeCode.trim()))
      setEnrolled(false)
      setRemoveCode('')
      setNotice('認証アプリを解除しました。')
    } catch (cause) {
      if (cause instanceof StepUpCancelledError) return
      setError(errorMessage(cause, '認証アプリを解除できませんでした。'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <AccountShell
      active="security"
      username={username}
      isAdmin={isAdmin}
      title="セキュリティ"
      description="パスワードと二段階認証 (認証アプリ) を管理します。"
    >
      {notice ? <Alert variant="success">{notice}</Alert> : null}
      {error ? <Alert variant="destructive">{error}</Alert> : null}

      <Card className="flex flex-col gap-4 p-5">
        <div className="flex items-start gap-3">
          <span className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-slate-100 text-slate-600">
            <IconKey size={20} aria-hidden="true" />
          </span>
          <div className="min-w-0">
            <p className="text-sm font-semibold text-slate-900">パスワード</p>
            <p className="mt-1 text-sm text-slate-600">
              最終変更: {formatDateTime(security.password_changed_at)}
            </p>
          </div>
        </div>
        <div>
          <Button asChild variant="outline">
            <a href={tenantURL('/account/password')}>
              パスワードを変更
              <IconArrowRight size={16} aria-hidden="true" />
            </a>
          </Button>
        </div>
      </Card>

      <Card className="flex flex-col gap-4 p-5">
        <div className="flex items-start gap-3">
          <span className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-slate-100 text-slate-600">
            <IconDeviceMobile size={20} aria-hidden="true" />
          </span>
          <div className="min-w-0">
            <p className="text-sm font-semibold text-slate-900">認証アプリ (TOTP)</p>
            <p className="mt-1 text-sm text-slate-600">
              Google Authenticator などの認証アプリで生成する確認コードを、サインインの
              二段階目に使います。
            </p>
            <span
              className={`mt-2 inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ${
                enrolled ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-100 text-slate-600'
              }`}
            >
              {enrolled ? <IconCircleCheck size={13} aria-hidden="true" /> : null}
              {enrolled ? '設定済み' : '未設定'}
            </span>
          </div>
        </div>

        {!enrolled && !enrollment ? (
          <div>
            <Button type="button" onClick={handleStart} disabled={busy}>
              {busy ? '準備中…' : '認証アプリを設定'}
            </Button>
          </div>
        ) : null}

        {!enrolled && enrollment ? (
          <form onSubmit={handleConfirm} className="grid gap-4 border-t border-slate-100 pt-4">
            <div className="flex flex-col items-center gap-3 border-b border-slate-100 pb-4">
              <p className="text-center text-sm text-slate-700">
                認証アプリ (Google Authenticator など) で、この QR コードをスキャンしてください。
              </p>
              <div className="rounded-xl border border-slate-200 bg-white p-3">
                <QRCodeSVG
                  value={enrollment.otpauth_uri}
                  size={176}
                  level="M"
                  marginSize={0}
                  title="認証アプリ登録用の QR コード"
                />
              </div>
            </div>
            <details className="rounded-lg bg-slate-50 px-3.5 py-3 text-sm text-slate-600">
              <summary className="cursor-pointer font-medium text-slate-700">
                QR コードをスキャンできない場合
              </summary>
              <div className="mt-3 grid gap-1.5">
                <Label htmlFor="totp-secret">セットアップキー</Label>
                <Input
                  id="totp-secret"
                  readOnly
                  value={enrollment.secret}
                  className="font-mono tracking-wider"
                  onFocus={(event) => event.target.select()}
                />
                <p className="mt-1 text-xs text-slate-500">
                  認証アプリに手動でこのキーを入力してください (時間ベース / 6 桁 / 30 秒)。
                </p>
                <p className="mt-2 break-all text-xs text-slate-400">{enrollment.otpauth_uri}</p>
              </div>
            </details>
            <div className="grid gap-1.5">
              <Label htmlFor="totp-code">認証アプリに表示された 6 桁コード</Label>
              <Input
                id="totp-code"
                inputMode="numeric"
                autoComplete="one-time-code"
                pattern="[0-9]{6}"
                maxLength={6}
                required
                placeholder="123456"
                value={enrollCode}
                className="font-mono tracking-[0.3em]"
                onChange={(event) => setEnrollCode(event.target.value.replace(/\D/g, ''))}
              />
            </div>
            <div className="flex gap-2">
              <Button type="submit" disabled={busy || enrollCode.trim().length !== 6}>
                {busy ? '登録中…' : '登録を完了'}
              </Button>
              <Button
                type="button"
                variant="ghost"
                disabled={busy}
                onClick={() => {
                  setEnrollment(null)
                  setEnrollCode('')
                  setError('')
                }}
              >
                キャンセル
              </Button>
            </div>
          </form>
        ) : null}

        {enrolled ? (
          <form onSubmit={handleRemove} className="grid gap-4 border-t border-slate-100 pt-4">
            <div className="grid gap-1.5">
              <Label htmlFor="remove-code">解除するには現在の 6 桁コードを入力</Label>
              <Input
                id="remove-code"
                inputMode="numeric"
                autoComplete="one-time-code"
                pattern="[0-9]{6}"
                maxLength={6}
                required
                placeholder="123456"
                value={removeCode}
                className="font-mono tracking-[0.3em]"
                onChange={(event) => setRemoveCode(event.target.value.replace(/\D/g, ''))}
              />
              <p className="text-xs text-slate-500">
                解除すると二段階認証が無効になります。共有端末では特に注意してください。
              </p>
            </div>
            <div>
              <Button
                type="submit"
                variant="destructive"
                disabled={busy || removeCode.trim().length !== 6}
              >
                {busy ? '解除中…' : '認証アプリを解除'}
              </Button>
            </div>
          </form>
        ) : null}
      </Card>

      <div className="flex items-start gap-3 rounded-xl bg-slate-50 p-3.5 text-xs leading-5 text-slate-600">
        <IconShieldLock className="mt-0.5 shrink-0 text-slate-500" size={17} aria-hidden="true" />
        <p>
          二段階認証を有効にすると、パスワードが漏れても認証アプリがなければサインインできません。
        </p>
      </div>
      {dialog}
    </AccountShell>
  )
}

import {
  IconAlertCircle,
  IconArrowLeft,
  IconArrowRight,
  IconCircleCheck,
  IconEye,
  IconEyeOff,
  IconLock,
  IconShieldLock,
} from '@tabler/icons-react'
import { type FormEvent, useState } from 'react'
import { AuthenticationAPIError, changePassword, PasswordPolicyError, tenantURL } from '../../api'
import { AuthShell } from '../../components/AuthShell'
import { StepUpCancelledError, useStepUpGuard } from '../../components/StepUpDialog'
import { Alert } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'
import { Input } from '../../components/ui/input'
import { Label } from '../../components/ui/label'

function violationMessage(violation: string): string {
  switch (violation) {
    case 'too_short':
      return 'パスワードが短すぎます。'
    case 'too_long':
      return 'パスワードが長すぎます。'
    default:
      return 'パスワードがセキュリティ要件を満たしていません。'
  }
}

export function ChangePasswordPage({
  csrfToken,
  preferredUsername,
  isAdmin,
}: {
  csrfToken: string
  preferredUsername: string
  isAdmin: boolean
}) {
  const backHref = isAdmin ? tenantURL('/admin') : tenantURL('/account/profile')
  const backLabel = isAdmin ? '管理コンソールへ戻る' : 'プロフィールへ戻る'
  const [showCurrent, setShowCurrent] = useState(false)
  const [showNew, setShowNew] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const { guard, dialog } = useStepUpGuard(csrfToken)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const formEl = event.currentTarget
    const form = new FormData(formEl)
    const current = String(form.get('current_password') ?? '')
    const next = String(form.get('new_password') ?? '')
    setSubmitting(true)
    setError('')
    setSuccess(false)
    try {
      await guard(() => changePassword(csrfToken, current, next))
      setSuccess(true)
      formEl.reset()
    } catch (cause) {
      if (cause instanceof StepUpCancelledError) return
      if (cause instanceof PasswordPolicyError) {
        setError(cause.violations.map(violationMessage).join(' ') || cause.message)
      } else if (cause instanceof AuthenticationAPIError) {
        switch (cause.code) {
          case 'access_denied':
            setError('現在のパスワードが一致しません。')
            break
          case 'password_reuse':
            setError('新しいパスワードは最近使用したものを再利用できません。')
            break
          case 'authentication_required':
            setError('認証セッションが切れています。ログインし直してください。')
            break
          default:
            setError(cause.message)
        }
      } else {
        setError('認証サービスに接続できませんでした。')
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <AuthShell aside={false}>
      <div className="flex flex-col gap-7">
        <header className="flex flex-col gap-2.5">
          <a
            href={backHref}
            className="inline-flex w-fit items-center gap-1 text-sm font-medium text-slate-500 hover:text-slate-800"
          >
            <IconArrowLeft size={15} aria-hidden="true" />
            {backLabel}
          </a>
          <p className="eyebrow">アカウントセキュリティ</p>
          <h2 className="page-title">パスワードを変更</h2>
          <p className="page-description">
            {preferredUsername
              ? `${preferredUsername} の現在のパスワードと新しいパスワードを入力してください。`
              : '現在のパスワードと新しいパスワードを入力してください。'}
          </p>
        </header>

        {success ? (
          <Alert className="flex gap-3 border-emerald-200 bg-emerald-50" aria-live="polite">
            <IconCircleCheck
              className="mt-0.5 shrink-0 text-emerald-600"
              size={19}
              aria-hidden="true"
            />
            <div>
              <p className="font-semibold text-emerald-900">パスワードを更新しました</p>
              <p className="mt-1 text-sm leading-5 text-emerald-900">
                次回のログインから新しいパスワードを使用してください。
              </p>
              <a
                href={backHref}
                className="mt-2 inline-flex items-center gap-1 text-sm font-semibold text-emerald-900 hover:underline"
              >
                <IconArrowLeft size={15} aria-hidden="true" />
                {backLabel}
              </a>
            </div>
          </Alert>
        ) : null}

        {error ? (
          <Alert className="flex gap-3" aria-live="polite">
            <IconAlertCircle
              className="mt-0.5 shrink-0 text-red-600"
              size={19}
              aria-hidden="true"
            />
            <div>
              <p className="font-semibold">変更できません</p>
              <p className="mt-1 text-sm leading-5 text-red-800">{error}</p>
            </div>
          </Alert>
        ) : null}

        <form onSubmit={handleSubmit}>
          <div className="flex flex-col gap-5">
            <div className="flex flex-col gap-2">
              <Label htmlFor="current_password">現在のパスワード</Label>
              <div className="relative">
                <IconLock
                  aria-hidden="true"
                  className="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400"
                  size={18}
                />
                <Input
                  id="current_password"
                  type={showCurrent ? 'text' : 'password'}
                  name="current_password"
                  placeholder="現在のパスワードを入力"
                  className="px-10"
                  autoComplete="current-password"
                  required
                  autoFocus
                  disabled={submitting}
                />
                <button
                  type="button"
                  onClick={() => setShowCurrent((visible) => !visible)}
                  className="absolute right-2.5 top-1/2 flex size-8 -translate-y-1/2 cursor-pointer items-center justify-center rounded-md text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600/30"
                  aria-label={showCurrent ? 'パスワードを隠す' : 'パスワードを表示'}
                  aria-pressed={showCurrent}
                >
                  {showCurrent ? (
                    <IconEyeOff size={18} aria-hidden="true" />
                  ) : (
                    <IconEye size={18} aria-hidden="true" />
                  )}
                </button>
              </div>
            </div>

            <div className="flex flex-col gap-2">
              <Label htmlFor="new_password">新しいパスワード</Label>
              <div className="relative">
                <IconLock
                  aria-hidden="true"
                  className="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400"
                  size={18}
                />
                <Input
                  id="new_password"
                  type={showNew ? 'text' : 'password'}
                  name="new_password"
                  placeholder="12文字以上"
                  className="px-10"
                  autoComplete="new-password"
                  minLength={12}
                  required
                  disabled={submitting}
                />
                <button
                  type="button"
                  onClick={() => setShowNew((visible) => !visible)}
                  className="absolute right-2.5 top-1/2 flex size-8 -translate-y-1/2 cursor-pointer items-center justify-center rounded-md text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600/30"
                  aria-label={showNew ? 'パスワードを隠す' : 'パスワードを表示'}
                  aria-pressed={showNew}
                >
                  {showNew ? (
                    <IconEyeOff size={18} aria-hidden="true" />
                  ) : (
                    <IconEye size={18} aria-hidden="true" />
                  )}
                </button>
              </div>
            </div>

            <Button type="submit" size="lg" className="mt-1 w-full" disabled={submitting}>
              {submitting ? '変更しています…' : 'パスワードを変更'}
              <IconArrowRight size={18} aria-hidden="true" />
            </Button>
          </div>
        </form>

        <div className="flex items-start gap-3 rounded-xl bg-slate-50 p-3.5 text-xs leading-5 text-slate-600">
          <IconShieldLock className="mt-0.5 shrink-0 text-slate-500" size={17} aria-hidden="true" />
          <p>
            直近に使ったパスワードは再利用できません。共有端末では変更後に必ずログアウトしてください。
          </p>
        </div>
      </div>
      {dialog}
    </AuthShell>
  )
}

import { IconAlertCircle, IconArrowRight, IconCircleCheck, IconLock } from '@tabler/icons-react'
import { type FormEvent, useState } from 'react'
import { AuthenticationAPIError, PasswordPolicyError, resetPassword } from '../../api'
import { AuthShell } from '../../components/AuthShell'
import { Alert } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'
import { Input } from '../../components/ui/input'
import { Label } from '../../components/ui/label'

export function ResetPasswordPage({ csrfToken, token }: { csrfToken: string; token: string }) {
  const [submitting, setSubmitting] = useState(false)
  const [success, setSuccess] = useState(false)
  const [error, setError] = useState(token ? '' : 'リセットリンクが不正です。')

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const password = String(new FormData(event.currentTarget).get('new_password') ?? '')
    setSubmitting(true)
    setError('')
    try {
      await resetPassword(csrfToken, token, password)
      setSuccess(true)
    } catch (cause) {
      if (cause instanceof PasswordPolicyError) {
        setError('12文字以上の、最近使用していないパスワードを指定してください。')
      } else if (cause instanceof AuthenticationAPIError) {
        setError(cause.message)
      } else {
        setError('認証サービスに接続できませんでした。')
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <AuthShell>
      <div className="flex flex-col gap-7">
        <header className="flex flex-col gap-2.5">
          <p className="eyebrow">アカウント復旧</p>
          <h2 className="page-title">新しいパスワードを設定</h2>
          <p className="page-description">新しいパスワードは12文字以上で入力してください。</p>
        </header>
        {success ? (
          <Alert className="flex gap-3 border-emerald-200 bg-emerald-50" aria-live="polite">
            <IconCircleCheck className="mt-0.5 text-emerald-600" size={19} aria-hidden="true" />
            <p className="text-sm text-emerald-900">パスワードを更新しました。ログインできます。</p>
          </Alert>
        ) : null}
        {error ? (
          <Alert className="flex gap-3" aria-live="polite">
            <IconAlertCircle className="mt-0.5 text-red-600" size={19} aria-hidden="true" />
            <p className="text-sm text-red-800">{error}</p>
          </Alert>
        ) : null}
        {!success ? (
          <form onSubmit={handleSubmit}>
            <div className="flex flex-col gap-5">
              <div className="flex flex-col gap-2">
                <Label htmlFor="new_password">新しいパスワード</Label>
                <div className="relative">
                  <IconLock
                    className="absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400"
                    size={18}
                  />
                  <Input
                    id="new_password"
                    name="new_password"
                    type="password"
                    className="pl-10"
                    autoComplete="new-password"
                    minLength={12}
                    required
                    autoFocus
                    disabled={!token || submitting}
                  />
                </div>
              </div>
              <Button type="submit" size="lg" className="w-full" disabled={!token || submitting}>
                {submitting ? '更新しています…' : 'パスワードを更新'}
                <IconArrowRight size={18} aria-hidden="true" />
              </Button>
            </div>
          </form>
        ) : null}
        <a className="text-center text-sm font-medium text-blue-700 hover:underline" href="/login">
          ログインに戻る
        </a>
      </div>
    </AuthShell>
  )
}

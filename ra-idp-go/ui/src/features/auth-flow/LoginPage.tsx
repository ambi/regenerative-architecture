import {
  IconAlertCircle,
  IconArrowRight,
  IconAt,
  IconEye,
  IconEyeOff,
  IconLock,
  IconShieldLock,
} from '@tabler/icons-react'
import { type FormEvent, useState } from 'react'
import { AuthenticationAPIError, continueBrowserFlow, login } from '../../api'
import { AuthShell } from '../../components/AuthShell'
import { Alert } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'
import { Input } from '../../components/ui/input'
import { Label } from '../../components/ui/label'

export function LoginPage({ csrfToken, returnTo }: { csrfToken: string; returnTo?: string }) {
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    setSubmitting(true)
    setError('')
    try {
      const result = await login(
        csrfToken,
        String(form.get('username') ?? ''),
        String(form.get('password') ?? ''),
        returnTo,
      )
      continueBrowserFlow(result)
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : '認証サービスに接続できませんでした。',
      )
      setSubmitting(false)
    }
  }

  return (
    <AuthShell>
      <div className="flex flex-col gap-7">
        <header className="flex flex-col gap-2.5">
          <p className="eyebrow">サインイン</p>
          <h2 className="page-title">アカウントにログイン</h2>
          <p className="page-description">
            組織から発行された認証情報を入力して、安全に続行してください。
          </p>
        </header>

        {error ? (
          <Alert className="flex gap-3" aria-live="polite">
            <IconAlertCircle
              className="mt-0.5 shrink-0 text-red-600"
              size={19}
              aria-hidden="true"
            />
            <div>
              <p className="font-semibold">ログインできません</p>
              <p className="mt-1 text-sm leading-5 text-red-800">{error}</p>
            </div>
          </Alert>
        ) : null}

        <form onSubmit={handleSubmit}>
          <div className="flex flex-col gap-5">
            <div className="flex flex-col gap-2">
              <Label htmlFor="username">ユーザー名</Label>
              <div className="relative">
                <IconAt
                  aria-hidden="true"
                  className="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400"
                  size={18}
                />
                <Input
                  id="username"
                  name="username"
                  placeholder="例: your.name"
                  className="pl-10"
                  autoComplete="username"
                  spellCheck={false}
                  required
                  autoFocus
                  disabled={submitting}
                />
              </div>
            </div>

            <div className="flex flex-col gap-2">
              <Label htmlFor="password">パスワード</Label>
              <div className="relative">
                <IconLock
                  aria-hidden="true"
                  className="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400"
                  size={18}
                />
                <Input
                  id="password"
                  type={showPassword ? 'text' : 'password'}
                  name="password"
                  placeholder="パスワードを入力"
                  className="px-10"
                  autoComplete="current-password"
                  required
                  disabled={submitting}
                />
                <button
                  type="button"
                  onClick={() => setShowPassword((visible) => !visible)}
                  className="absolute right-2.5 top-1/2 flex size-8 -translate-y-1/2 cursor-pointer items-center justify-center rounded-md text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600/30"
                  aria-label={showPassword ? 'パスワードを隠す' : 'パスワードを表示'}
                  aria-pressed={showPassword}
                >
                  {showPassword ? (
                    <IconEyeOff size={18} aria-hidden="true" />
                  ) : (
                    <IconEye size={18} aria-hidden="true" />
                  )}
                </button>
              </div>
            </div>

            <Button type="submit" size="lg" className="mt-1 w-full" disabled={submitting}>
              {submitting ? '確認しています…' : 'ログインして続行'}
              <IconArrowRight size={18} aria-hidden="true" />
            </Button>

            <div className="flex justify-center">
              <a
                className="text-xs font-medium text-blue-700 hover:underline"
                href="/forgot_password"
              >
                パスワードを忘れた場合
              </a>
            </div>
          </div>
        </form>

        <div className="flex items-start gap-3 rounded-xl bg-slate-50 p-3.5 text-xs leading-5 text-slate-600">
          <IconShieldLock className="mt-0.5 shrink-0 text-slate-500" size={17} aria-hidden="true" />
          <p>
            認証情報は保護された接続で送信されます。共有端末では、利用後に必ずログアウトしてください。
          </p>
        </div>
      </div>
    </AuthShell>
  )
}

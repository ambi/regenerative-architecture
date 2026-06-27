import { IconAlertCircle, IconArrowRight, IconKey, IconShieldCheck } from '@tabler/icons-react'
import { type FormEvent, useState } from 'react'
import { AuthenticationAPIError, continueBrowserFlow, submitTOTP } from '../../api'
import { AuthShell } from '../../components/AuthShell'
import { Alert } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'
import { Input } from '../../components/ui/input'
import { Label } from '../../components/ui/label'

export function TotpPage({ csrfToken, returnTo }: { csrfToken: string; returnTo?: string }) {
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    setSubmitting(true)
    setError('')
    try {
      const result = await submitTOTP(csrfToken, String(form.get('code') ?? ''), returnTo)
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
          <p className="eyebrow">二要素認証</p>
          <h2 className="page-title">確認コードを入力</h2>
          <p className="page-description">
            Authenticator アプリに表示されている6桁のコードを入力してください。
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
              <p className="font-semibold">確認できません</p>
              <p className="mt-1 text-sm leading-5 text-red-800">{error}</p>
            </div>
          </Alert>
        ) : null}

        <form onSubmit={handleSubmit}>
          <div className="flex flex-col gap-5">
            <div className="flex flex-col gap-2">
              <Label htmlFor="code">確認コード</Label>
              <div className="relative">
                <IconKey
                  aria-hidden="true"
                  className="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400"
                  size={18}
                />
                <Input
                  id="code"
                  name="code"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  pattern="[0-9]{6}"
                  maxLength={6}
                  placeholder="000000"
                  className="pl-10"
                  required
                  autoFocus
                  disabled={submitting}
                />
              </div>
            </div>

            <Button type="submit" size="lg" className="mt-1 w-full" disabled={submitting}>
              {submitting ? '確認しています…' : 'コードを確認'}
              <IconArrowRight size={18} aria-hidden="true" />
            </Button>
          </div>
        </form>

        <div className="flex items-start gap-3 rounded-xl bg-slate-50 p-3.5 text-xs leading-5 text-slate-600">
          <IconShieldCheck
            className="mt-0.5 shrink-0 text-slate-500"
            size={17}
            aria-hidden="true"
          />
          <p>コードは短時間で期限切れになります。最新のコードを使用してください。</p>
        </div>
      </div>
    </AuthShell>
  )
}

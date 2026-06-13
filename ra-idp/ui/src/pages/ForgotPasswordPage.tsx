import { IconCheck, IconLoader2 } from '@tabler/icons-react'
import { type FormEvent, useId, useState } from 'react'
import { AuthLayout } from '@/components/layout/AuthLayout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useMessages } from '@/i18n/context'
import { readForgotPasswordContext } from '@/lib/page-context'

/**
 * /forgot_password の SPA ページ。
 *
 * - ADR-030: サーバは anti-enumeration のため常に 204 を返す。
 *   画面側も email 存在有無を区別せず "受け付けた" メッセージのみを表示する。
 * - ネットワーク失敗時のみ network error を出す。
 */
export function ForgotPasswordPage() {
  const ctx = readForgotPasswordContext()
  const m = useMessages()
  const errorId = useId()
  const [email, setEmail] = useState('')
  const [submitted, setSubmitted] = useState(false)
  const [networkError, setNetworkError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setNetworkError(null)
    setSubmitting(true)
    try {
      await fetch('/api/auth/forgot_password', {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
          'X-CSRF-Token': ctx.csrf,
        },
        body: JSON.stringify({ email }),
        credentials: 'same-origin',
      })
      // 204 / 4xx (CSRF 失敗等) すべて、UX としては "受け付けた" を表示する。
      setSubmitted(true)
    } catch {
      setNetworkError(m.forgotPassword.networkError)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <AuthLayout
      title={m.forgotPassword.title}
      description={m.forgotPassword.description}
      status="secure"
      footer={
        <span>
          {m.forgotPassword.footer}
          <span className="font-mono">{m.forgotPassword.footerCode}</span>
          {m.forgotPassword.footerTail}
        </span>
      }
    >
      <Card>
        <CardContent className="pt-6">
          {submitted ? (
            <div className="space-y-5">
              <Alert>
                <IconCheck className="h-4 w-4" aria-hidden />
                <AlertTitle>{m.forgotPassword.successTitle}</AlertTitle>
                <AlertDescription>{m.forgotPassword.successBody}</AlertDescription>
              </Alert>
              <a
                href="/login"
                className="block text-center text-sm text-primary underline-offset-4 hover:underline"
              >
                {m.forgotPassword.backToLogin}
              </a>
            </div>
          ) : (
            <form
              className="space-y-5"
              onSubmit={onSubmit}
              noValidate
              aria-describedby={networkError ? errorId : undefined}
            >
              {networkError ? (
                <Alert variant="destructive" id={errorId}>
                  <AlertDescription>{networkError}</AlertDescription>
                </Alert>
              ) : null}

              <div className="space-y-2">
                <Label htmlFor="email">{m.forgotPassword.email}</Label>
                <Input
                  id="email"
                  name="email"
                  type="email"
                  autoComplete="email"
                  autoFocus
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
              </div>

              <Button type="submit" className="w-full" disabled={submitting}>
                {submitting ? (
                  <>
                    <IconLoader2 className="h-4 w-4 motion-safe:animate-spin" aria-hidden />
                    {m.forgotPassword.submitting}
                  </>
                ) : (
                  m.forgotPassword.submit
                )}
              </Button>

              <a
                href="/login"
                className="block text-center text-sm text-muted-foreground underline-offset-4 hover:underline"
              >
                {m.forgotPassword.backToLogin}
              </a>
            </form>
          )}
        </CardContent>
      </Card>
    </AuthLayout>
  )
}

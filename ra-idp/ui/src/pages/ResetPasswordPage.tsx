import { IconAlertCircle, IconCheck, IconLoader2 } from '@tabler/icons-react'
import { type FormEvent, useId, useState } from 'react'
import { AuthLayout } from '@/components/layout/AuthLayout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useMessages } from '@/i18n/context'
import { readResetPasswordContext } from '@/lib/page-context'

/**
 * /reset_password の SPA ページ。
 *
 * - サーバが GET /reset_password で SPA shell + token (meta) + CSRF cookie を発行。
 * - 送信は POST /api/auth/reset_password { token, new_password }。410 を期限切れ、
 *   400 を policy/reuse、200 を成功として扱う。
 */
export function ResetPasswordPage() {
  const ctx = readResetPasswordContext()
  const m = useMessages()
  const errorId = useId()
  const [next, setNext] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  function violationToMessage(v: string): string {
    switch (v) {
      case 'too_short':
        return m.resetPassword.errorPolicyTooShort
      case 'too_long':
        return m.resetPassword.errorPolicyTooLong
      case 'similar_to_identifier':
        return m.resetPassword.errorPolicySimilar
      case 'common_password':
        return m.resetPassword.errorPolicyCommon
      case 'breached':
        return m.resetPassword.errorPolicyBreached
      default:
        return m.resetPassword.errorGeneric
    }
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setSuccess(false)
    if (!ctx.token) {
      setError(m.resetPassword.errorMissingToken)
      return
    }
    setSubmitting(true)
    try {
      const res = await fetch('/api/auth/reset_password', {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
          'X-CSRF-Token': ctx.csrf,
        },
        body: JSON.stringify({ token: ctx.token, new_password: next }),
        credentials: 'same-origin',
      })
      if (res.ok) {
        setSuccess(true)
        setNext('')
        return
      }
      const detail = (await res.json().catch(() => null)) as {
        error?: string
        violations?: string[]
      } | null
      if (res.status === 410 || detail?.error === 'invalid_reset_token') {
        setError(m.resetPassword.errorInvalidToken)
      } else if (detail?.error === 'password_reuse') {
        setError(m.resetPassword.errorReuse)
      } else if (detail?.error === 'password_policy_violation') {
        const messages = (detail.violations ?? []).map(violationToMessage)
        setError(messages.length > 0 ? messages.join(' ') : m.resetPassword.errorGeneric)
      } else {
        setError(m.resetPassword.errorGeneric)
      }
    } catch {
      setError(m.resetPassword.networkError)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <AuthLayout
      title={m.resetPassword.title}
      description={m.resetPassword.description}
      status="secure"
      footer={
        <span>
          {m.resetPassword.footer}
          <span className="font-mono">{m.resetPassword.footerCode}</span>
          {m.resetPassword.footerTail}
        </span>
      }
    >
      <Card>
        <CardContent className="pt-6">
          {success ? (
            <div className="space-y-5">
              <Alert>
                <IconCheck className="h-4 w-4" aria-hidden />
                <AlertTitle>{m.resetPassword.successTitle}</AlertTitle>
                <AlertDescription>{m.resetPassword.successBody}</AlertDescription>
              </Alert>
              <a
                href="/login"
                className="block text-center text-sm text-primary underline-offset-4 hover:underline"
              >
                {m.resetPassword.backToLogin}
              </a>
            </div>
          ) : (
            <form
              className="space-y-5"
              onSubmit={onSubmit}
              noValidate
              aria-describedby={error ? errorId : undefined}
            >
              {error ? (
                <Alert variant="destructive" id={errorId}>
                  <IconAlertCircle className="h-4 w-4" aria-hidden />
                  <AlertTitle>{m.resetPassword.errorTitle}</AlertTitle>
                  <AlertDescription>{error}</AlertDescription>
                </Alert>
              ) : null}

              <div className="space-y-2">
                <Label htmlFor="new_password">{m.resetPassword.newPassword}</Label>
                <Input
                  id="new_password"
                  name="new_password"
                  type="password"
                  autoComplete="new-password"
                  autoFocus
                  required
                  value={next}
                  onChange={(e) => setNext(e.target.value)}
                />
              </div>

              <Button type="submit" className="w-full" disabled={submitting}>
                {submitting ? (
                  <>
                    <IconLoader2 className="h-4 w-4 motion-safe:animate-spin" aria-hidden />
                    {m.resetPassword.submitting}
                  </>
                ) : (
                  m.resetPassword.submit
                )}
              </Button>
            </form>
          )}
        </CardContent>
      </Card>
    </AuthLayout>
  )
}

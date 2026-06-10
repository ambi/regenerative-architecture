import { IconAlertCircle, IconCheck, IconLoader2 } from '@tabler/icons-react'
import { type FormEvent, useId, useState } from 'react'
import { AuthLayout } from '@/components/layout/AuthLayout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useMessages } from '@/i18n/context'
import { readChangePasswordContext } from '@/lib/page-context'

/**
 * /account/password の SPA ページ。
 *
 * - サーバが session cookie を検証し未ログインなら 303 /login するため
 *   ここでは session 喪失を 401 応答ハンドリングだけで扱う。
 * - error は API が返す { error, violations? } を i18n に変換する。
 */
export function ChangePasswordPage() {
  const ctx = readChangePasswordContext()
  const m = useMessages()
  const errorId = useId()
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  function violationToMessage(v: string): string {
    switch (v) {
      case 'too_short':
        return m.changePassword.errorPolicyTooShort
      case 'too_long':
        return m.changePassword.errorPolicyTooLong
      case 'similar_to_identifier':
        return m.changePassword.errorPolicySimilar
      case 'common_password':
        return m.changePassword.errorPolicyCommon
      default:
        return m.changePassword.errorGeneric
    }
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setSuccess(false)
    setSubmitting(true)
    try {
      const res = await fetch('/api/auth/change_password', {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
          'X-CSRF-Token': ctx.csrf,
        },
        body: JSON.stringify({ current_password: current, new_password: next }),
        credentials: 'same-origin',
      })
      if (res.ok) {
        setSuccess(true)
        setCurrent('')
        setNext('')
        return
      }
      const detail = (await res.json().catch(() => null)) as {
        error?: string
        violations?: string[]
      } | null
      if (res.status === 401) {
        setError(m.changePassword.errorSession)
      } else if (detail?.error === 'current_password_mismatch') {
        setError(m.changePassword.errorCurrentMismatch)
      } else if (detail?.error === 'password_reuse') {
        setError(m.changePassword.errorReuse)
      } else if (detail?.error === 'password_policy_violation') {
        const messages = (detail.violations ?? []).map(violationToMessage)
        setError(messages.length > 0 ? messages.join(' ') : m.changePassword.errorGeneric)
      } else {
        setError(m.changePassword.errorGeneric)
      }
    } catch {
      setError(m.changePassword.networkError)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <AuthLayout
      title={m.changePassword.title}
      description={m.changePassword.description}
      status="secure"
      footer={
        <span>
          {m.changePassword.footer}
          <span className="font-mono">{m.changePassword.footerCode}</span>
          {m.changePassword.footerTail}
        </span>
      }
    >
      <Card>
        <CardContent className="pt-6">
          <form
            className="space-y-5"
            onSubmit={onSubmit}
            noValidate
            aria-describedby={error ? errorId : undefined}
          >
            {success ? (
              <Alert>
                <IconCheck className="h-4 w-4" aria-hidden />
                <AlertTitle>{m.changePassword.successTitle}</AlertTitle>
                <AlertDescription>{m.changePassword.successBody}</AlertDescription>
              </Alert>
            ) : null}

            {error ? (
              <Alert variant="destructive" id={errorId}>
                <IconAlertCircle className="h-4 w-4" aria-hidden />
                <AlertTitle>{m.changePassword.errorTitle}</AlertTitle>
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            ) : null}

            <div className="space-y-2">
              <Label htmlFor="current_password">{m.changePassword.currentPassword}</Label>
              <Input
                id="current_password"
                name="current_password"
                type="password"
                autoComplete="current-password"
                autoFocus
                required
                value={current}
                onChange={(e) => setCurrent(e.target.value)}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="new_password">{m.changePassword.newPassword}</Label>
              <Input
                id="new_password"
                name="new_password"
                type="password"
                autoComplete="new-password"
                required
                value={next}
                onChange={(e) => setNext(e.target.value)}
              />
            </div>

            <Button type="submit" className="w-full" disabled={submitting}>
              {submitting ? (
                <>
                  <IconLoader2 className="h-4 w-4 motion-safe:animate-spin" aria-hidden />
                  {m.changePassword.submitting}
                </>
              ) : (
                m.changePassword.submit
              )}
            </Button>
          </form>
        </CardContent>
      </Card>
    </AuthLayout>
  )
}

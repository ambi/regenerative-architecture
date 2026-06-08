import { IconAlertCircle, IconLoader2, IconShieldCheck } from '@tabler/icons-react'
import { type ChangeEvent, type FormEvent, useEffect, useId, useState } from 'react'
import { AuthLayout } from '@/components/layout/AuthLayout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { useMessages } from '@/i18n/context'
import {
  continueBrowserFlow,
  loadBrowserTransaction,
  type BrowserFlowResponse,
} from '@/lib/browser-flow'
import { readTotpContext } from '@/lib/page-context'

/**
 * /totp の SPA ページ。
 *
 * - Authenticator アプリの 6 桁コードを入力するフォーム
 * - 送信は POST /api/auth/totp、サーバは next / redirect_to を JSON で返す
 * - サーバが invalid な場合は 401 JSON を返し、同画面でエラー表示する
 * - a11y: code 入力欄に Label、エラーは role=alert
 */
export function TotpPage() {
  const ctx = readTotpContext()
  const m = useMessages()
  const errorId = useId()
  const [code, setCode] = useState('')
  const [csrf, setCsrf] = useState(ctx.csrf)
  const [error, setError] = useState<string | null>(ctx.invalidPrevious ? m.totp.errorBody : null)
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    let active = true
    loadBrowserTransaction()
      .then((transaction) => {
        if (!active) return
        if (transaction.kind === 'login') {
          window.location.assign('/login')
          return
        }
        if (transaction.kind === 'consent') {
          window.location.assign('/consent')
          return
        }
        setCsrf(transaction.csrf_token)
      })
      .catch(() => {
        if (active) setError(m.totp.errorBody)
      })
    return () => {
      active = false
    }
  }, [m.totp.errorBody])

  function onChange(e: ChangeEvent<HTMLInputElement>) {
    setCode(e.target.value.replace(/\D/g, '').slice(0, 6))
    setError(null)
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    if (!code) {
      setError(m.totp.errorEmptyCode)
      return
    }
    setError(null)
    setSubmitting(true)
    try {
      const res = await fetch('/api/auth/totp', {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
          'X-CSRF-Token': csrf,
        },
        body: JSON.stringify({ code }),
        credentials: 'same-origin',
      })
      if (res.ok) {
        continueBrowserFlow((await res.json()) as BrowserFlowResponse)
        return
      }
      if (res.status === 401) {
        setError(m.totp.errorBody)
        return
      }
      setError(m.totp.errorBody)
    } catch {
      setError(m.totp.networkError)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <AuthLayout
      title={m.totp.title}
      description={m.totp.description}
      status="pending"
      footer={
        <span>
          {m.totp.footer}
          <span className="font-mono">{m.totp.footerCode}</span>
          {m.totp.footerTail}
        </span>
      }
    >
      <Card>
        <CardContent className="space-y-6 pt-6">
          {error ? (
            <Alert variant="destructive" id={errorId}>
              <IconAlertCircle className="h-4 w-4" aria-hidden />
              <AlertTitle>{m.totp.errorTitle}</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}

          <div className="flex items-start gap-3 rounded-md border border-border bg-muted/40 p-4">
            <IconShieldCheck className="mt-0.5 h-5 w-5 text-primary" aria-hidden />
            <div className="space-y-0.5">
              <div className="text-sm font-medium">{m.totp.title}</div>
              <div className="text-xs text-muted-foreground">{m.totp.description}</div>
            </div>
          </div>

          <form
            className="space-y-4"
            onSubmit={onSubmit}
            noValidate
            aria-describedby={error ? errorId : undefined}
          >
            <div className="space-y-2.5">
              <Label
                htmlFor="code"
                className="text-xs uppercase tracking-wider text-muted-foreground"
              >
                {m.totp.codeLabel}
              </Label>
              <input
                id="code"
                name="code"
                autoComplete="one-time-code"
                inputMode="numeric"
                spellCheck={false}
                maxLength={6}
                value={code}
                onChange={onChange}
                placeholder={m.totp.placeholder}
                aria-describedby={error ? errorId : undefined}
                aria-invalid={error ? true : undefined}
                className="h-16 w-full rounded-md border border-input bg-card px-4 text-center font-mono text-2xl font-semibold tracking-[0.4em] uppercase ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              />
            </div>

            <Button type="submit" className="w-full" disabled={submitting}>
              {submitting ? (
                <>
                  <IconLoader2 className="h-4 w-4 motion-safe:animate-spin" aria-hidden />
                  {m.totp.submitting}
                </>
              ) : (
                m.totp.submit
              )}
            </Button>
          </form>
        </CardContent>
      </Card>
    </AuthLayout>
  )
}

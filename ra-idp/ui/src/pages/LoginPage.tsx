import { IconAlertCircle, IconInfoCircle, IconLoader2 } from '@tabler/icons-react'
import { type FormEvent, useEffect, useId, useState } from 'react'
import { AuthLayout } from '@/components/layout/AuthLayout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useMessages } from '@/i18n/context'
import {
  continueBrowserFlow,
  loadBrowserTransaction,
  type BrowserFlowResponse,
} from '@/lib/browser-flow'
import { readLoginContext } from '@/lib/page-context'

/**
 * /login の SPA ページ。
 *
 * - 認証成立後は POST /api/auth/login の JSON 応答に従って /totp、/consent、
 *   または client redirect_uri へ遷移する。
 * - a11y: form エラーは `role=alert` + `aria-describedby` で入力欄に紐付け
 */
export function LoginPage() {
  const ctx = readLoginContext()
  const m = useMessages()
  const errorId = useId()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [csrf, setCsrf] = useState(ctx.csrf)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    let active = true
    loadBrowserTransaction()
      .then((transaction) => {
        if (!active) return
        if (transaction.kind === 'totp') {
          window.location.assign('/totp')
          return
        }
        if (transaction.kind === 'consent') {
          window.location.assign('/consent')
          return
        }
        setCsrf(transaction.csrf_token)
      })
      .catch(() => {
        if (active) setError(m.login.invalidRequestBody)
      })
    return () => {
      active = false
    }
  }, [m.login.invalidRequestBody])

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      const res = await fetch('/api/auth/login', {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
          'X-CSRF-Token': csrf,
        },
        body: JSON.stringify({ username, password }),
        credentials: 'same-origin',
      })
      if (res.ok) {
        continueBrowserFlow((await res.json()) as BrowserFlowResponse)
        return
      }
      // 400 invalid_request は資格情報ではなく認可リクエスト側の問題なので
      // 「ユーザー名/パスワードが誤り」と取り違えないよう個別のメッセージを出す。
      const detail = (await res.json().catch(() => null)) as { error?: string } | null
      if (res.status === 400 && detail?.error === 'invalid_request') {
        setError(m.login.invalidRequestBody)
      } else {
        setError(m.login.errorBody)
      }
    } catch {
      setError(m.login.networkError)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <AuthLayout
      title={m.login.title}
      description={m.login.description}
      status="secure"
      footer={
        <span>
          {m.login.footer}
          <span className="font-mono">{m.login.footerCode}</span>
          {m.login.footerTail}
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
            {!csrf ? (
              <Alert>
                <IconInfoCircle className="h-4 w-4" aria-hidden />
                <AlertTitle>{m.login.invalidRequestTitle}</AlertTitle>
                <AlertDescription>{m.login.invalidRequestBody}</AlertDescription>
              </Alert>
            ) : null}

            {error ? (
              <Alert variant="destructive" id={errorId}>
                <IconAlertCircle className="h-4 w-4" aria-hidden />
                <AlertTitle>{m.login.errorTitle}</AlertTitle>
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            ) : null}

            <div className="space-y-2">
              <Label htmlFor="username">{m.login.username}</Label>
              <Input
                id="username"
                name="username"
                autoComplete="username"
                autoFocus
                required
                value={username}
                onChange={(e) => setUsername(e.target.value)}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="password">{m.login.password}</Label>
              <Input
                id="password"
                name="password"
                type="password"
                autoComplete="current-password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>

            <Button type="submit" className="w-full" disabled={submitting}>
              {submitting ? (
                <>
                  <IconLoader2 className="h-4 w-4 motion-safe:animate-spin" aria-hidden />
                  {m.login.submitting}
                </>
              ) : (
                m.login.submit
              )}
            </Button>
          </form>
        </CardContent>
      </Card>
    </AuthLayout>
  )
}

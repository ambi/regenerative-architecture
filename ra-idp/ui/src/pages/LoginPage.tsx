import { IconAlertCircle, IconLoader2 } from '@tabler/icons-react'
import { type FormEvent, useId, useState } from 'react'
import { AuthLayout } from '@/components/layout/AuthLayout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useMessages } from '@/i18n/context'
import { readLoginContext } from '@/lib/page-context'

/**
 * /login の SPA ページ。
 *
 * - 認証成立後はバックエンドの POST /login が 30x で /authorize へ戻す
 * - SPA からの fetch は `redirect: 'follow'`。最終応答が
 *     - HTML (続きの consent / authorize 画面) → window.location.assign で遷移
 *     - JSON エラー → そのまま表示
 *   の 2 パターンに分岐する
 * - a11y: form エラーは `role=alert` + `aria-describedby` で入力欄に紐付け
 */
export function LoginPage() {
  const ctx = readLoginContext()
  const m = useMessages()
  const errorId = useId()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      const body = new URLSearchParams({
        request_id: ctx.requestId,
        csrf: ctx.csrf,
        username,
        password,
      })
      const res = await fetch('/login', {
        method: 'POST',
        headers: {
          'content-type': 'application/x-www-form-urlencoded',
          accept: 'application/json',
        },
        body,
        redirect: 'follow',
        credentials: 'same-origin',
      })
      if (res.redirected) {
        window.location.assign(res.url)
        return
      }
      if (res.ok) {
        const json = (await res.json().catch(() => null)) as { redirect?: string } | null
        if (json?.redirect) {
          window.location.assign(json.redirect)
          return
        }
        window.location.reload()
        return
      }
      setError(m.login.errorBody)
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

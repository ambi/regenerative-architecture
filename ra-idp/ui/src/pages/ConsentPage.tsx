import { IconAlertCircle, IconCheck, IconLoader2, IconShieldLock, IconX } from '@tabler/icons-react'
import { type FormEvent, useEffect, useId, useState } from 'react'
import { AuthLayout } from '@/components/layout/AuthLayout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { useMessages } from '@/i18n/context'
import {
  continueBrowserFlow,
  loadBrowserTransaction,
  type BrowserFlowResponse,
} from '@/lib/browser-flow'
import { readConsentContext } from '@/lib/page-context'
import type { Messages } from '@/i18n/messages'

/**
 * /consent の SPA ページ。
 *
 * - クライアント名と要求スコープを明示
 * - allow / deny の 2 ボタン (allow は primary、deny は outline)
 * - 送信は POST /api/auth/consent、サーバは redirect_to を JSON で返す
 * - a11y: scope リストは `<ul>` + 各 li、ボタンは識別可能なテキスト
 */
export function ConsentPage() {
  const ctx = readConsentContext()
  const m = useMessages()
  const errorId = useId()
  const [csrf, setCsrf] = useState(ctx.csrf)
  const [clientName, setClientName] = useState(ctx.clientName)
  const [scopes, setScopes] = useState(ctx.scopes)
  const [action, setAction] = useState<'allow' | 'deny' | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let active = true
    loadBrowserTransaction()
      .then((transaction) => {
        if (!active) return
        if (transaction.kind === 'login') {
          window.location.assign('/login')
          return
        }
        if (transaction.kind === 'totp') {
          window.location.assign('/totp')
          return
        }
        setCsrf(transaction.csrf_token)
        setClientName(transaction.client_name ?? '')
        setScopes(transaction.scopes ?? [])
      })
      .catch(() => {
        if (active) setError(m.consent.errorBody)
      })
    return () => {
      active = false
    }
  }, [m.consent.errorBody])

  async function submit(chosen: 'allow' | 'deny', e: FormEvent) {
    e.preventDefault()
    setError(null)
    setAction(chosen)
    try {
      const res = await fetch('/api/auth/consent', {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
          'X-CSRF-Token': csrf,
        },
        body: JSON.stringify({ action: chosen }),
        credentials: 'same-origin',
      })
      if (res.ok) {
        continueBrowserFlow((await res.json()) as BrowserFlowResponse)
        return
      }
      setError(m.consent.errorBody)
      setAction(null)
    } catch {
      setError(m.consent.networkError)
      setAction(null)
    }
  }

  return (
    <AuthLayout
      title={m.consent.title}
      description={`${m.consent.descriptionPrefix}${clientName}${m.consent.descriptionSuffix}`}
      status="pending"
      footer={
        <span>
          {m.consent.footer}
          <span className="font-mono">{m.consent.footerCode}</span>
          {m.consent.footerTail}
        </span>
      }
    >
      <Card>
        <CardContent className="space-y-6 pt-6">
          {error ? (
            <Alert variant="destructive" id={errorId}>
              <IconAlertCircle className="h-4 w-4" aria-hidden />
              <AlertTitle>{m.consent.errorTitle}</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}

          <div className="flex items-start gap-3 rounded-md border border-border bg-muted/40 p-4">
            <IconShieldLock className="mt-0.5 h-5 w-5 text-primary" aria-hidden />
            <div className="flex-1 space-y-0.5">
              <div className="font-serif text-base font-medium">{clientName}</div>
              <div className="text-[11px] text-muted-foreground font-mono">{ctx.clientId}</div>
            </div>
          </div>

          <div className="space-y-3">
            <h2 className="text-sm font-medium text-muted-foreground">
              {m.consent.requestedHeading}
            </h2>
            <ul className="space-y-2">
              {scopes.map((scope) => {
                const desc = describeScope(scope, m)
                return (
                  <li
                    key={scope}
                    className="flex items-start gap-3 rounded-md border border-border bg-card p-3"
                  >
                    <IconCheck className="mt-0.5 h-4 w-4 text-primary" aria-hidden />
                    <div className="flex-1">
                      <div className="text-sm font-medium">{desc.title}</div>
                      <div className="text-xs text-muted-foreground">{desc.description}</div>
                    </div>
                    <code className="text-[10px] text-muted-foreground">{scope}</code>
                  </li>
                )
              })}
            </ul>
          </div>

          <div className="flex gap-3 pt-2" aria-describedby={error ? errorId : undefined}>
            <Button
              type="button"
              variant="outline"
              className="flex-1"
              disabled={action !== null}
              onClick={(e) => submit('deny', e)}
            >
              {action === 'deny' ? (
                <IconLoader2 className="h-4 w-4 motion-safe:animate-spin" aria-hidden />
              ) : (
                <IconX className="h-4 w-4" aria-hidden />
              )}
              {m.consent.deny}
            </Button>
            <Button
              type="button"
              className="flex-1"
              disabled={action !== null}
              onClick={(e) => submit('allow', e)}
            >
              {action === 'allow' ? (
                <IconLoader2 className="h-4 w-4 motion-safe:animate-spin" aria-hidden />
              ) : (
                <IconCheck className="h-4 w-4" aria-hidden />
              )}
              {m.consent.allow}
            </Button>
          </div>
        </CardContent>
      </Card>
    </AuthLayout>
  )
}

function describeScope(scope: string, m: Messages): { title: string; description: string } {
  const known = m.consent.scopes[scope as keyof typeof m.consent.scopes]
  if (known?.title) return { title: known.title, description: known.description }
  return { title: scope, description: m.consent.scopes.unknown.description }
}

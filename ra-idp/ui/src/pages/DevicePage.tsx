import {
  IconAlertCircle,
  IconCheck,
  IconDeviceMobile,
  IconLoader2,
  IconX,
} from '@tabler/icons-react'
import { type ChangeEvent, type FormEvent, useId, useState } from 'react'
import { AuthLayout } from '@/components/layout/AuthLayout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { useMessages } from '@/i18n/context'
import { readDeviceContext } from '@/lib/page-context'

/**
 * /device (RFC 8628 verification_uri) の SPA ページ。
 *
 * デザイン:
 *   - user_code は chunked input (XXXX-XXXX) の見た目で、入力時に自動でハイフンを補う
 *   - 「デバイスから表示された 8 文字のコード」というメンタルモデルを優先
 *   - 承認/拒否ボタンを 2 つ並べる
 * a11y:
 *   - user_code フィールドは Label と紐付け
 *   - error は `role=alert` (Alert コンポーネント) で読み上げ
 */
export function DevicePage() {
  const ctx = readDeviceContext()
  const m = useMessages()
  const errorId = useId()
  const [userCode, setUserCode] = useState(formatUserCode(ctx.prefillUserCode))
  const [action, setAction] = useState<'allow' | 'deny' | null>(null)
  const [error, setError] = useState<string | null>(null)

  function onChange(e: ChangeEvent<HTMLInputElement>) {
    setUserCode(formatUserCode(e.target.value))
    setError(null)
  }

  async function submit(chosen: 'allow' | 'deny', e: FormEvent) {
    e.preventDefault()
    if (!userCode.replace(/[^a-z0-9]/gi, '')) {
      setError(m.device.errorEmptyCode)
      return
    }
    setError(null)
    setAction(chosen)
    try {
      const body = new URLSearchParams({
        user_code: userCode,
        csrf: ctx.csrf,
        action: chosen,
      })
      // バックエンドが完了画面 (error shell) を返すパターンと、redirect を返すパターンが
      // ありうる。fetch が cross-origin redirect に follow して落ちるのを避けるため
      // `redirect: 'manual'` で受け取り reload で遷移する (LoginPage 参照)。
      const res = await fetch('/device', {
        method: 'POST',
        headers: { 'content-type': 'application/x-www-form-urlencoded' },
        body,
        redirect: 'manual',
        credentials: 'same-origin',
      })
      if (res.type === 'opaqueredirect' || res.ok) {
        window.location.reload()
        return
      }
      setError(m.device.errorBody)
      setAction(null)
    } catch {
      setError(m.device.networkError)
      setAction(null)
    }
  }

  return (
    <AuthLayout
      title={m.device.title}
      description={m.device.description}
      status="pending"
      footer={
        <span>
          {m.device.footer}
          <span className="font-mono">{m.device.footerCode}</span>
          {m.device.footerTail}
        </span>
      }
    >
      <Card>
        <CardContent className="space-y-6 pt-6">
          {error ? (
            <Alert variant="destructive" id={errorId}>
              <IconAlertCircle className="h-4 w-4" aria-hidden />
              <AlertTitle>{m.device.errorTitle}</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}

          <div className="flex items-start gap-3 rounded-md border border-border bg-muted/40 p-4">
            <IconDeviceMobile className="mt-0.5 h-5 w-5 text-primary" aria-hidden />
            <div className="space-y-0.5">
              <div className="text-sm font-medium">{m.device.deviceRequesting}</div>
              <div className="text-xs text-muted-foreground">{m.device.physicalHint}</div>
            </div>
          </div>

          <div className="space-y-2.5">
            <Label
              htmlFor="user_code"
              className="text-xs uppercase tracking-wider text-muted-foreground"
            >
              {m.device.userCodeLabel}
            </Label>
            <input
              id="user_code"
              name="user_code"
              autoComplete="one-time-code"
              inputMode="text"
              spellCheck={false}
              maxLength={10}
              value={userCode}
              onChange={onChange}
              placeholder={m.device.placeholder}
              aria-describedby={error ? errorId : undefined}
              aria-invalid={error ? true : undefined}
              className="h-16 w-full rounded-md border border-input bg-card px-4 text-center font-mono text-2xl font-semibold tracking-[0.4em] uppercase ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
            />
          </div>

          <div className="flex gap-3">
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
              {m.device.deny}
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
              {m.device.allow}
            </Button>
          </div>
        </CardContent>
      </Card>
    </AuthLayout>
  )
}

/**
 * 4 文字 + ハイフン + 4 文字 (例: WDJB-MJHT) に整形。RFC 8628 §6.1 の推奨。
 */
function formatUserCode(raw: string): string {
  const cleaned = raw
    .replace(/[^a-z0-9]/gi, '')
    .toUpperCase()
    .slice(0, 8)
  if (cleaned.length <= 4) return cleaned
  return `${cleaned.slice(0, 4)}-${cleaned.slice(4)}`
}

import {
  IconAlertTriangle,
  IconCircleCheck,
  IconLogout,
  IconShieldX,
} from '@tabler/icons-react'
import type { ReactNode } from 'react'
import { AuthLayout } from '@/components/layout/AuthLayout'
import { Card, CardContent } from '@/components/ui/card'
import { useMessages } from '@/i18n/context'
import type { Messages } from '@/i18n/messages'
import { readErrorContext } from '@/lib/page-context'

/**
 * /error と /end_session の完了表示を担う SPA ページ。
 *
 * 種別 (`ra-idp:error-kind`) に応じてアイコン・トーン・status pill を切り替える。
 * リダイレクト不可な OAuth エラー (redirect_uri 未確定など) もここに集約する。
 *
 * 文言は i18n catalog に固定キーで持ち、サーバが特殊な文言を上書きしたいときは
 * `ra-idp:error-title` / `ra-idp:error-description` を直接送ると catalog より
 * 優先される (Phase 5 のテナント別文言差し替えで使う slot)。
 */
export function ErrorPage() {
  const ctx = readErrorContext()
  const m = useMessages()
  const v = pickVariant(ctx.kind, m)

  const title = ctx.title || v.title
  const description = ctx.description || v.description

  return (
    <AuthLayout
      title={title}
      description={description}
      status={v.status}
      footer={
        ctx.detail ? (
          <span className="font-mono">{ctx.detail}</span>
        ) : (
          <span>
            {m.error.detailFallback}
            <span className="font-mono">{m.error.detailFallbackCode}</span>
            {m.error.detailFallbackTail}
          </span>
        )
      }
    >
      <Card>
        <CardContent className="space-y-5 pt-8 pb-8 text-center">
          <div
            className={`mx-auto grid h-14 w-14 place-items-center rounded-full ${v.iconWrap}`}
            aria-hidden
          >
            {v.icon}
          </div>
          <div className="space-y-2">
            <p className="font-serif text-xl font-medium">{title}</p>
            {description ? (
              <p className="text-sm text-muted-foreground leading-relaxed">{description}</p>
            ) : null}
          </div>
          <div className="flex justify-center pt-2">
            <span className="rounded-full bg-muted/60 px-3 py-1 font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
              {ctx.kind}
            </span>
          </div>
        </CardContent>
      </Card>
    </AuthLayout>
  )
}

interface ErrorVariant {
  title: string
  description: string
  icon: ReactNode
  iconWrap: string
  status: 'secure' | 'pending' | 'error' | null
}

function pickVariant(kind: string, m: Messages): ErrorVariant {
  const v = m.error.variants
  switch (kind) {
    case 'logged_out':
      return {
        ...v.logged_out,
        icon: <IconLogout className="h-6 w-6 text-primary" />,
        iconWrap: 'bg-primary/10 ring-1 ring-primary/20',
        status: 'secure',
      }
    case 'access_denied':
      return {
        ...v.access_denied,
        icon: <IconShieldX className="h-6 w-6 text-destructive" />,
        iconWrap: 'bg-destructive/10 ring-1 ring-destructive/20',
        status: 'error',
      }
    case 'device_approved':
      return {
        ...v.device_approved,
        icon: <IconCircleCheck className="h-6 w-6 text-emerald-600" />,
        iconWrap: 'bg-emerald-500/10 ring-1 ring-emerald-500/20',
        status: 'secure',
      }
    case 'device_denied':
      return {
        ...v.device_denied,
        icon: <IconShieldX className="h-6 w-6 text-destructive" />,
        iconWrap: 'bg-destructive/10 ring-1 ring-destructive/20',
        status: 'error',
      }
    case 'invalid_request':
    case 'invalid_grant':
      return {
        ...v.invalid_request,
        icon: <IconAlertTriangle className="h-6 w-6 text-accent" />,
        iconWrap: 'bg-accent/10 ring-1 ring-accent/20',
        status: 'pending',
      }
    case 'login_required':
      return {
        ...v.login_required,
        icon: <IconLogout className="h-6 w-6 text-accent" />,
        iconWrap: 'bg-accent/10 ring-1 ring-accent/20',
        status: 'pending',
      }
    default:
      return {
        ...v.default,
        icon: <IconAlertTriangle className="h-6 w-6 text-accent" />,
        iconWrap: 'bg-accent/10 ring-1 ring-accent/20',
        status: 'error',
      }
  }
}

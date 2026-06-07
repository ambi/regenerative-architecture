import { IconShieldLock } from '@tabler/icons-react'
import type { ReactNode } from 'react'
import { useBranding } from '@/branding/context'
import { useMessages } from '@/i18n/context'

/**
 * 認証系画面 (login / 同意 / device / error) で共通の枠。
 *
 * Keycloak / Okta に近い「中央単一カードに集中させて周辺はミニマル」を
 * 基本としつつ、frontend-design skill の指針に従って次を加える:
 *
 *   - ambient canvas (cool→warm radial + noise) でフラット grey から離れる
 *   - 見出しは IBM Plex Serif で editorial な weight
 *   - asymmetric な status pill (右上) と footer line で生気を出す
 *   - staggered slide-up-fade で初回レンダリングに 1 つだけ delight を仕込む
 *
 * a11y:
 *   - 言語属性は I18nProvider が `<html lang>` から読み取る (サーバ側で付与)
 *   - skip link は `index.html` 直下に出力済み
 *   - status pill の装飾アイコンは `aria-hidden`、テキストはスクリーンリーダ可読
 */
export function AuthLayout({
  title,
  description,
  children,
  footer,
  status = 'secure',
}: {
  title: string
  description?: string
  children: ReactNode
  footer?: ReactNode
  /** 右上に出す status pill。秘匿性の演出と視覚的アクセント。 */
  status?: 'secure' | 'pending' | 'error' | null
}) {
  const brand = useBranding()
  const m = useMessages()
  return (
    <div className="ambient-canvas flex min-h-full flex-col">
      <header className="border-b border-border/60 bg-card/70 backdrop-blur-sm">
        <div className="mx-auto flex h-14 max-w-6xl items-center justify-between gap-2 px-6">
          <div className="flex items-center gap-2.5">
            {brand.logoUrl ? (
              <img src={brand.logoUrl} alt="" className="h-7 w-7 rounded-md" aria-hidden />
            ) : (
              <span
                className="grid h-7 w-7 place-items-center rounded-md bg-primary text-primary-foreground"
                aria-hidden
              >
                <IconShieldLock className="h-4 w-4" />
              </span>
            )}
            <span className="font-serif text-base font-semibold tracking-tight">{brand.name}</span>
          </div>
          {status ? <StatusPill kind={status} label={m.brand[status]} /> : null}
        </div>
      </header>

      <main id="main-content" className="flex flex-1 items-center justify-center px-4 py-12">
        <div className="w-full max-w-md animate-slide-up-fade">
          <div className="mb-8 space-y-2 text-center">
            <h1 className="font-serif text-3xl font-semibold tracking-tight">{title}</h1>
            {description ? (
              <p className="text-sm text-muted-foreground leading-relaxed">{description}</p>
            ) : null}
          </div>
          {children}
          {footer ? (
            <div className="mt-6 text-center text-xs text-muted-foreground leading-relaxed">
              {footer}
            </div>
          ) : null}
        </div>
      </main>

      <footer className="border-t border-border/60 bg-card/70 backdrop-blur-sm">
        <div className="mx-auto flex h-12 max-w-6xl items-center justify-between px-6 text-xs text-muted-foreground">
          <span className="font-mono">{m.layout.footerLeft}</span>
          <span>{m.layout.footerRight}</span>
        </div>
      </footer>
    </div>
  )
}

function StatusPill({
  kind,
  label,
}: {
  kind: 'secure' | 'pending' | 'error'
  label: string
}) {
  const variants = {
    secure: { dot: 'bg-emerald-500' },
    pending: { dot: 'bg-accent' },
    error: { dot: 'bg-destructive' },
  } as const
  const v = variants[kind]
  return (
    <div
      className="flex items-center gap-2 rounded-full bg-card/80 px-2.5 py-1 ring-1 ring-border/60"
      role="status"
      aria-live="polite"
    >
      <span className={`relative h-1.5 w-1.5 rounded-full ${v.dot}`} aria-hidden>
        <span
          className={`absolute inset-0 motion-safe:animate-ping rounded-full opacity-50 ${v.dot}`}
        />
      </span>
      <span className="text-[10px] font-mono uppercase tracking-wider text-muted-foreground">
        {label}
      </span>
    </div>
  )
}

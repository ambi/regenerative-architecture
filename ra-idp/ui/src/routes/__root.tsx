import { Outlet, createRootRoute } from '@tanstack/react-router'
import { ChangePasswordPage } from '@/pages/ChangePasswordPage'
import { ConsentPage } from '@/pages/ConsentPage'
import { DevicePage } from '@/pages/DevicePage'
import { ErrorPage } from '@/pages/ErrorPage'
import { ForgotPasswordPage } from '@/pages/ForgotPasswordPage'
import { LoginPage } from '@/pages/LoginPage'
import { ResetPasswordPage } from '@/pages/ResetPasswordPage'
import { TotpPage } from '@/pages/TotpPage'
import { readMeta } from '@/lib/page-context'

/**
 * バックエンドが `<meta name="ra-idp:page">` でどのページを描画すべきかを伝える。
 * 認証画面は `/login` `/totp` `/consent` の URL で表示するが、SSR shell では
 * meta を権威にして初期ページを dispatch する。
 *
 * meta が無い場合 (純粋な SPA 直接ナビゲーション) のみ URL ベースの file route
 * (login.tsx 等) にフォールバックする。
 */
export const Route = createRootRoute({
  component: PageDispatcher,
})

function PageDispatcher() {
  const page = readMeta('ra-idp:page')
  switch (page) {
    case 'login':
      return <LoginPage />
    case 'totp':
      return <TotpPage />
    case 'consent':
      return <ConsentPage />
    case 'device':
      return <DevicePage />
    case 'change-password':
      return <ChangePasswordPage />
    case 'forgot-password':
      return <ForgotPasswordPage />
    case 'reset-password':
      return <ResetPasswordPage />
    case 'error':
      return <ErrorPage />
    default:
      return <Outlet />
  }
}

import { Outlet, createRootRoute } from '@tanstack/react-router'
import { ConsentPage } from '@/pages/ConsentPage'
import { DevicePage } from '@/pages/DevicePage'
import { ErrorPage } from '@/pages/ErrorPage'
import { LoginPage } from '@/pages/LoginPage'
import { readMeta } from '@/lib/page-context'

/**
 * バックエンドが `<meta name="ra-idp:page">` でどのページを描画すべきかを伝える。
 * URL は `/authorize` `/end_session` のような OAuth エンドポイントになることがあり
 * 「URL = 画面種別」では一致しないため、SPA は meta を権威にして dispatch する。
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
    case 'consent':
      return <ConsentPage />
    case 'device':
      return <DevicePage />
    case 'error':
      return <ErrorPage />
    default:
      return <Outlet />
  }
}

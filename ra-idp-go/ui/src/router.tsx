import {
  createBrowserHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
} from '@tanstack/react-router'
import { useEffect, type ComponentType } from 'react'
import { AuthenticationAPIError, resolvePageData, tenantBasePath, tenantURL } from './api'
import type { PageData } from './types'

// ページごとに props 型 (PageData の各 variant) が異なるため、共通の描画口では any を許す。
// 実際の props は loader が解決した data (kind 一致) を spread するため整合する。
// biome-ignore lint/suspicious/noExplicitAny: per-page props differ; data spread keeps them consistent
type PageComponent = ComponentType<any>
type PageKind = PageData['kind']

// loadPageComponent は kind ごとのページコンポーネントを動的 import する。各ページは個別
// チャンクに code-split され、ロードは route loader 内で await される。描画時に
// React.lazy/Suspense を経由しないため、遷移中は前ページが表示され続け、画面が空白に
// ならない (wi-67)。
const loadPageComponent: Record<PageKind, () => Promise<PageComponent>> = {
  home: () => import('./features/auth-flow/HomePage').then((m) => m.HomePage),
  login: () => import('./features/auth-flow/LoginPage').then((m) => m.LoginPage),
  consent: () => import('./features/auth-flow/ConsentPage').then((m) => m.ConsentPage),
  totp: () => import('./features/auth-flow/TotpPage').then((m) => m.TotpPage),
  device: () => import('./features/auth-flow/DevicePage').then((m) => m.DevicePage),
  status: () => import('./features/auth-flow/StatusPage').then((m) => m.StatusPage),
  callback: () => import('./features/auth-flow/CallbackPage').then((m) => m.CallbackPage),
  'change-password': () =>
    import('./features/account/ChangePasswordPage').then((m) => m.ChangePasswordPage),
  'account-home': () => import('./features/account/AccountHomePage').then((m) => m.AccountHomePage),
  'account-profile': () =>
    import('./features/account/AccountProfilePage').then((m) => m.AccountProfilePage),
  'account-emails': () =>
    import('./features/account/AccountEmailsPage').then((m) => m.AccountEmailsPage),
  'email-verify': () => import('./features/auth-flow/EmailVerifyPage').then((m) => m.EmailVerifyPage),
  'account-applications': () =>
    import('./features/account/AccountApplicationsPage').then((m) => m.AccountApplicationsPage),
  'account-apps': () =>
    import('./features/account/AccountAppsPage').then((m) => m.AccountAppsPage),
  'account-data': () => import('./features/account/AccountDataPage').then((m) => m.AccountDataPage),
  'account-security': () =>
    import('./features/account/AccountSecurityPage').then((m) => m.AccountSecurityPage),
  'account-activity': () =>
    import('./features/account/AccountActivityPage').then((m) => m.AccountActivityPage),
  'forgot-password': () =>
    import('./features/auth-flow/ForgotPasswordPage').then((m) => m.ForgotPasswordPage),
  'reset-password': () =>
    import('./features/auth-flow/ResetPasswordPage').then((m) => m.ResetPasswordPage),
  'admin-dashboard': () =>
    import('./features/admin-dashboard/AdminDashboardPage').then((m) => m.AdminDashboardPage),
  'admin-users': () => import('./features/admin-users/AdminUsersPage').then((m) => m.AdminUsersPage),
  'admin-user-detail': () =>
    import('./features/admin-users/AdminUsersPage').then((m) => m.AdminUserDetailPage),
  'admin-roles': () => import('./features/admin-roles/AdminRolesPage').then((m) => m.AdminRolesPage),
  'admin-role-detail': () =>
    import('./features/admin-roles/AdminRolesPage').then((m) => m.AdminRoleDetailPage),
  'admin-applications': () =>
    import('./features/admin-applications/AdminApplicationsPage').then(
      (m) => m.AdminApplicationsPage,
    ),
  'admin-clients': () =>
    import('./features/admin-clients/AdminClientsPage').then((m) => m.AdminClientsPage),
  'admin-client-detail': () =>
    import('./features/admin-clients/AdminClientsPage').then((m) => m.AdminClientDetailPage),
  'admin-consents': () =>
    import('./features/admin-consents/AdminConsentsPage').then((m) => m.AdminConsentsPage),
  'admin-wsfed-relying-parties': () =>
    import('./features/admin-wsfed/AdminWsFedRelyingPartiesPage').then(
      (m) => m.AdminWsFedRelyingPartiesPage,
    ),
  'admin-authz-detail-types': () =>
    import('./features/admin-authz-detail-types/AdminAuthorizationDetailTypesPage').then(
      (m) => m.AdminAuthorizationDetailTypesPage,
    ),
  'admin-audit-events': () =>
    import('./features/admin-audit-events/AdminAuditEventsPage').then((m) => m.AdminAuditEventsPage),
  'admin-keys': () => import('./features/admin-keys/AdminKeysPage').then((m) => m.AdminKeysPage),
  'admin-tenants': () =>
    import('./features/admin-tenants/AdminTenantsPage').then((m) => m.AdminTenantsPage),
  'admin-groups': () =>
    import('./features/admin-groups/AdminGroupsPage').then((m) => m.AdminGroupsPage),
  'admin-group-detail': () =>
    import('./features/admin-groups/AdminGroupsPage').then((m) => m.AdminGroupDetailPage),
  'admin-agents': () =>
    import('./features/admin-agents/AdminAgentsPage').then((m) => m.AdminAgentsPage),
  'admin-agent-detail': () =>
    import('./features/admin-agents/AdminAgentsPage').then((m) => m.AdminAgentDetailPage),
  'admin-settings': () =>
    import('./features/admin-settings/AdminSettingsPage').then((m) => m.AdminSettingsPage),
  'admin-tenant-attributes': () =>
    import('./features/admin-tenants/AdminTenantAttributesPage').then(
      (m) => m.AdminTenantAttributesPage,
    ),
}

let preloadStarted = false

// preloadPageChunks は全ページの JS chunk をアイドル時にバックグラウンド先読みする。
// loader が chunk を await するため先読みは必須ではないが、初回遷移の loader を高速化する。
export function preloadPageChunks() {
  if (preloadStarted) {
    return
  }
  preloadStarted = true
  const run = () => {
    for (const load of Object.values(loadPageComponent)) {
      void load()
    }
  }
  const idle = (window as { requestIdleCallback?: (cb: () => void) => void }).requestIdleCallback
  if (idle) {
    idle(run)
  } else {
    setTimeout(run, 200)
  }
}

// markPage は描画したページ種別を <meta name="ra-idp:page"> で DOM に表明する。
// SPA dispatcher の分岐を E2E から機械的に検証するための不変条件マーカー (wi-22)。
function markPage(kind: string) {
  let meta = document.head.querySelector<HTMLMetaElement>('meta[name="ra-idp:page"]')
  if (!meta) {
    meta = document.createElement('meta')
    meta.name = 'ra-idp:page'
    document.head.appendChild(meta)
  }
  meta.content = kind
}

type LoadedPage = { data: PageData; Component: PageComponent }

// PageView は loader が解決済みのコンポーネントとデータを描画する。コンポーネントは
// loader 内で読み込み済みのため Suspense を介さず、マウント時に種別を meta へ表明する。
function PageView({ data, Component }: LoadedPage) {
  useEffect(() => {
    markPage(data.kind)
  }, [data.kind])
  return <Component {...data} />
}

// ErrorScreen は loader が投げた認証エラーを、ブート時と同じ案内画面で表示する。
function ErrorScreen({ error }: { error: unknown }) {
  markPage('error')
  const rawMessage =
    error instanceof AuthenticationAPIError ? error.message : '認証画面を読み込めませんでした。'
  const expiredLogin =
    rawMessage.includes('認可トランザクション') || rawMessage.toLowerCase().includes('transaction')
  const title = expiredLogin ? 'ログイン要求が終了しています' : '認証を続行できません'
  const message = expiredLogin
    ? '前のログイン要求は完了または期限切れになっています。利用したい画面をもう一度開いてログインしてください。'
    : rawMessage || '認証画面を読み込めませんでした。もう一度お試しください。'
  return (
    <main className="flex min-h-screen items-center justify-center bg-slate-100 p-6">
      <div className="w-full max-w-md rounded-2xl border bg-white p-8 text-center shadow-lg">
        <h1 className="text-xl font-semibold text-slate-950">{title}</h1>
        <p className="mt-3 text-sm leading-6 text-slate-600">{message}</p>
        {expiredLogin ? (
          <div className="mt-6 grid gap-2">
            <a
              href={tenantURL('/account')}
              className="inline-flex h-10 items-center justify-center rounded-lg bg-slate-950 px-4 text-sm font-semibold text-white hover:bg-slate-800"
            >
              マイページを開く
            </a>
            <a
              href={tenantURL('/admin')}
              className="inline-flex h-10 items-center justify-center rounded-lg border border-slate-300 bg-white px-4 text-sm font-semibold text-slate-800 hover:bg-slate-50"
            >
              管理コンソールを開く
            </a>
          </div>
        ) : null}
      </div>
    </main>
  )
}

const rootRoute = createRootRoute({ component: Outlet })

// PAGE_PATHS は SPA が扱う全パス。各パスは共有 loader (resolvePageData + コンポーネント
// ロード) と PageView を持つ。
const PAGE_PATHS = [
  '/',
  '/login',
  '/consent',
  '/totp',
  '/device',
  '/status',
  '/callback',
  '/account/password',
  '/account',
  '/account/profile',
  '/account/emails',
  '/account/email/verify',
  '/account/applications',
  '/account/apps',
  '/account/data',
  '/account/security',
  '/account/activity',
  '/forgot_password',
  '/reset_password',
  '/admin',
  '/admin/users',
  '/admin/users/$sub',
  '/admin/roles',
  '/admin/roles/$name',
  '/admin/applications',
  '/admin/clients',
  '/admin/clients/$clientId',
  '/admin/consents',
  '/admin/wsfed/relying-parties',
  '/admin/authorization-detail-types',
  '/admin/audit_events',
  '/admin/keys',
  '/admin/tenants',
  '/admin/groups',
  '/admin/groups/$groupId',
  '/admin/agents',
  '/admin/agents/$agentId',
  '/admin/settings',
  '/admin/tenant/attributes',
] as const

function makePageRoute(path: string) {
  const route = createRoute({
    getParentRoute: () => rootRoute,
    path,
    // loader は遷移先 location のデータとページコンポーネントの両方を解決する。両方が
    // 揃うまで TanStack Router は前ページを表示し続けるため、遷移時に空白が出ない (wi-67)。
    loader: async ({ location }): Promise<LoadedPage> => {
      const data = await resolvePageData({
        pathname: location.pathname,
        search: location.searchStr,
      })
      const Component = await loadPageComponent[data.kind]()
      return { data, Component }
    },
    component: () => {
      const { data, Component } = route.useLoaderData() as LoadedPage
      return <PageView data={data} Component={Component} />
    },
  })
  return route
}

export function createAppRouter() {
  return createRouter({
    routeTree: rootRoute.addChildren(PAGE_PATHS.map(makePageRoute)),
    history: createBrowserHistory(),
    basepath: tenantBasePath() || '/',
    // pendingComponent を設定しないことで、loader 解決中は前ページを表示したままにする
    // (全画面の空白を出さない)。コンポーネントは loader で await 済みのため Suspense も挟まない。
    defaultErrorComponent: ({ error }) => <ErrorScreen error={error} />,
  })
}

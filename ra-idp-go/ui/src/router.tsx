import {
  Outlet,
  createBrowserHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import { Suspense, lazy, useEffect, type ComponentType, type ReactNode } from 'react'
import type { PageData } from './types'
import { AuthenticationAPIError, resolvePageData, tenantBasePath, tenantURL } from './api'

function namedPage(loader: () => Promise<Record<string, unknown>>, exportName: string) {
  return lazy(async () => {
    const module = await loader()
    return { default: module[exportName] as ComponentType<object> }
  })
}

const AccountActivityPage = namedPage(
  () => import('./features/account/AccountActivityPage'),
  'AccountActivityPage',
)
const AccountApplicationsPage = namedPage(
  () => import('./features/account/AccountApplicationsPage'),
  'AccountApplicationsPage',
)
const AccountDataPage = namedPage(
  () => import('./features/account/AccountDataPage'),
  'AccountDataPage',
)
const AccountEmailsPage = namedPage(
  () => import('./features/account/AccountEmailsPage'),
  'AccountEmailsPage',
)
const AccountHomePage = namedPage(
  () => import('./features/account/AccountHomePage'),
  'AccountHomePage',
)
const AccountProfilePage = namedPage(
  () => import('./features/account/AccountProfilePage'),
  'AccountProfilePage',
)
const AccountSecurityPage = namedPage(
  () => import('./features/account/AccountSecurityPage'),
  'AccountSecurityPage',
)
const AdminAuditEventsPage = namedPage(
  () => import('./features/admin-audit-events/AdminAuditEventsPage'),
  'AdminAuditEventsPage',
)
const AdminClientDetailPage = namedPage(
  () => import('./features/admin-clients/AdminClientsPage'),
  'AdminClientDetailPage',
)
const AdminClientsPage = namedPage(
  () => import('./features/admin-clients/AdminClientsPage'),
  'AdminClientsPage',
)
const AdminConsentsPage = namedPage(
  () => import('./features/admin-consents/AdminConsentsPage'),
  'AdminConsentsPage',
)
const AdminAuthorizationDetailTypesPage = namedPage(
  () => import('./features/admin-authz-detail-types/AdminAuthorizationDetailTypesPage'),
  'AdminAuthorizationDetailTypesPage',
)
const AdminDashboardPage = namedPage(
  () => import('./features/admin-dashboard/AdminDashboardPage'),
  'AdminDashboardPage',
)
const AdminGroupDetailPage = namedPage(
  () => import('./features/admin-groups/AdminGroupsPage'),
  'AdminGroupDetailPage',
)
const AdminGroupsPage = namedPage(
  () => import('./features/admin-groups/AdminGroupsPage'),
  'AdminGroupsPage',
)
const AdminAgentDetailPage = namedPage(
  () => import('./features/admin-agents/AdminAgentsPage'),
  'AdminAgentDetailPage',
)
const AdminAgentsPage = namedPage(
  () => import('./features/admin-agents/AdminAgentsPage'),
  'AdminAgentsPage',
)
const AdminKeysPage = namedPage(
  () => import('./features/admin-keys/AdminKeysPage'),
  'AdminKeysPage',
)
const AdminRoleDetailPage = namedPage(
  () => import('./features/admin-roles/AdminRolesPage'),
  'AdminRoleDetailPage',
)
const AdminRolesPage = namedPage(
  () => import('./features/admin-roles/AdminRolesPage'),
  'AdminRolesPage',
)
const AdminSettingsPage = namedPage(
  () => import('./features/admin-settings/AdminSettingsPage'),
  'AdminSettingsPage',
)
const AdminTenantAttributesPage = namedPage(
  () => import('./features/admin-tenants/AdminTenantAttributesPage'),
  'AdminTenantAttributesPage',
)
const AdminTenantsPage = namedPage(
  () => import('./features/admin-tenants/AdminTenantsPage'),
  'AdminTenantsPage',
)
const AdminUserDetailPage = namedPage(
  () => import('./features/admin-users/AdminUsersPage'),
  'AdminUserDetailPage',
)
const AdminUsersPage = namedPage(
  () => import('./features/admin-users/AdminUsersPage'),
  'AdminUsersPage',
)
const AdminWsFedRelyingPartiesPage = namedPage(
  () => import('./features/admin-wsfed/AdminWsFedRelyingPartiesPage'),
  'AdminWsFedRelyingPartiesPage',
)
const CallbackPage = namedPage(() => import('./features/auth-flow/CallbackPage'), 'CallbackPage')
const ChangePasswordPage = namedPage(
  () => import('./features/account/ChangePasswordPage'),
  'ChangePasswordPage',
)
const ConsentPage = namedPage(() => import('./features/auth-flow/ConsentPage'), 'ConsentPage')
const DevicePage = namedPage(() => import('./features/auth-flow/DevicePage'), 'DevicePage')
const EmailVerifyPage = namedPage(
  () => import('./features/auth-flow/EmailVerifyPage'),
  'EmailVerifyPage',
)
const ForgotPasswordPage = namedPage(
  () => import('./features/auth-flow/ForgotPasswordPage'),
  'ForgotPasswordPage',
)
const HomePage = namedPage(() => import('./features/auth-flow/HomePage'), 'HomePage')
const LoginPage = namedPage(() => import('./features/auth-flow/LoginPage'), 'LoginPage')
const ResetPasswordPage = namedPage(
  () => import('./features/auth-flow/ResetPasswordPage'),
  'ResetPasswordPage',
)
const StatusPage = namedPage(() => import('./features/auth-flow/StatusPage'), 'StatusPage')
const TotpPage = namedPage(() => import('./features/auth-flow/TotpPage'), 'TotpPage')

function routePage(page: ReactNode) {
  return <Suspense fallback={<div className="min-h-screen bg-slate-50" />}>{page}</Suspense>
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

// renderPage は解決済み PageData を対応するページコンポーネントへ振り分ける。
function renderPage(data: PageData): ReactNode {
  switch (data.kind) {
    case 'home':
      return <HomePage {...data} />
    case 'login':
      return <LoginPage {...data} />
    case 'consent':
      return <ConsentPage {...data} />
    case 'totp':
      return <TotpPage {...data} />
    case 'device':
      return <DevicePage {...data} />
    case 'status':
      return <StatusPage {...data} />
    case 'callback':
      return <CallbackPage {...data} />
    case 'change-password':
      return <ChangePasswordPage {...data} />
    case 'account-home':
      return <AccountHomePage {...data} />
    case 'account-profile':
      return <AccountProfilePage {...data} />
    case 'account-emails':
      return <AccountEmailsPage {...data} />
    case 'email-verify':
      return <EmailVerifyPage {...data} />
    case 'account-applications':
      return <AccountApplicationsPage {...data} />
    case 'account-data':
      return <AccountDataPage {...data} />
    case 'account-security':
      return <AccountSecurityPage {...data} />
    case 'account-activity':
      return <AccountActivityPage {...data} />
    case 'forgot-password':
      return <ForgotPasswordPage {...data} />
    case 'reset-password':
      return <ResetPasswordPage {...data} />
    case 'admin-dashboard':
      return <AdminDashboardPage {...data} />
    case 'admin-users':
      return <AdminUsersPage {...data} />
    case 'admin-user-detail':
      return <AdminUserDetailPage {...data} />
    case 'admin-roles':
      return <AdminRolesPage {...data} />
    case 'admin-role-detail':
      return <AdminRoleDetailPage {...data} />
    case 'admin-clients':
      return <AdminClientsPage {...data} />
    case 'admin-client-detail':
      return <AdminClientDetailPage {...data} />
    case 'admin-consents':
      return <AdminConsentsPage {...data} />
    case 'admin-wsfed-relying-parties':
      return <AdminWsFedRelyingPartiesPage {...data} />
    case 'admin-authz-detail-types':
      return <AdminAuthorizationDetailTypesPage {...data} />
    case 'admin-audit-events':
      return <AdminAuditEventsPage {...data} />
    case 'admin-keys':
      return <AdminKeysPage {...data} />
    case 'admin-tenants':
      return <AdminTenantsPage {...data} />
    case 'admin-groups':
      return <AdminGroupsPage {...data} />
    case 'admin-group-detail':
      return <AdminGroupDetailPage {...data} />
    case 'admin-agents':
      return <AdminAgentsPage {...data} />
    case 'admin-agent-detail':
      return <AdminAgentDetailPage {...data} />
    case 'admin-settings':
      return <AdminSettingsPage {...data} />
    case 'admin-tenant-attributes':
      return <AdminTenantAttributesPage {...data} />
    default:
      return null
  }
}

// PageContent は遅延ロードされたページを描画し、マウント後に種別を meta へ表明する。
// markPage を Suspense 境界の内側で呼ぶことで、lazy chunk のロード完了 (= ページ DOM が
// 実在する) 時点で meta が更新される (E2E の不変条件が描画と一致する、wi-67)。
function PageContent({ data }: { data: PageData }) {
  useEffect(() => {
    markPage(data.kind)
  }, [data.kind])
  return renderPage(data)
}

// PageView は loader が解決した PageData を Suspense 境界つきで描画する。
function PageView({ data }: { data: PageData }) {
  return routePage(<PageContent data={data} />)
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

// PAGE_PATHS は SPA が扱う全パス。各パスは共有 loader (resolvePageData) と PageView を持つ。
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
    // loader は遷移先 location でページデータを取得する。client-side 遷移では window への
    // 依存を持たず、その route のデータだけを取得する (wi-67)。
    loader: ({ location }) =>
      resolvePageData({ pathname: location.pathname, search: location.searchStr }),
    component: () => <PageView data={route.useLoaderData() as PageData} />,
  })
  return route
}

export function createAppRouter() {
  return createRouter({
    routeTree: rootRoute.addChildren(PAGE_PATHS.map(makePageRoute)),
    history: createBrowserHistory(),
    basepath: tenantBasePath() || '/',
    defaultPendingComponent: () => <div className="min-h-screen bg-slate-50" />,
    defaultErrorComponent: ({ error }) => <ErrorScreen error={error} />,
  })
}

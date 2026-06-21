import {
  Outlet,
  createBrowserHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import { Suspense, lazy, type ComponentType, type ReactNode } from 'react'
import type { PageData } from './types'
import { tenantBasePath } from './api'

function namedPage(loader: () => Promise<Record<string, unknown>>, exportName: string) {
  return lazy(async () => {
    const module = await loader()
    return { default: module[exportName] as ComponentType<object> }
  })
}

const AccountActivityPage = namedPage(
  () => import('./pages/AccountActivityPage'),
  'AccountActivityPage',
)
const AccountApplicationsPage = namedPage(
  () => import('./pages/AccountApplicationsPage'),
  'AccountApplicationsPage',
)
const AccountDataPage = namedPage(() => import('./pages/AccountDataPage'), 'AccountDataPage')
const AccountEmailsPage = namedPage(() => import('./pages/AccountEmailsPage'), 'AccountEmailsPage')
const AccountHomePage = namedPage(() => import('./pages/AccountHomePage'), 'AccountHomePage')
const AccountProfilePage = namedPage(
  () => import('./pages/AccountProfilePage'),
  'AccountProfilePage',
)
const AccountSecurityPage = namedPage(
  () => import('./pages/AccountSecurityPage'),
  'AccountSecurityPage',
)
const AdminAuditEventsPage = namedPage(
  () => import('./pages/AdminAuditEventsPage'),
  'AdminAuditEventsPage',
)
const AdminClientDetailPage = namedPage(
  () => import('./pages/AdminClientsPage'),
  'AdminClientDetailPage',
)
const AdminClientsPage = namedPage(() => import('./pages/AdminClientsPage'), 'AdminClientsPage')
const AdminConsentsPage = namedPage(() => import('./pages/AdminConsentsPage'), 'AdminConsentsPage')
const AdminDashboardPage = namedPage(
  () => import('./pages/AdminDashboardPage'),
  'AdminDashboardPage',
)
const AdminGroupDetailPage = namedPage(
  () => import('./pages/AdminGroupsPage'),
  'AdminGroupDetailPage',
)
const AdminGroupsPage = namedPage(() => import('./pages/AdminGroupsPage'), 'AdminGroupsPage')
const AdminKeysPage = namedPage(() => import('./pages/AdminKeysPage'), 'AdminKeysPage')
const AdminRoleDetailPage = namedPage(() => import('./pages/AdminRolesPage'), 'AdminRoleDetailPage')
const AdminRolesPage = namedPage(() => import('./pages/AdminRolesPage'), 'AdminRolesPage')
const AdminSettingsPage = namedPage(() => import('./pages/AdminSettingsPage'), 'AdminSettingsPage')
const AdminTenantAttributesPage = namedPage(
  () => import('./pages/AdminTenantAttributesPage'),
  'AdminTenantAttributesPage',
)
const AdminTenantsPage = namedPage(() => import('./pages/AdminTenantsPage'), 'AdminTenantsPage')
const AdminUserDetailPage = namedPage(() => import('./pages/AdminUsersPage'), 'AdminUserDetailPage')
const AdminUsersPage = namedPage(() => import('./pages/AdminUsersPage'), 'AdminUsersPage')
const CallbackPage = namedPage(() => import('./pages/CallbackPage'), 'CallbackPage')
const ChangePasswordPage = namedPage(
  () => import('./pages/ChangePasswordPage'),
  'ChangePasswordPage',
)
const ConsentPage = namedPage(() => import('./pages/ConsentPage'), 'ConsentPage')
const DevicePage = namedPage(() => import('./pages/DevicePage'), 'DevicePage')
const EmailVerifyPage = namedPage(() => import('./pages/EmailVerifyPage'), 'EmailVerifyPage')
const ForgotPasswordPage = namedPage(
  () => import('./pages/ForgotPasswordPage'),
  'ForgotPasswordPage',
)
const HomePage = namedPage(() => import('./pages/HomePage'), 'HomePage')
const LoginPage = namedPage(() => import('./pages/LoginPage'), 'LoginPage')
const ResetPasswordPage = namedPage(
  () => import('./pages/ResetPasswordPage'),
  'ResetPasswordPage',
)
const StatusPage = namedPage(() => import('./pages/StatusPage'), 'StatusPage')
const TotpPage = namedPage(() => import('./pages/TotpPage'), 'TotpPage')

function routePage(page: ReactNode) {
  return <Suspense fallback={<div className="min-h-screen bg-slate-50" />}>{page}</Suspense>
}

const rootRoute = createRootRoute({
  component: Outlet,
})

export function createAppRouter(data: PageData) {
  const homeRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/',
    component: () => (data.kind === 'home' ? routePage(<HomePage {...data} />) : null),
  })
  const loginRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/login',
    component: () => (data.kind === 'login' ? routePage(<LoginPage {...data} />) : null),
  })
  const consentRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/consent',
    component: () => (data.kind === 'consent' ? routePage(<ConsentPage {...data} />) : null),
  })
  const totpRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/totp',
    component: () => (data.kind === 'totp' ? routePage(<TotpPage {...data} />) : null),
  })
  const deviceRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/device',
    component: () => (data.kind === 'device' ? routePage(<DevicePage {...data} />) : null),
  })
  const statusRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/status',
    component: () => (data.kind === 'status' ? routePage(<StatusPage {...data} />) : null),
  })
  const callbackRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/callback',
    component: () => (data.kind === 'callback' ? routePage(<CallbackPage {...data} />) : null),
  })
  const changePasswordRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/account/password',
    component: () =>
      data.kind === 'change-password' ? routePage(<ChangePasswordPage {...data} />) : null,
  })
  const accountHomeRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/account',
    component: () =>
      data.kind === 'account-home' ? routePage(<AccountHomePage {...data} />) : null,
  })
  const accountProfileRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/account/profile',
    component: () =>
      data.kind === 'account-profile' ? routePage(<AccountProfilePage {...data} />) : null,
  })
  const accountEmailsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/account/emails',
    component: () =>
      data.kind === 'account-emails' ? routePage(<AccountEmailsPage {...data} />) : null,
  })
  const emailVerifyRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/account/email/verify',
    component: () =>
      data.kind === 'email-verify' ? routePage(<EmailVerifyPage {...data} />) : null,
  })
  const accountApplicationsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/account/applications',
    component: () =>
      data.kind === 'account-applications' ? routePage(<AccountApplicationsPage {...data} />) : null,
  })
  const accountDataRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/account/data',
    component: () =>
      data.kind === 'account-data' ? routePage(<AccountDataPage {...data} />) : null,
  })
  const accountSecurityRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/account/security',
    component: () =>
      data.kind === 'account-security' ? routePage(<AccountSecurityPage {...data} />) : null,
  })
  const accountActivityRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/account/activity',
    component: () =>
      data.kind === 'account-activity' ? routePage(<AccountActivityPage {...data} />) : null,
  })
  const forgotPasswordRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/forgot_password',
    component: () =>
      data.kind === 'forgot-password' ? routePage(<ForgotPasswordPage {...data} />) : null,
  })
  const resetPasswordRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/reset_password',
    component: () =>
      data.kind === 'reset-password' ? routePage(<ResetPasswordPage {...data} />) : null,
  })
  const adminDashboardRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin',
    component: () =>
      data.kind === 'admin-dashboard' ? routePage(<AdminDashboardPage {...data} />) : null,
  })
  const adminUsersRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/users',
    component: () =>
      data.kind === 'admin-users' ? routePage(<AdminUsersPage {...data} />) : null,
  })
  const adminUserDetailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/users/$sub',
    component: () =>
      data.kind === 'admin-user-detail' ? routePage(<AdminUserDetailPage {...data} />) : null,
  })
  const adminRolesRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/roles',
    component: () =>
      data.kind === 'admin-roles' ? routePage(<AdminRolesPage {...data} />) : null,
  })
  const adminRoleDetailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/roles/$name',
    component: () =>
      data.kind === 'admin-role-detail' ? routePage(<AdminRoleDetailPage {...data} />) : null,
  })
  const adminClientsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/clients',
    component: () =>
      data.kind === 'admin-clients' ? routePage(<AdminClientsPage {...data} />) : null,
  })
  const adminClientDetailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/clients/$clientId',
    component: () =>
      data.kind === 'admin-client-detail' ? routePage(<AdminClientDetailPage {...data} />) : null,
  })
  const adminConsentsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/consents',
    component: () =>
      data.kind === 'admin-consents' ? routePage(<AdminConsentsPage {...data} />) : null,
  })
  const adminAuditEventsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/audit_events',
    component: () =>
      data.kind === 'admin-audit-events' ? routePage(<AdminAuditEventsPage {...data} />) : null,
  })
  const adminKeysRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/keys',
    component: () => (data.kind === 'admin-keys' ? routePage(<AdminKeysPage {...data} />) : null),
  })
  const adminTenantsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/tenants',
    component: () =>
      data.kind === 'admin-tenants' ? routePage(<AdminTenantsPage {...data} />) : null,
  })
  const adminGroupsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/groups',
    component: () =>
      data.kind === 'admin-groups' ? routePage(<AdminGroupsPage {...data} />) : null,
  })
  const adminGroupDetailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/groups/$groupId',
    component: () =>
      data.kind === 'admin-group-detail' ? routePage(<AdminGroupDetailPage {...data} />) : null,
  })
  const adminSettingsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/settings',
    component: () =>
      data.kind === 'admin-settings' ? routePage(<AdminSettingsPage {...data} />) : null,
  })
  const adminTenantAttributesRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/tenant/attributes',
    component: () =>
      data.kind === 'admin-tenant-attributes'
        ? routePage(<AdminTenantAttributesPage {...data} />)
        : null,
  })

  return createRouter({
    routeTree: rootRoute.addChildren([
      homeRoute,
      loginRoute,
      totpRoute,
      consentRoute,
      deviceRoute,
      statusRoute,
      callbackRoute,
      changePasswordRoute,
      accountHomeRoute,
      accountProfileRoute,
      accountEmailsRoute,
      emailVerifyRoute,
      accountApplicationsRoute,
      accountDataRoute,
      accountSecurityRoute,
      accountActivityRoute,
      forgotPasswordRoute,
      resetPasswordRoute,
      adminDashboardRoute,
      adminUsersRoute,
      adminUserDetailRoute,
      adminRolesRoute,
      adminRoleDetailRoute,
      adminClientsRoute,
      adminClientDetailRoute,
      adminConsentsRoute,
      adminAuditEventsRoute,
      adminKeysRoute,
      adminTenantsRoute,
      adminGroupsRoute,
      adminGroupDetailRoute,
      adminSettingsRoute,
      adminTenantAttributesRoute,
    ]),
    history: createBrowserHistory(),
    basepath: tenantBasePath() || '/',
  })
}

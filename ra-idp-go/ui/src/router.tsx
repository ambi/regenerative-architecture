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
      data.kind === 'account-applications'
        ? routePage(<AccountApplicationsPage {...data} />)
        : null,
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
    component: () => (data.kind === 'admin-users' ? routePage(<AdminUsersPage {...data} />) : null),
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
    component: () => (data.kind === 'admin-roles' ? routePage(<AdminRolesPage {...data} />) : null),
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
  const adminAuthzDetailTypesRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/authorization-detail-types',
    component: () =>
      data.kind === 'admin-authz-detail-types'
        ? routePage(<AdminAuthorizationDetailTypesPage {...data} />)
        : null,
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
  const adminAgentsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/agents',
    component: () =>
      data.kind === 'admin-agents' ? routePage(<AdminAgentsPage {...data} />) : null,
  })
  const adminAgentDetailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/agents/$agentId',
    component: () =>
      data.kind === 'admin-agent-detail' ? routePage(<AdminAgentDetailPage {...data} />) : null,
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
      adminAuthzDetailTypesRoute,
      adminClientDetailRoute,
      adminConsentsRoute,
      adminAuditEventsRoute,
      adminKeysRoute,
      adminTenantsRoute,
      adminGroupsRoute,
      adminGroupDetailRoute,
      adminAgentsRoute,
      adminAgentDetailRoute,
      adminSettingsRoute,
      adminTenantAttributesRoute,
    ]),
    history: createBrowserHistory(),
    basepath: tenantBasePath() || '/',
  })
}

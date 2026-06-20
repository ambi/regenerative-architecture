import {
  Outlet,
  createBrowserHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import { AccountHomePage } from './pages/AccountHomePage'
import { AccountProfilePage } from './pages/AccountProfilePage'
import { CallbackPage } from './pages/CallbackPage'
import { AdminAuditEventsPage } from './pages/AdminAuditEventsPage'
import { AdminClientDetailPage, AdminClientsPage } from './pages/AdminClientsPage'
import { AdminConsentsPage } from './pages/AdminConsentsPage'
import { AdminDashboardPage } from './pages/AdminDashboardPage'
import { AdminGroupDetailPage, AdminGroupsPage } from './pages/AdminGroupsPage'
import { AdminKeysPage } from './pages/AdminKeysPage'
import { AdminRoleDetailPage, AdminRolesPage } from './pages/AdminRolesPage'
import { AdminSettingsPage } from './pages/AdminSettingsPage'
import { AdminTenantAttributesPage } from './pages/AdminTenantAttributesPage'
import { AdminTenantsPage } from './pages/AdminTenantsPage'
import { AdminUserDetailPage, AdminUsersPage } from './pages/AdminUsersPage'
import { ChangePasswordPage } from './pages/ChangePasswordPage'
import { ConsentPage } from './pages/ConsentPage'
import { DevicePage } from './pages/DevicePage'
import { HomePage } from './pages/HomePage'
import { LoginPage } from './pages/LoginPage'
import { ForgotPasswordPage } from './pages/ForgotPasswordPage'
import { ResetPasswordPage } from './pages/ResetPasswordPage'
import { StatusPage } from './pages/StatusPage'
import { TotpPage } from './pages/TotpPage'
import type { PageData } from './types'
import { tenantBasePath } from './api'

const rootRoute = createRootRoute({
  component: Outlet,
})

export function createAppRouter(data: PageData) {
  const homeRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/',
    component: () => (data.kind === 'home' ? <HomePage {...data} /> : null),
  })
  const loginRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/login',
    component: () => (data.kind === 'login' ? <LoginPage {...data} /> : null),
  })
  const consentRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/consent',
    component: () => (data.kind === 'consent' ? <ConsentPage {...data} /> : null),
  })
  const totpRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/totp',
    component: () => (data.kind === 'totp' ? <TotpPage {...data} /> : null),
  })
  const deviceRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/device',
    component: () => (data.kind === 'device' ? <DevicePage {...data} /> : null),
  })
  const statusRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/status',
    component: () => (data.kind === 'status' ? <StatusPage {...data} /> : null),
  })
  const callbackRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/callback',
    component: () => (data.kind === 'callback' ? <CallbackPage {...data} /> : null),
  })
  const changePasswordRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/account/password',
    component: () => (data.kind === 'change-password' ? <ChangePasswordPage {...data} /> : null),
  })
  const accountHomeRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/account',
    component: () => (data.kind === 'account-home' ? <AccountHomePage {...data} /> : null),
  })
  const accountProfileRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/account/profile',
    component: () => (data.kind === 'account-profile' ? <AccountProfilePage {...data} /> : null),
  })
  const forgotPasswordRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/forgot_password',
    component: () => (data.kind === 'forgot-password' ? <ForgotPasswordPage {...data} /> : null),
  })
  const resetPasswordRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/reset_password',
    component: () => (data.kind === 'reset-password' ? <ResetPasswordPage {...data} /> : null),
  })
  const adminDashboardRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin',
    component: () =>
      data.kind === 'admin-dashboard' ? <AdminDashboardPage {...data} /> : null,
  })
  const adminUsersRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/users',
    component: () => (data.kind === 'admin-users' ? <AdminUsersPage {...data} /> : null),
  })
  const adminUserDetailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/users/$sub',
    component: () =>
      data.kind === 'admin-user-detail' ? <AdminUserDetailPage {...data} /> : null,
  })
  const adminRolesRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/roles',
    component: () => (data.kind === 'admin-roles' ? <AdminRolesPage {...data} /> : null),
  })
  const adminRoleDetailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/roles/$name',
    component: () =>
      data.kind === 'admin-role-detail' ? <AdminRoleDetailPage {...data} /> : null,
  })
  const adminClientsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/clients',
    component: () => (data.kind === 'admin-clients' ? <AdminClientsPage {...data} /> : null),
  })
  const adminClientDetailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/clients/$clientId',
    component: () =>
      data.kind === 'admin-client-detail' ? <AdminClientDetailPage {...data} /> : null,
  })
  const adminConsentsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/consents',
    component: () => (data.kind === 'admin-consents' ? <AdminConsentsPage {...data} /> : null),
  })
  const adminAuditEventsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/audit_events',
    component: () =>
      data.kind === 'admin-audit-events' ? <AdminAuditEventsPage {...data} /> : null,
  })
  const adminKeysRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/keys',
    component: () => (data.kind === 'admin-keys' ? <AdminKeysPage {...data} /> : null),
  })
  const adminTenantsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/tenants',
    component: () => (data.kind === 'admin-tenants' ? <AdminTenantsPage {...data} /> : null),
  })
  const adminGroupsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/groups',
    component: () => (data.kind === 'admin-groups' ? <AdminGroupsPage {...data} /> : null),
  })
  const adminGroupDetailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/groups/$groupId',
    component: () =>
      data.kind === 'admin-group-detail' ? <AdminGroupDetailPage {...data} /> : null,
  })
  const adminSettingsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/settings',
    component: () => (data.kind === 'admin-settings' ? <AdminSettingsPage {...data} /> : null),
  })
  const adminTenantAttributesRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/tenant/attributes',
    component: () =>
      data.kind === 'admin-tenant-attributes' ? <AdminTenantAttributesPage {...data} /> : null,
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

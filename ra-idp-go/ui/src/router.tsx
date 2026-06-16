import {
  Outlet,
  createBrowserHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import { CallbackPage } from './pages/CallbackPage'
import { AdminAuditEventsPage } from './pages/AdminAuditEventsPage'
import { AdminClientsPage } from './pages/AdminClientsPage'
import { AdminConsentsPage } from './pages/AdminConsentsPage'
import { AdminDashboardPage } from './pages/AdminDashboardPage'
import { AdminKeysPage } from './pages/AdminKeysPage'
import { AdminRolesPage } from './pages/AdminRolesPage'
import { AdminSettingsPage } from './pages/AdminSettingsPage'
import { AdminTenantsPage } from './pages/AdminTenantsPage'
import { AdminUsersPage } from './pages/AdminUsersPage'
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
  const adminRolesRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/roles',
    component: () => (data.kind === 'admin-roles' ? <AdminRolesPage {...data} /> : null),
  })
  const adminClientsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/clients',
    component: () => (data.kind === 'admin-clients' ? <AdminClientsPage {...data} /> : null),
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
  const adminSettingsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/admin/settings',
    component: () => (data.kind === 'admin-settings' ? <AdminSettingsPage {...data} /> : null),
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
      forgotPasswordRoute,
      resetPasswordRoute,
      adminDashboardRoute,
      adminUsersRoute,
      adminRolesRoute,
      adminClientsRoute,
      adminConsentsRoute,
      adminAuditEventsRoute,
      adminKeysRoute,
      adminTenantsRoute,
      adminSettingsRoute,
    ]),
    history: createBrowserHistory(),
    basepath: tenantBasePath() || '/',
  })
}

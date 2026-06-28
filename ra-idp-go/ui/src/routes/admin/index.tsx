import { createFileRoute } from '@tanstack/react-router'
import { listAdminAuditEvents } from '../../api/admin'
import { request } from '../../api/core'
import { AdminDashboardPage } from '../../features/admin-dashboard/AdminDashboardPage'
import type { AdminOAuth2Client, AdminConsent, AdminUser } from '../../types'
import { requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

type AdminUserListResponse = { users: AdminUser[] }
type AdminOAuth2ClientListResponse = { clients: AdminOAuth2Client[] }
type AdminConsentListResponse = { consents: AdminConsent[] }

export const Route = createFileRoute('/admin/')({
  loader: async ({ location }) => {
    const account = await requirePortalAccount('admin', location.pathname, location.searchStr)
    const since = new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString()
    const [users, clients, consents, recentEvents] = await Promise.all([
      request<AdminUserListResponse>('/api/admin/users'),
      request<AdminOAuth2ClientListResponse>('/api/admin/clients'),
      request<AdminConsentListResponse>('/api/admin/consents'),
      listAdminAuditEvents({ after: since, limit: 100 }),
    ])
    const activeUserCount = users.users.filter((u) => !u.disabled_at).length
    return {
      csrfToken: account.csrf_token,
      actorUsername: account.preferred_username,
      actorRoles: account.roles ?? [],
      actorTenantID: account.tenant_id ?? '',
      userCount: users.users.length,
      activeUserCount,
      disabledUserCount: users.users.length - activeUserCount,
      clientCount: clients.clients.length,
      grantedConsentCount: consents.consents.filter((c) => c.state === 'granted').length,
      auditEventCount24h: recentEvents.length,
      recentEvents: recentEvents.slice(0, 5),
    }
  },
  component: AdminDashboardRoute,
})

function AdminDashboardRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind="admin-dashboard">
      <AdminDashboardPage {...data} />
    </PageMarker>
  )
}

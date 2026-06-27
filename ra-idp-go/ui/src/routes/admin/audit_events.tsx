import { createFileRoute } from '@tanstack/react-router'
import { request } from '../../api/core'
import { AdminAuditEventsPage } from '../../features/admin-audit-events/AdminAuditEventsPage'
import type { AdminAuditEvent } from '../../types'
import { requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

type AdminAuditEventListResponse = { events: AdminAuditEvent[] }

export const Route = createFileRoute('/admin/audit_events')({
  loader: async ({ location }) => {
    const account = await requirePortalAccount('admin', location.pathname, location.searchStr)
    const events = await request<AdminAuditEventListResponse>('/api/admin/audit_events')
    return {
      csrfToken: account.csrf_token,
      actorUsername: account.preferred_username,
      actorRoles: account.roles ?? [],
      actorTenantID: account.tenant_id ?? '',
      events: events.events,
    }
  },
  component: AdminAuditEventsRoute,
})

function AdminAuditEventsRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind="admin-audit-events">
      <AdminAuditEventsPage {...data} />
    </PageMarker>
  )
}

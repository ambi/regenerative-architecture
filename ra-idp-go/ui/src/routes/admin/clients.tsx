import { createFileRoute } from '@tanstack/react-router'
import { request } from '../../api/core'
import { AdminClientsPage } from '../../features/admin-clients/AdminClientsPage'
import type { AdminClient } from '../../types'
import { requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

type AdminClientListResponse = { clients: AdminClient[] }

export const Route = createFileRoute('/admin/clients')({
  loader: async ({ location }) => {
    const account = await requirePortalAccount('admin', location.pathname, location.searchStr)
    const clients = await request<AdminClientListResponse>('/api/admin/clients')
    return {
      kind: 'admin-clients',
      csrfToken: account.csrf_token,
      actorUsername: account.preferred_username,
      clients: clients.clients,
    }
  },
  component: AdminClientsRoute,
})

function AdminClientsRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind={data.kind}>
      <AdminClientsPage {...data} />
    </PageMarker>
  )
}

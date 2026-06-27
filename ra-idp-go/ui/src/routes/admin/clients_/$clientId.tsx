import { createFileRoute } from '@tanstack/react-router'
import { getAdminClient } from '../../../api/admin'
import { AdminClientDetailPage } from '../../../features/admin-clients/AdminClientsPage'
import type { AdminClientDetailPage as AdminClientDetailPageData } from '../../../types'
import { requirePortalAccount } from '../../-guards'
import { PageMarker } from '../../-page'

export const Route = createFileRoute('/admin/clients_/$clientId')({
  loader: async ({ location, params }): Promise<AdminClientDetailPageData> => {
    const account = await requirePortalAccount('admin', location.pathname, location.searchStr)
    const client = await getAdminClient(params.clientId)
    return {
      kind: 'admin-client-detail',
      csrfToken: account.csrf_token,
      actorUsername: account.preferred_username,
      client,
    }
  },
  component: AdminClientDetailRoute,
})

function AdminClientDetailRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind={data.kind}>
      <AdminClientDetailPage {...data} />
    </PageMarker>
  )
}

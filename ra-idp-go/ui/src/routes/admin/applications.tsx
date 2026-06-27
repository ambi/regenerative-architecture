import { createFileRoute } from '@tanstack/react-router'
import { listAdminApplications } from '../../api/admin'
import { AdminApplicationsPage } from '../../features/admin-applications/AdminApplicationsPage'
import { requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

export const Route = createFileRoute('/admin/applications')({
  loader: async ({ location }) => {
    const account = await requirePortalAccount('admin', location.pathname, location.searchStr)
    const applications = await listAdminApplications()
    return {
      kind: 'admin-applications',
      csrfToken: account.csrf_token,
      actorUsername: account.preferred_username,
      applications,
    }
  },
  component: AdminApplicationsRoute,
})

function AdminApplicationsRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind={data.kind}>
      <AdminApplicationsPage {...data} />
    </PageMarker>
  )
}

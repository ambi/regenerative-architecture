import { createFileRoute } from '@tanstack/react-router'
import { listAuthorizationDetailTypes } from '../../api/admin'
import { AdminAuthorizationDetailTypesPage } from '../../features/admin-authz-detail-types/AdminAuthorizationDetailTypesPage'
import { requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

export const Route = createFileRoute('/admin/authorization-detail-types')({
  loader: async ({ location }) => {
    const account = await requirePortalAccount('admin', location.pathname, location.searchStr)
    const types = await listAuthorizationDetailTypes()
    return {
      kind: 'admin-authz-detail-types',
      csrfToken: account.csrf_token,
      actorUsername: account.preferred_username,
      types,
    }
  },
  component: AdminAuthorizationDetailTypesRoute,
})

function AdminAuthorizationDetailTypesRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind={data.kind}>
      <AdminAuthorizationDetailTypesPage {...data} />
    </PageMarker>
  )
}

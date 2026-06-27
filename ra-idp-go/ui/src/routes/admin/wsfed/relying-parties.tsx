import { createFileRoute } from '@tanstack/react-router'
import { listWsFedRelyingParties } from '../../../api/admin'
import { AdminWsFedRelyingPartiesPage } from '../../../features/admin-wsfed/AdminWsFedRelyingPartiesPage'
import type { AdminWsFedRelyingPartiesPage as AdminWsFedRelyingPartiesPageData } from '../../../types'
import { requirePortalAccount } from '../../-guards'
import { PageMarker } from '../../-page'

export const Route = createFileRoute('/admin/wsfed/relying-parties')({
  loader: async ({ location }): Promise<AdminWsFedRelyingPartiesPageData> => {
    const account = await requirePortalAccount('admin', location.pathname, location.searchStr)
    const relyingParties = await listWsFedRelyingParties()
    return {
      kind: 'admin-wsfed-relying-parties',
      csrfToken: account.csrf_token,
      actorUsername: account.preferred_username,
      relyingParties,
    }
  },
  component: AdminWsFedRelyingPartiesRoute,
})

function AdminWsFedRelyingPartiesRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind={data.kind}>
      <AdminWsFedRelyingPartiesPage {...data} />
    </PageMarker>
  )
}

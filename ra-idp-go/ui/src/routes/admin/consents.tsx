import { createFileRoute } from '@tanstack/react-router'
import { request } from '../../api/core'
import { AdminConsentsPage } from '../../features/admin-consents/AdminConsentsPage'
import type { AdminConsent } from '../../types'
import { requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

type AdminConsentListResponse = { consents: AdminConsent[] }

export const Route = createFileRoute('/admin/consents')({
  loader: async ({ location }) => {
    const account = await requirePortalAccount('admin', location.pathname, location.searchStr)
    const consents = await request<AdminConsentListResponse>('/api/admin/consents')
    return {
      csrfToken: account.csrf_token,
      actorUsername: account.preferred_username,
      consents: consents.consents,
    }
  },
  component: AdminConsentsRoute,
})

function AdminConsentsRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind="admin-consents">
      <AdminConsentsPage {...data} />
    </PageMarker>
  )
}

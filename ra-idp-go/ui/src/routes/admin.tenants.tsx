import { createFileRoute } from '@tanstack/react-router'
import { request } from '../api/core'
import { AdminTenantsPage } from '../features/admin-tenants/AdminTenantsPage'
import type { AdminTenant, AdminTenantsPage as AdminTenantsPageData } from '../types'
import { requirePortalAccount } from './-guards'
import { PageMarker } from './-page'

type AdminTenantListResponse = { tenants: AdminTenant[] }

export const Route = createFileRoute('/admin/tenants')({
  loader: async ({ location }): Promise<AdminTenantsPageData> => {
    const account = await requirePortalAccount('admin', location.pathname, location.searchStr)
    const tenants = await request<AdminTenantListResponse>('/admin/tenants')
    return {
      kind: 'admin-tenants',
      csrfToken: account.csrf_token,
      actorUsername: account.preferred_username,
      tenants: tenants.tenants,
    }
  },
  component: AdminTenantsRoute,
})

function AdminTenantsRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind={data.kind}>
      <AdminTenantsPage {...data} />
    </PageMarker>
  )
}

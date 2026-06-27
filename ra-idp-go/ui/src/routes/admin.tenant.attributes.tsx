import { createFileRoute } from '@tanstack/react-router'
import { request } from '../api/core'
import { AdminTenantAttributesPage } from '../features/admin-tenants/AdminTenantAttributesPage'
import type {
  AdminTenantAttributesPage as AdminTenantAttributesPageData,
  TenantUserAttributeSchema,
} from '../types'
import { requirePortalAccount } from './-guards'
import { PageMarker } from './-page'

export const Route = createFileRoute('/admin/tenant/attributes')({
  loader: async ({ location }): Promise<AdminTenantAttributesPageData> => {
    const account = await requirePortalAccount('admin', location.pathname, location.searchStr)
    const schema = await request<TenantUserAttributeSchema>(
      '/api/admin/tenant/user_attribute_schema',
    )
    return {
      kind: 'admin-tenant-attributes',
      csrfToken: account.csrf_token,
      actorUsername: account.preferred_username,
      schema,
    }
  },
  component: AdminTenantAttributesRoute,
})

function AdminTenantAttributesRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind={data.kind}>
      <AdminTenantAttributesPage {...data} />
    </PageMarker>
  )
}

import { createFileRoute } from '@tanstack/react-router'
import { getAdminUser } from '../../../api/admin'
import { request } from '../../../api/core'
import { AdminUserDetailPage } from '../../../features/admin-users/AdminUsersPage'
import type { TenantUserAttributeSchema } from '../../../types'
import { requirePortalAccount } from '../../-guards'
import { PageMarker } from '../../-page'

export const Route = createFileRoute('/admin/users_/$sub')({
  loader: async ({ location, params }) => {
    const account = await requirePortalAccount('admin', location.pathname, location.searchStr)
    const [user, schema] = await Promise.all([
      getAdminUser(params.sub),
      request<TenantUserAttributeSchema>('/api/admin/tenant/user_attribute_schema'),
    ])
    return {
      csrfToken: account.csrf_token,
      actorUsername: account.preferred_username,
      user,
      schema,
    }
  },
  component: AdminUserDetailRoute,
})

function AdminUserDetailRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind="admin-user-detail">
      <AdminUserDetailPage {...data} />
    </PageMarker>
  )
}

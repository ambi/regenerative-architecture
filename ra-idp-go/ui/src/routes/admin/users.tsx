import { createFileRoute } from '@tanstack/react-router'
import { request } from '../../api/core'
import { AdminUsersPage } from '../../features/admin-users/AdminUsersPage'
import type { AdminUser, TenantUserAttributeSchema } from '../../types'
import { requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

type AdminUserListResponse = { users: AdminUser[] }

export const Route = createFileRoute('/admin/users')({
  loader: async ({ location }) => {
    const account = await requirePortalAccount('admin', location.pathname, location.searchStr)
    const [users, schema] = await Promise.all([
      request<AdminUserListResponse>('/api/admin/users'),
      request<TenantUserAttributeSchema>('/api/admin/tenant/user_attribute_schema'),
    ])
    return {
      csrfToken: account.csrf_token,
      actorUsername: account.preferred_username,
      users: users.users,
      attributeDefs: [...schema.builtin, ...schema.attributes],
    }
  },
  component: AdminUsersRoute,
})

function AdminUsersRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind="admin-users">
      <AdminUsersPage {...data} />
    </PageMarker>
  )
}

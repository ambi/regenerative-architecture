import { createFileRoute } from '@tanstack/react-router'
import { request } from '../../api/core'
import { AdminRolesPage } from '../../features/admin-roles/AdminRolesPage'
import type { AdminRole, AdminUser } from '../../types'
import { requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

type AdminRoleListResponse = { roles: AdminRole[] }
type AdminUserListResponse = { users: AdminUser[] }

export const Route = createFileRoute('/admin/roles')({
  loader: async ({ location }) => {
    const account = await requirePortalAccount('admin', location.pathname, location.searchStr)
    const [roles, users] = await Promise.all([
      request<AdminRoleListResponse>('/api/admin/policy/roles'),
      request<AdminUserListResponse>('/api/admin/users'),
    ])
    return {
      actorUsername: account.preferred_username,
      roles: roles.roles,
      users: users.users,
    }
  },
  component: AdminRolesRoute,
})

function AdminRolesRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind="admin-roles">
      <AdminRolesPage {...data} />
    </PageMarker>
  )
}

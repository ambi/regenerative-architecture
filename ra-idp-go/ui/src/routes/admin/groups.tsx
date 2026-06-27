import { createFileRoute } from '@tanstack/react-router'
import { request } from '../../api/core'
import { AdminGroupsPage } from '../../features/admin-groups/AdminGroupsPage'
import type { AdminGroup } from '../../types'
import { requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

export const Route = createFileRoute('/admin/groups')({
  loader: async ({ location }) => {
    const account = await requirePortalAccount('admin', location.pathname, location.searchStr)
    const groups = await request<{ groups: AdminGroup[] }>('/api/admin/groups')
    return {
      csrfToken: account.csrf_token,
      actorUsername: account.preferred_username,
      groups: groups.groups,
    }
  },
  component: AdminGroupsRoute,
})

function AdminGroupsRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind="admin-groups">
      <AdminGroupsPage {...data} />
    </PageMarker>
  )
}

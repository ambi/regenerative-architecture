import { createFileRoute } from '@tanstack/react-router'
import { request } from '../../api/core'
import { AdminGroupsPage } from '../../features/admin-groups/AdminGroupsPage'
import type { AdminGroup, AdminGroupsPage as AdminGroupsPageData } from '../../types'
import { requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

export const Route = createFileRoute('/admin/groups')({
  loader: async ({ location }): Promise<AdminGroupsPageData> => {
    const account = await requirePortalAccount('admin', location.pathname, location.searchStr)
    const groups = await request<{ groups: AdminGroup[] }>('/api/admin/groups')
    return {
      kind: 'admin-groups',
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
    <PageMarker kind={data.kind}>
      <AdminGroupsPage {...data} />
    </PageMarker>
  )
}

import { createFileRoute } from '@tanstack/react-router'
import { request } from '../../api/core'
import { AdminAgentsPage } from '../../features/admin-agents/AdminAgentsPage'
import type { AdminAgent } from '../../types'
import { requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

export const Route = createFileRoute('/admin/agents')({
  loader: async ({ location }) => {
    const account = await requirePortalAccount('admin', location.pathname, location.searchStr)
    const agents = await request<{ agents: AdminAgent[] }>('/api/admin/agents')
    return {
      csrfToken: account.csrf_token,
      actorUsername: account.preferred_username,
      agents: agents.agents,
    }
  },
  component: AdminAgentsRoute,
})

function AdminAgentsRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind="admin-agents">
      <AdminAgentsPage {...data} />
    </PageMarker>
  )
}

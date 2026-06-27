import { createFileRoute } from '@tanstack/react-router'
import { getAdminAgent } from '../../../api/admin'
import { AdminAgentDetailPage } from '../../../features/admin-agents/AdminAgentsPage'
import { requirePortalAccount } from '../../-guards'
import { PageMarker } from '../../-page'

export const Route = createFileRoute('/admin/agents_/$agentId')({
  loader: async ({ location, params }) => {
    const account = await requirePortalAccount('admin', location.pathname, location.searchStr)
    const agent = await getAdminAgent(params.agentId)
    return {
      csrfToken: account.csrf_token,
      actorUsername: account.preferred_username,
      agent,
    }
  },
  component: AdminAgentDetailRoute,
})

function AdminAgentDetailRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind="admin-agent-detail">
      <AdminAgentDetailPage {...data} />
    </PageMarker>
  )
}

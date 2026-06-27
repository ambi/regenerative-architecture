import { createFileRoute } from '@tanstack/react-router'
import { request } from '../../api/core'
import { AdminKeysPage } from '../../features/admin-keys/AdminKeysPage'
import type { AdminKey } from '../../types'
import { requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

type AdminKeyListResponse = { keys: AdminKey[] }

export const Route = createFileRoute('/admin/keys')({
  loader: async ({ location }) => {
    const account = await requirePortalAccount('admin', location.pathname, location.searchStr)
    const keys = await request<AdminKeyListResponse>('/api/admin/keys')
    return {
      csrfToken: account.csrf_token,
      actorUsername: account.preferred_username,
      actorRoles: account.roles ?? [],
      actorTenantID: account.tenant_id ?? '',
      keys: keys.keys,
    }
  },
  component: AdminKeysRoute,
})

function AdminKeysRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind="admin-keys">
      <AdminKeysPage {...data} />
    </PageMarker>
  )
}

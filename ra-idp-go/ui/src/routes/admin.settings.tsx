import { createFileRoute } from '@tanstack/react-router'
import { request } from '../api/core'
import { AdminSettingsPage } from '../features/admin-settings/AdminSettingsPage'
import type { AdminSettings, AdminSettingsPage as AdminSettingsPageData } from '../types'
import { requirePortalAccount } from './-guards'
import { PageMarker } from './-page'

export const Route = createFileRoute('/admin/settings')({
  loader: async ({ location }): Promise<AdminSettingsPageData> => {
    const account = await requirePortalAccount('admin', location.pathname, location.searchStr)
    const settings = await request<AdminSettings>('/api/admin/settings')
    return {
      kind: 'admin-settings',
      csrfToken: account.csrf_token,
      actorUsername: account.preferred_username,
      actorRoles: account.roles ?? [],
      actorTenantID: account.tenant_id ?? '',
      settings,
    }
  },
  component: AdminSettingsRoute,
})

function AdminSettingsRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind={data.kind}>
      <AdminSettingsPage {...data} />
    </PageMarker>
  )
}

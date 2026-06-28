import { createFileRoute } from '@tanstack/react-router'
import { listMyApplications } from '../../api/account'
import { AccountAppsPage } from '../../features/account/AccountAppsPage'
import { hasAdminRole, requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

export const Route = createFileRoute('/account/apps')({
  loader: async ({ location }) => {
    const account = await requirePortalAccount('account', location.pathname, location.searchStr)
    const portal = await listMyApplications()
    return {
      username: account.preferred_username ?? 'account',
      applications: portal.applications,
      categories: portal.categories,
      csrfToken: account.csrf_token,
      isAdmin: hasAdminRole(account.roles),
    }
  },
  component: AccountAppsRoute,
})

function AccountAppsRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind="account-apps">
      <AccountAppsPage {...data} />
    </PageMarker>
  )
}

import { createFileRoute } from '@tanstack/react-router'
import { listMyApplications } from '../../api/account'
import { AccountAppsPage } from '../../features/account/AccountAppsPage'
import { hasAdminRole, requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

export const Route = createFileRoute('/account/apps')({
  loader: async ({ location }) => {
    const account = await requirePortalAccount('account', location.pathname, location.searchStr)
    const applications = await listMyApplications()
    return {
      kind: 'account-apps',
      username: account.preferred_username ?? 'account',
      applications,
      isAdmin: hasAdminRole(account.roles),
    }
  },
  component: AccountAppsRoute,
})

function AccountAppsRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind={data.kind}>
      <AccountAppsPage {...data} />
    </PageMarker>
  )
}

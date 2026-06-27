import { createFileRoute } from '@tanstack/react-router'
import { AccountDataPage } from '../../features/account/AccountDataPage'
import { hasAdminRole, requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

export const Route = createFileRoute('/account/data')({
  loader: async ({ location }) => {
    const account = await requirePortalAccount('account', location.pathname, location.searchStr)
    return {
      kind: 'account-data',
      username: account.preferred_username ?? 'account',
      isAdmin: hasAdminRole(account.roles),
    }
  },
  component: AccountDataRoute,
})

function AccountDataRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind={data.kind}>
      <AccountDataPage {...data} />
    </PageMarker>
  )
}

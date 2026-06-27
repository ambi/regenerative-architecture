import { createFileRoute } from '@tanstack/react-router'
import { getAccountSummary } from '../../api/account'
import { AccountHomePage } from '../../features/account/AccountHomePage'
import type { AccountHomePage as AccountHomePageData } from '../../types'
import { hasAdminRole, requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

export const Route = createFileRoute('/account/')({
  loader: async ({ location }): Promise<AccountHomePageData> => {
    const account = await requirePortalAccount('account', location.pathname, location.searchStr)
    const summary = await getAccountSummary()
    return { kind: 'account-home', summary, isAdmin: hasAdminRole(account.roles) }
  },
  component: AccountHomeRoute,
})

function AccountHomeRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind={data.kind}>
      <AccountHomePage {...data} />
    </PageMarker>
  )
}

import { createFileRoute } from '@tanstack/react-router'
import { getSignInActivity, listAccountSessions } from '../../api/account'
import { AccountActivityPage } from '../../features/account/AccountActivityPage'
import { hasAdminRole, requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

export const Route = createFileRoute('/account/activity')({
  loader: async ({ location }) => {
    const account = await requirePortalAccount('account', location.pathname, location.searchStr)
    const [activities, sessions] = await Promise.all([getSignInActivity(), listAccountSessions()])
    return {
      csrfToken: account.csrf_token,
      username: account.preferred_username ?? 'account',
      isAdmin: hasAdminRole(account.roles),
      activities,
      sessions,
    }
  },
  component: AccountActivityRoute,
})

function AccountActivityRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind="account-activity">
      <AccountActivityPage {...data} />
    </PageMarker>
  )
}

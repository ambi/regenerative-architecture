import { createFileRoute } from '@tanstack/react-router'
import { getAccountSecurity } from '../../api/account'
import { AccountSecurityPage } from '../../features/account/AccountSecurityPage'
import { hasAdminRole, requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

export const Route = createFileRoute('/account/security')({
  loader: async ({ location }) => {
    const account = await requirePortalAccount('account', location.pathname, location.searchStr)
    const security = await getAccountSecurity()
    return {
      csrfToken: account.csrf_token,
      username: account.preferred_username ?? 'account',
      isAdmin: hasAdminRole(account.roles),
      security,
    }
  },
  component: AccountSecurityRoute,
})

function AccountSecurityRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind="account-security">
      <AccountSecurityPage {...data} />
    </PageMarker>
  )
}

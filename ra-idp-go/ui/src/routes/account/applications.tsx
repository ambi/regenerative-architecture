import { createFileRoute } from '@tanstack/react-router'
import { listAccountConsents } from '../../api/account'
import { AccountApplicationsPage } from '../../features/account/AccountApplicationsPage'
import { hasAdminRole, requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

export const Route = createFileRoute('/account/applications')({
  loader: async ({ location }) => {
    const account = await requirePortalAccount('account', location.pathname, location.searchStr)
    const consents = await listAccountConsents()
    return {
      csrfToken: account.csrf_token,
      username: account.preferred_username ?? 'account',
      consents,
      isAdmin: hasAdminRole(account.roles),
    }
  },
  component: AccountApplicationsRoute,
})

function AccountApplicationsRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind="account-applications">
      <AccountApplicationsPage {...data} />
    </PageMarker>
  )
}

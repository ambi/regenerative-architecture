import { createFileRoute } from '@tanstack/react-router'
import { getAccountSummary } from '../../api/account'
import { AccountEmailsPage } from '../../features/account/AccountEmailsPage'
import { hasAdminRole, requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

export const Route = createFileRoute('/account/emails')({
  loader: async ({ location }) => {
    const account = await requirePortalAccount('account', location.pathname, location.searchStr)
    const summary = await getAccountSummary()
    return {
      csrfToken: account.csrf_token,
      email: summary.email,
      emailVerified: summary.email_verified,
      isAdmin: hasAdminRole(account.roles),
    }
  },
  component: AccountEmailsRoute,
})

function AccountEmailsRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind="account-emails">
      <AccountEmailsPage {...data} />
    </PageMarker>
  )
}

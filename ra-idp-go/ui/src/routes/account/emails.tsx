import { createFileRoute } from '@tanstack/react-router'
import { getAccountSummary } from '../../api/account'
import { AccountEmailsPage } from '../../features/account/AccountEmailsPage'
import type { AccountEmailsPage as AccountEmailsPageData } from '../../types'
import { hasAdminRole, requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

export const Route = createFileRoute('/account/emails')({
  loader: async ({ location }): Promise<AccountEmailsPageData> => {
    const account = await requirePortalAccount('account', location.pathname, location.searchStr)
    const summary = await getAccountSummary()
    return {
      kind: 'account-emails',
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
    <PageMarker kind={data.kind}>
      <AccountEmailsPage {...data} />
    </PageMarker>
  )
}

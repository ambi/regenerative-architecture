import { createFileRoute } from '@tanstack/react-router'
import { request } from '../../api/core'
import { AccountProfilePage } from '../../features/account/AccountProfilePage'
import type { AccountProfile } from '../../types'
import { hasAdminRole, requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

export const Route = createFileRoute('/account/profile')({
  loader: async ({ location }) => {
    const account = await requirePortalAccount('account', location.pathname, location.searchStr)
    const profile = await request<AccountProfile>('/api/account/profile')
    return {
      csrfToken: account.csrf_token,
      profile,
      isAdmin: hasAdminRole(account.roles),
    }
  },
  component: AccountProfileRoute,
})

function AccountProfileRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind="account-profile">
      <AccountProfilePage {...data} />
    </PageMarker>
  )
}

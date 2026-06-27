import { createFileRoute } from '@tanstack/react-router'
import { ChangePasswordPage } from '../../features/account/ChangePasswordPage'
import { hasAdminRole, requirePortalAccount } from '../-guards'
import { PageMarker } from '../-page'

export const Route = createFileRoute('/account/password')({
  loader: async ({ location }) => {
    const account = await requirePortalAccount('account', location.pathname, location.searchStr)
    return {
      kind: 'change-password',
      csrfToken: account.csrf_token,
      sub: account.sub,
      preferredUsername: account.preferred_username,
      isAdmin: hasAdminRole(account.roles),
    }
  },
  component: ChangePasswordRoute,
})

function ChangePasswordRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind={data.kind}>
      <ChangePasswordPage {...data} />
    </PageMarker>
  )
}

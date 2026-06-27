import { createFileRoute } from '@tanstack/react-router'
import { request } from '../../../api/core'
import { EmailVerifyPage } from '../../../features/auth-flow/EmailVerifyPage'
import { PageMarker } from '../../-page'

export const Route = createFileRoute('/account/email/verify')({
  loader: async ({ location }) => {
    const ctx = await request<{ csrf_token: string }>('/api/account/email/verify_context')
    const token = new URLSearchParams(location.searchStr).get('token') ?? ''
    return { csrfToken: ctx.csrf_token, token }
  },
  component: EmailVerifyRoute,
})

function EmailVerifyRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind="email-verify">
      <EmailVerifyPage {...data} />
    </PageMarker>
  )
}

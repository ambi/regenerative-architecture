import { createFileRoute } from '@tanstack/react-router'
import { request } from '../../../api/core'
import { EmailVerifyPage } from '../../../features/auth-flow/EmailVerifyPage'
import type { EmailVerifyPage as EmailVerifyPageData } from '../../../types'
import { PageMarker } from '../../-page'

export const Route = createFileRoute('/account/email/verify')({
  loader: async ({ location }): Promise<EmailVerifyPageData> => {
    const ctx = await request<{ csrf_token: string }>('/api/account/email/verify_context')
    const token = new URLSearchParams(location.searchStr).get('token') ?? ''
    return { kind: 'email-verify', csrfToken: ctx.csrf_token, token }
  },
  component: EmailVerifyRoute,
})

function EmailVerifyRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind={data.kind}>
      <EmailVerifyPage {...data} />
    </PageMarker>
  )
}

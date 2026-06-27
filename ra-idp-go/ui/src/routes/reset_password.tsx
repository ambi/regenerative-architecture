import { createFileRoute } from '@tanstack/react-router'
import { request } from '../api/core'
import { ResetPasswordPage } from '../features/auth-flow/ResetPasswordPage'
import { PageMarker } from './-page'

type PasswordResetContextResponse = { csrf_token: string }

export const Route = createFileRoute('/reset_password')({
  loader: async ({ location }) => {
    const data = await request<PasswordResetContextResponse>('/api/auth/password_reset_context')
    return {
      csrfToken: data.csrf_token,
      token: new URLSearchParams(location.searchStr).get('token') ?? '',
    }
  },
  component: ResetPasswordRoute,
})

function ResetPasswordRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind="reset-password">
      <ResetPasswordPage {...data} />
    </PageMarker>
  )
}

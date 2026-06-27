import { createFileRoute } from '@tanstack/react-router'
import { request } from '../api/core'
import { ForgotPasswordPage } from '../features/auth-flow/ForgotPasswordPage'
import { PageMarker } from './-page'

type PasswordResetContextResponse = { csrf_token: string }

export const Route = createFileRoute('/forgot_password')({
  loader: async () => {
    const data = await request<PasswordResetContextResponse>('/api/auth/password_reset_context')
    return { csrfToken: data.csrf_token }
  },
  component: ForgotPasswordRoute,
})

function ForgotPasswordRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind="forgot-password">
      <ForgotPasswordPage {...data} />
    </PageMarker>
  )
}

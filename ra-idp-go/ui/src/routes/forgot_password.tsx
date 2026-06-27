import { createFileRoute } from '@tanstack/react-router'
import { request } from '../api/core'
import { ForgotPasswordPage } from '../features/auth-flow/ForgotPasswordPage'
import type { ForgotPasswordPage as ForgotPasswordPageData } from '../types'
import { PageMarker } from './-page'

type PasswordResetContextResponse = { csrf_token: string }

export const Route = createFileRoute('/forgot_password')({
  loader: async (): Promise<ForgotPasswordPageData> => {
    const data = await request<PasswordResetContextResponse>('/api/auth/password_reset_context')
    return { kind: 'forgot-password', csrfToken: data.csrf_token }
  },
  component: ForgotPasswordRoute,
})

function ForgotPasswordRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind={data.kind}>
      <ForgotPasswordPage {...data} />
    </PageMarker>
  )
}

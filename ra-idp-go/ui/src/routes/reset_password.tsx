import { createFileRoute } from '@tanstack/react-router'
import { request } from '../api/core'
import { ResetPasswordPage } from '../features/auth-flow/ResetPasswordPage'
import type { ResetPasswordPage as ResetPasswordPageData } from '../types'
import { PageMarker } from './-page'

type PasswordResetContextResponse = { csrf_token: string }

export const Route = createFileRoute('/reset_password')({
  loader: async ({ location }): Promise<ResetPasswordPageData> => {
    const data = await request<PasswordResetContextResponse>('/api/auth/password_reset_context')
    return {
      kind: 'reset-password',
      csrfToken: data.csrf_token,
      token: new URLSearchParams(location.searchStr).get('token') ?? '',
    }
  },
  component: ResetPasswordRoute,
})

function ResetPasswordRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind={data.kind}>
      <ResetPasswordPage {...data} />
    </PageMarker>
  )
}

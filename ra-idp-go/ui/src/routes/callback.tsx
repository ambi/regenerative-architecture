import { createFileRoute } from '@tanstack/react-router'
import { completeLoginFromCallback } from '../api/oidc'
import { CallbackPage } from '../features/auth-flow/CallbackPage'
import { PageMarker } from './-page'

export const Route = createFileRoute('/callback')({
  loader: async ({ location }) => {
    if (await completeLoginFromCallback()) {
      return new Promise(() => {})
    }
    const parameters = new URLSearchParams(location.searchStr)
    return {
      code: parameters.get('code') ?? undefined,
      error: parameters.get('error') ?? undefined,
      errorDescription: parameters.get('error_description') ?? undefined,
    }
  },
  component: CallbackRoute,
})

function CallbackRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind="callback">
      <CallbackPage {...data} />
    </PageMarker>
  )
}

import { createFileRoute } from '@tanstack/react-router'
import { completeLoginFromCallback } from '../api/oidc'
import { CallbackPage } from '../features/auth-flow/CallbackPage'
import type { CallbackPage as CallbackPageData } from '../types'
import { PageMarker } from './-page'

export const Route = createFileRoute('/callback')({
  loader: async ({ location }): Promise<CallbackPageData> => {
    if (await completeLoginFromCallback()) {
      return new Promise<CallbackPageData>(() => {})
    }
    const parameters = new URLSearchParams(location.searchStr)
    return {
      kind: 'callback',
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
    <PageMarker kind={data.kind}>
      <CallbackPage {...data} />
    </PageMarker>
  )
}

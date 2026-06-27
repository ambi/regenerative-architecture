import { createFileRoute } from '@tanstack/react-router'
import { StatusPage } from '../features/auth-flow/StatusPage'
import { PageMarker } from './-page'

export const Route = createFileRoute('/status')({
  loader: ({ location }) => {
    const state = new URLSearchParams(location.searchStr).get('state')
    const supported = ['approved', 'denied', 'signed-out', 'authentication-required'] as const
    const status = supported.find((value) => value === state) ?? 'authentication-required'
    return { status }
  },
  component: StatusRoute,
})

function StatusRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind="status">
      <StatusPage {...data} />
    </PageMarker>
  )
}

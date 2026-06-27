import { createFileRoute } from '@tanstack/react-router'
import { HomePage } from '../features/auth-flow/HomePage'
import type { HomePage as HomePageData } from '../types'
import { PageMarker } from './-page'

export const Route = createFileRoute('/')({
  loader: (): HomePageData => ({ kind: 'home', demoEnabled: import.meta.env.DEV }),
  component: HomeRoute,
})

function HomeRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind={data.kind}>
      <HomePage {...data} />
    </PageMarker>
  )
}

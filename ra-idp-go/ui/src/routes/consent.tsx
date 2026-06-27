import { createFileRoute } from '@tanstack/react-router'
import { BrowserFlowRoute, loadBrowserFlowData } from './-authFlow'

export const Route = createFileRoute('/consent')({
  loader: ({ location }) => loadBrowserFlowData('/consent', location.searchStr),
  component: ConsentRoute,
})

function ConsentRoute() {
  return <BrowserFlowRoute data={Route.useLoaderData()} />
}

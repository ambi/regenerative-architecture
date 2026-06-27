import { createFileRoute } from '@tanstack/react-router'
import { BrowserFlowRoute, loadBrowserFlowData } from './-authFlow'

export const Route = createFileRoute('/totp')({
  loader: ({ location }) => loadBrowserFlowData('/totp', location.searchStr),
  component: TotpRoute,
})

function TotpRoute() {
  return <BrowserFlowRoute data={Route.useLoaderData()} />
}

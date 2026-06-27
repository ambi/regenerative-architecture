import { createFileRoute } from '@tanstack/react-router'
import { BrowserFlowRoute, loadBrowserFlowData } from './-authFlow'

export const Route = createFileRoute('/login')({
  loader: ({ location }) => loadBrowserFlowData('/login', location.searchStr),
  component: LoginRoute,
})

function LoginRoute() {
  return <BrowserFlowRoute data={Route.useLoaderData()} />
}

import {
  Outlet,
  createBrowserHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import { ConsentPage } from './pages/ConsentPage'
import { CallbackPage } from './pages/CallbackPage'
import { DevicePage } from './pages/DevicePage'
import { HomePage } from './pages/HomePage'
import { LoginPage } from './pages/LoginPage'
import { StatusPage } from './pages/StatusPage'
import { TotpPage } from './pages/TotpPage'
import type { PageData } from './types'

const rootRoute = createRootRoute({
  component: Outlet,
})

export function createAppRouter(data: PageData) {
  const homeRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/',
    component: () => (data.kind === 'home' ? <HomePage {...data} /> : null),
  })
  const loginRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/login',
    component: () => (data.kind === 'login' ? <LoginPage {...data} /> : null),
  })
  const consentRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/consent',
    component: () => (data.kind === 'consent' ? <ConsentPage {...data} /> : null),
  })
  const totpRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/totp',
    component: () => (data.kind === 'totp' ? <TotpPage {...data} /> : null),
  })
  const deviceRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/device',
    component: () => (data.kind === 'device' ? <DevicePage {...data} /> : null),
  })
  const statusRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/status',
    component: () => (data.kind === 'status' ? <StatusPage {...data} /> : null),
  })
  const callbackRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/callback',
    component: () => (data.kind === 'callback' ? <CallbackPage {...data} /> : null),
  })

  return createRouter({
    routeTree: rootRoute.addChildren([
      homeRoute,
      loginRoute,
      totpRoute,
      consentRoute,
      deviceRoute,
      statusRoute,
      callbackRoute,
    ]),
    history: createBrowserHistory(),
  })
}

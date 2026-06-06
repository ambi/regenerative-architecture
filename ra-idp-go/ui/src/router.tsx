import {
  Outlet,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { ConsentPage } from "./pages/ConsentPage";
import { DevicePage } from "./pages/DevicePage";
import { LoginPage } from "./pages/LoginPage";
import { StatusPage } from "./pages/StatusPage";
import type { PageData } from "./types";

const rootRoute = createRootRoute({
  component: Outlet,
});

function pathFor(data: PageData) {
  return `/${data.kind}`;
}

export function createAppRouter(data: PageData) {
  const loginRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/login",
    component: () => data.kind === "login" ? <LoginPage {...data} /> : null,
  });
  const consentRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/consent",
    component: () => data.kind === "consent" ? <ConsentPage {...data} /> : null,
  });
  const deviceRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/device",
    component: () => data.kind === "device" ? <DevicePage {...data} /> : null,
  });
  const statusRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/status",
    component: () => data.kind === "status" ? <StatusPage {...data} /> : null,
  });

  return createRouter({
    routeTree: rootRoute.addChildren([
      loginRoute,
      consentRoute,
      deviceRoute,
      statusRoute,
    ]),
    history: createMemoryHistory({ initialEntries: [pathFor(data)] }),
  });
}

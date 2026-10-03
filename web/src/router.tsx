/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import {
  createRootRoute,
  createRoute,
  createRouter,
  lazyRouteComponent,
  Outlet,
  redirect
} from "@tanstack/react-router";
import { ConfigurationProvider } from "./contexts/ConfigurationContext";
import { AuthProvider } from "./contexts/AuthContext";
import { UIPreferencesProvider } from "./contexts/UIPreferencesContext";
import { ServiceDataProvider } from "./hooks/useServiceData";
import { ProtectedRoute } from "./components/auth/ProtectedRoute";

const rootRoute = createRootRoute({
  // The providers use router hooks, so they render inside the router.
  component: () => (
    <AuthProvider>
      <ConfigurationProvider>
        <UIPreferencesProvider>
          <ServiceDataProvider>
            <Outlet />
          </ServiceDataProvider>
        </UIPreferencesProvider>
      </ConfigurationProvider>
    </AuthProvider>
  ),
});

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/login",
  component: lazyRouteComponent(
    () => import("./components/auth/LoginPage"),
    "LoginPage"
  ),
});

const AppContent = lazyRouteComponent(() => import("./AppContent"));

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: () => (
    <ProtectedRoute>
      <AppContent />
    </ProtectedRoute>
  ),
});

const authLoginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/auth/login",
  beforeLoad: () => {
    throw redirect({ to: "/login" });
  },
});

const catchAllRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "$",
  beforeLoad: () => {
    throw redirect({ to: "/" });
  },
});

export const router = createRouter({
  routeTree: rootRoute.addChildren([
    loginRoute,
    indexRoute,
    authLoginRoute,
    catchAllRoute,
  ]),
  // Shown while a lazy route component loads.
  defaultPendingComponent: () => (
    <div className="min-h-screen bg-color pattern flex items-center justify-center">
      <div className="text-sm text-gray-400">Loading...</div>
    </div>
  ),
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

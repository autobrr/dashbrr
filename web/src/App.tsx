/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { ConfigurationProvider } from "./contexts/ConfigurationContext";
import { AuthProvider } from "./contexts/AuthContext";
import { ServiceDataProvider } from "./hooks/useServiceData";
import { lazy, Suspense } from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { ProtectedRoute } from "./components/auth/ProtectedRoute";
const LoginPage = lazy(() =>
  import("./components/auth/LoginPage").then((m) => ({ default: m.LoginPage }))
);
const AppContent = lazy(() => import("./AppContent"));

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <ConfigurationProvider>
          <ServiceDataProvider>
            <Suspense
              fallback={
                <div className="min-h-screen bg-color pattern flex items-center justify-center">
                  <div className="text-sm text-gray-400">Loading...</div>
                </div>
              }
            >
              <Routes>
                <Route path="/login" element={<LoginPage />} />
                <Route
                  path="/"
                  element={
                    <ProtectedRoute>
                      <AppContent />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/auth/login"
                  element={<Navigate to="/login" replace />}
                />
                <Route path="*" element={<Navigate to="/" replace />} />
              </Routes>
            </Suspense>
          </ServiceDataProvider>
        </ConfigurationProvider>
      </AuthProvider>
    </BrowserRouter>
  );
}

export default App;

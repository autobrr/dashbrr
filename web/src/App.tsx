/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { ServiceHealthMonitor } from "./components/services/ServiceHealthMonitor";
import { ConfigurationProvider } from "./contexts/ConfigurationContext";
import { AuthProvider } from "./contexts/AuthContext";
import { useAuth } from "./hooks/useAuth";
import { Toaster } from "react-hot-toast";
import Toast from "./components/Toast";
import { AddServicesMenu } from "./components/AddServicesMenu";
import { useServiceManagement } from "./hooks/useServiceManagement";
import { TailscaleStatusBar } from "./components/services/TailscaleStatusBar";
import logo from "./assets/logo.svg";
import { serviceTemplates } from "./config/serviceTemplates";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { ProtectedRoute } from "./components/auth/ProtectedRoute";
import { LoginPage } from "./components/auth/LoginPage";
import { CallbackPage } from "./components/auth/CallbackPage";
import { ServiceType } from "./types/service";
import { ArrowRightStartOnRectangleIcon } from "@heroicons/react/20/solid";
import { StatusCounters } from "./components/shared/StatusCounters";
import { useServiceHealth } from "./hooks/useServiceHealth";

// Preload the logo image
const preloadLogo = new Image();
preloadLogo.src = logo;

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <ConfigurationProvider>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route path="/auth/callback" element={<CallbackPage />} />
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
        </ConfigurationProvider>
      </AuthProvider>
    </BrowserRouter>
  );
}

function AppContent() {
  const {
    addServiceInstance,
    showServiceConfig,
    pendingService,
    confirmServiceAddition,
    cancelServiceAddition,
  } = useServiceManagement();
  const { logout } = useAuth();
  const { services } = useServiceHealth();

  const handleTailscaleConfig = () => {
    addServiceInstance("tailscale", "Tailscale");
  };

  return (
    <div
      className="min-h-screen bg-color pattern flex flex-col"
      style={{
        paddingTop: "max(env(safe-area-inset-top), 0.5rem)",
        paddingInline: "max(env(safe-area-inset-right), 0.5rem)",
        paddingBottom: "max(env(safe-area-inset-bottom), 0.5rem)",
      }}
    >
      <div className="p-2 flex-1">
        <header className="mb-4 pt-2 flex justify-between items-center">
          {/* Top header section with logo and controls */}
          <div className="flex items-center">
            <div
              className="flex items-center"
              style={{
                pointerEvents: "none",
                userSelect: "none",
                WebkitUserSelect: "none",
                MozUserSelect: "none",
                msUserSelect: "none",
              }}
              onContextMenu={(e) => e.preventDefault()}
            >
              <img src={logo} alt="Logo" className="h-8 mr-3" />
              <h1 className="text-2xl sm:text-3xl font-bold dark:text-white">
                Dashbrr
              </h1>
            </div>
          </div>
          <div className="flex items-center gap-4">
            <TailscaleStatusBar onConfigOpen={handleTailscaleConfig} />
            <button
              onClick={logout}
              className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-white"
              title="Logout"
            >
              <ArrowRightStartOnRectangleIcon className="h-5 w-5" />
            </button>
          </div>
        </header>

        {/* Subtitle and instruction text */}
        <div className="space-y-2">
          <p className="dark:text-gray-400 text-sm sm:text-base pb-4">
            Service Health Monitor - and then some
          </p>
        </div>

        <main>
          <div className="flex justify-between items-center w-full">
            <div className="flex-grow">
              {services && (
                <span className="p-2 bg-gray-800 rounded-md inline-block select-none pointer-events-none">
                  <StatusCounters services={services} />
                </span>
              )}
            </div>
            <div className="ml-4">
              <AddServicesMenu
                serviceTemplates={serviceTemplates}
                onAddService={(type: ServiceType, name: string) =>
                  addServiceInstance(type, name)
                }
                showServiceConfig={showServiceConfig}
                pendingService={pendingService}
                onConfirmService={confirmServiceAddition}
                onCancelService={cancelServiceAddition}
              />
            </div>
          </div>
          <ServiceHealthMonitor />
        </main>
        <Toaster position="top-right">
          {(t) => (
            <Toast
              type={
                t.type === "success"
                  ? "success"
                  : t.type === "error"
                  ? "error"
                  : "info"
              }
              body={t.message as string}
              t={t}
            />
          )}
        </Toaster>
      </div>
    </div>
  );
}

export default App;

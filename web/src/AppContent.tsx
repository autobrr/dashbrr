/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { Toaster } from "react-hot-toast";
import { ArrowRightStartOnRectangleIcon } from "@heroicons/react/20/solid";

import logo from "./assets/logo.svg";
import { serviceTemplates } from "./config/serviceTemplates";
import { useAuth } from "./hooks/useAuth";
import { useServiceManagement } from "./hooks/useServiceManagement";
import { useServiceHealth } from "./hooks/useServiceHealth";
import { ServiceType } from "./types/service";

import Toast from "./components/Toast";
import { AddServicesMenu } from "./components/AddServicesMenu";
import { ServiceHealthMonitor } from "./components/services/ServiceHealthMonitor";
import { StatusCounters } from "./components/shared/StatusCounters";
import { TailscaleStatusBar } from "./components/services/TailscaleStatusBar";
import { LanguageSelector } from "./components/shared/LanguageSelector";
import { useTranslation } from "react-i18next";

// Preload the logo image.
const preloadLogo = new Image();
preloadLogo.src = logo;

export default function AppContent() {
  const { t } = useTranslation();
  const {
    addServiceInstance,
    showServiceConfig,
    pendingService,
    confirmServiceAddition,
    cancelServiceAddition,
  } = useServiceManagement();
  const { logout } = useAuth();
  const { services } = useServiceHealth();

  return (
    <div
      className="min-h-screen bg-color pattern flex flex-col"
      style={{
        paddingTop: "max(env(safe-area-inset-top), 0.5rem)",
        paddingInline: "max(env(safe-area-inset-right), 0.5rem)",
        paddingBottom: "max(env(safe-area-inset-bottom), 0.5rem)",
      }}
    >
      <div className="mx-auto w-full max-w-[1800px] p-2 sm:p-3 flex-1">
        <header className="mb-4 pt-2">
          <div className="relative">
            <div className="flex flex-col sm:flex-row justify-between items-center gap-2 sm:gap-0">
              <div
                className="flex flex-col sm:flex-row items-center"
                style={{
                  pointerEvents: "none",
                  userSelect: "none",
                  WebkitUserSelect: "none",
                  MozUserSelect: "none",
                  msUserSelect: "none",
                }}
                onContextMenu={(e) => e.preventDefault()}
              >
                <img
                  src={logo}
                  alt="Logo"
                  className="h-8 sm:mr-3 mb-2 sm:mb-0"
                />
                <span className="flex flex-col sm:flex-row items-center gap-1 sm:gap-2">
                  <h1 className="text-2xl sm:text-3xl font-bold dark:text-white leading-none">
                    {t("header.title")}
                  </h1>
                  <span className="flex items-center">
                    <p className="dark:text-gray-400 text-xs tracking-wide lowercase mt-1 sm:mt-1">
                      {t("header.subtitle")}
                    </p>
                  </span>
                </span>
              </div>
              <div className="hidden sm:flex items-center gap-4">
                <TailscaleStatusBar />
                <LanguageSelector />
                <button
                  onClick={logout}
                  className="p-1 text-zinc-400 hover:text-zinc-600 dark:hover:text-white"
                  title={t("auth.logout")}
                >
                  <ArrowRightStartOnRectangleIcon className="h-5 w-5" />
                </button>
              </div>
            </div>
            <div className="sm:hidden absolute top-0 right-0 flex items-center gap-2 mt-2 mr-2">
              <LanguageSelector />
              <button
                onClick={logout}
                className="p-1 text-zinc-400 hover:text-zinc-600 dark:hover:text-white"
                title={t("auth.logout")}
              >
                <ArrowRightStartOnRectangleIcon className="h-5 w-5" />
              </button>
            </div>
            <div className="sm:hidden flex justify-center w-full mt-2">
              <TailscaleStatusBar />
            </div>
          </div>
        </header>

        <main>
          <div className="flex w-full flex-wrap items-center justify-between gap-3">
            <div className="flex-grow">
              {services && (
                <span className="inline-block select-none rounded-md bg-zinc-800 p-2 pointer-events-none">
                  <StatusCounters services={services} />
                </span>
              )}
            </div>
            <div className="w-full sm:ml-4 sm:w-auto">
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

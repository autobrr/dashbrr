import React, { useState } from "react";
import { createRoot } from "react-dom/client";

import { AddServicesMenu } from "../../src/components/AddServicesMenu";
import { ConfigurationForm } from "../../src/components/configuration/ConfigurationForm";
import { serviceTemplates } from "../../src/config/serviceTemplates";
import { ConfigurationContext } from "../../src/contexts/context";
import { useServiceManagement } from "../../src/hooks/useServiceManagement";
import type { ServiceConfig, ServiceConfigInput } from "../../src/types/service";

declare global {
  interface Window {
    saves: Array<{ instanceId: string; config: ServiceConfigInput }>;
  }
}

window.saves = [];

const stored: Record<string, ServiceConfig> = {
  "sonarr-2": { displayName: "Sonarr 2", url: "http://sonarr.test:8989", edits: "all" },
  "tailscale-1": { displayName: "Tailscale", url: "https://api.tailscale.com", edits: "all" },
  "netbird-1": { displayName: "NetBird", url: "https://api.netbird.io", edits: "all" },
  "general-1": { displayName: "Status page", url: "http://status.test/health", edits: "all" },
};

export function AddService() {
  const {
    addServiceInstance,
    showServiceConfig,
    pendingService,
    confirmServiceAddition,
    cancelServiceAddition,
  } = useServiceManagement();

  return (
    <AddServicesMenu
      serviceTemplates={serviceTemplates}
      onAddService={addServiceInstance}
      showServiceConfig={showServiceConfig}
      pendingService={pendingService}
      onConfirmService={confirmServiceAddition}
      onCancelService={cancelServiceAddition}
    />
  );
}

export function Harness() {
  const [configurations, setConfigurations] = useState(stored);
  const params = new URLSearchParams(window.location.search);
  const edit = params.get("edit");

  const value = {
    configurations,
    updateConfiguration: async (instanceId: string, config: ServiceConfigInput) => {
      window.saves.push({ instanceId, config });
      const saved = { ...config, apiKey: undefined, edits: "all" as const };
      setConfigurations((prev) => ({ ...prev, [instanceId]: saved }));
      return saved;
    },
    deleteConfiguration: async () => undefined,
    fetchConfigurations: async () => undefined,
    isLoading: false,
    error: null,
  };

  return (
    <ConfigurationContext.Provider value={value}>
      {edit ? (
        <ConfigurationForm instanceId={edit} onClose={() => undefined} />
      ) : (
        <AddService />
      )}
    </ConfigurationContext.Provider>
  );
}

const app = document.getElementById("app");

if (!app) {
  throw new Error("missing app mount");
}

createRoot(app).render(
  <React.StrictMode>
    <Harness />
  </React.StrictMode>
);

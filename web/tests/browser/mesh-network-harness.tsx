import React from "react";
import { createRoot } from "react-dom/client";

import { MeshNetworkStatusBar } from "../../src/components/services/MeshNetworkStatusBar";
import { ConfigurationContext } from "../../src/contexts/context";
import { ServiceDataContextProvider } from "../../src/hooks/useServiceData";
import type { NetBirdPeer, Service, ServiceConfig, TailscaleDevice } from "../../src/types/service";

const tailscaleDevices: TailscaleDevice[] = [
  {
    id: "ts-1",
    name: "laptop",
    ipAddress: "100.64.0.7",
    lastSeen: "2026-11-14T10:00:00Z",
    online: true,
    deviceType: "macOS",
    clientVersion: "1.80.0",
    updateAvailable: false,
    tags: ["tag:work"],
  },
  {
    id: "ts-2",
    name: "nas",
    ipAddress: "100.64.0.8",
    lastSeen: "2026-11-13T20:00:00Z",
    online: false,
    deviceType: "linux",
    clientVersion: "1.78.0",
    updateAvailable: true,
    tags: [],
  },
];

const netbirdPeers: NetBirdPeer[] = [
  {
    id: "nb-1",
    name: "host-1",
    ip: "100.111.0.10",
    online: true,
    lastSeen: "2026-11-14T10:05:26Z",
    os: "Linux",
    version: "0.60.0",
    updateAvailable: true,
    groups: ["homelab"],
    cityName: "Kiel",
    countryCode: "DE",
    loginExpired: false,
  },
  {
    id: "nb-2",
    name: "host-2",
    ip: "100.111.0.11",
    online: false,
    lastSeen: "2026-11-13T21:00:00Z",
    os: "Windows",
    version: "0.61.0",
    updateAvailable: false,
    groups: ["office"],
    cityName: "Berlin",
    countryCode: "DE",
    loginExpired: true,
  },
];

const tailscaleService: Service = {
  id: "tailscale-1",
  instanceId: "tailscale-1",
  name: "Tailscale",
  displayName: "Tailscale",
  type: "tailscale",
  status: "online",
  url: "https://api.tailscale.com",
  message: "1 devices online",
  stats: { tailscale: { devices: tailscaleDevices } },
  details: { tailscale: { total: 2, online: 1 } },
};

const netbirdService: Service = {
  id: "netbird-1",
  instanceId: "netbird-1",
  name: "NetBird",
  displayName: "NetBird",
  type: "netbird",
  status: "online",
  url: "https://api.netbird.io",
  message: "1/2 peers online",
  stats: { netbird: { peers: netbirdPeers } },
  details: { netbird: { total: 2, online: 1, updateAvailable: true } },
};

const script = new URLSearchParams(window.location.search).get("scenario") ?? "both";

// In netbird-warning, the token sees no peers. In discovered, Kubernetes discovery
// owns the NetBird service.
const netbirdScenarios = ["netbird-only", "netbird-warning", "discovered"];
const configurations: Record<string, ServiceConfig> = {};
const services: Service[] = [];

if (!netbirdScenarios.includes(script)) {
  configurations["tailscale-1"] = {
    instanceId: "tailscale-1",
    displayName: "Tailscale",
    type: "tailscale",
    status: "online",
    url: "https://api.tailscale.com",
    edits: "all",
  } as unknown as ServiceConfig;
  services.push(tailscaleService);
}
if (script !== "tailscale-only") {
  configurations["netbird-1"] = {
    instanceId: "netbird-1",
    displayName: "NetBird",
    type: "netbird",
    status: "online",
    url: "https://api.netbird.io",
    edits: script === "discovered" ? "none" : "all",
  } as unknown as ServiceConfig;
  services.push(
    script === "netbird-warning"
      ? {
        ...netbirdService,
        status: "warning",
        message: "Token sees no peers: use a token from an admin user",
        stats: { netbird: { peers: [] } },
        details: { netbird: { total: 0, online: 0 } },
      }
      : netbirdService
  );
}

const configurationValue = {
  configurations,
  updateConfiguration: async () => configurations[Object.keys(configurations)[0]],
  deleteConfiguration: async () => undefined,
  fetchConfigurations: async () => undefined,
  isLoading: false,
  error: null,
};

const app = document.getElementById("app");

if (!app) {
  throw new Error("missing app mount");
}

const scenario = script.replace("-", "_");

createRoot(app).render(
  <React.StrictMode>
    <ConfigurationContext.Provider value={configurationValue}>
      <ServiceDataContextProvider services={services}>
        <header
          data-testid={`header-${scenario}`}
          className="flex items-center gap-4 rounded-lg border border-zinc-700 bg-zinc-800 px-4 py-3"
        >
          <MeshNetworkStatusBar />
        </header>
      </ServiceDataContextProvider>
    </ConfigurationContext.Provider>
  </React.StrictMode>
);

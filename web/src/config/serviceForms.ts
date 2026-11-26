/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { ServiceType } from "../types/service";
import { serviceLink, ServiceLinkTarget } from "../utils/serviceLink";

export interface FormHelp {
  prefix: string;
  text: string;
  link: string | null;
}

export interface ServiceForm {
  urlPlaceholder: string;
  apiKeyLabel: string;
  apiKeyHelp: (service: ServiceLinkTarget) => FormHelp;
  credential?: "optional" | "plexSignIn";
  defaultUrl?: string;
  hideUrl?: true;
  testBeforeSave?: false;
}

const noHelp = (): FormHelp => ({ prefix: "", text: "", link: null });

const foundIn =
  (text: string, path: string) =>
    (service: ServiceLinkTarget): FormHelp => ({
      prefix: "Found in ",
      text,
      link: serviceLink(service, path),
    });

const defaultUrl = "Enter service URL";

export const serviceForms: Record<ServiceType, ServiceForm> = {
  autobrr: {
    urlPlaceholder: defaultUrl,
    apiKeyLabel: "API Key",
    apiKeyHelp: foundIn("Settings > API", "/settings/api"),
  },
  radarr: {
    urlPlaceholder: defaultUrl,
    apiKeyLabel: "API Key",
    apiKeyHelp: foundIn("Settings > General", "/settings/general"),
  },
  sonarr: {
    urlPlaceholder: defaultUrl,
    apiKeyLabel: "API Key",
    apiKeyHelp: foundIn("Settings > General", "/settings/general"),
  },
  whisparr: {
    urlPlaceholder: "http://localhost:6969",
    apiKeyLabel: "API Key",
    apiKeyHelp: foundIn("Settings > General", "/settings/general"),
  },
  lidarr: {
    urlPlaceholder: defaultUrl,
    apiKeyLabel: "API Key",
    apiKeyHelp: foundIn("Settings > General", "/settings/general"),
  },
  readarr: {
    urlPlaceholder: defaultUrl,
    apiKeyLabel: "API Key",
    apiKeyHelp: foundIn("Settings > General", "/settings/general"),
  },
  bazarr: {
    urlPlaceholder: "http://localhost:6767",
    apiKeyLabel: "API Key",
    apiKeyHelp: foundIn("Settings > General", "/settings/general"),
  },
  sabnzbd: {
    urlPlaceholder: "http://localhost:8080",
    apiKeyLabel: "API Key",
    apiKeyHelp: foundIn("Config > General", "/config/general/"),
  },
  nzbget: {
    urlPlaceholder: "http://localhost:6789",
    apiKeyLabel: "Control Password",
    apiKeyHelp: foundIn(
      "Settings > Security (ControlPassword, or username:password)",
      "/#settings"
    ),
  },
  prowlarr: {
    urlPlaceholder: defaultUrl,
    apiKeyLabel: "API Key",
    apiKeyHelp: foundIn("Settings > General", "/settings/general"),
  },
  traefik: {
    urlPlaceholder: "http://localhost:8080",
    apiKeyLabel: "Auth Token (Optional)",
    credential: "optional",
    apiKeyHelp: () => ({
      prefix: "Optional - ",
      text: "Bearer token or user:password for protected dashboard APIs",
      link: null,
    }),
  },
  overseerr: {
    urlPlaceholder: defaultUrl,
    apiKeyLabel: "API Key",
    apiKeyHelp: foundIn("Settings", "/settings/main"),
  },
  plex: {
    urlPlaceholder: "http://localhost:32400",
    apiKeyLabel: "X-Plex-Token",
    credential: "plexSignIn",
    apiKeyHelp: () => ({
      prefix: "Authenticate via ",
      text: "Plex PIN flow guide",
      link: "https://forums.plex.tv/t/authenticating-with-plex/609370",
    }),
  },
  jellyfin: {
    urlPlaceholder: "http://localhost:8096",
    apiKeyLabel: "API Key",
    apiKeyHelp: foundIn("Dashboard > API Keys", "/web/index.html#!/apikeys.html"),
  },
  emby: {
    urlPlaceholder: "http://localhost:8096",
    apiKeyLabel: "API Key",
    apiKeyHelp: foundIn("Settings > Advanced > API Keys", "/web/index.html#!/apikeys"),
  },
  uptimekuma: {
    urlPlaceholder: "http://localhost:3001",
    apiKeyLabel: "API Key",
    apiKeyHelp: foundIn("Settings > API Keys", "/settings/api-keys"),
  },
  tailscale: {
    urlPlaceholder: "https://api.tailscale.com",
    apiKeyLabel: "API Token",
    defaultUrl: "https://api.tailscale.com",
    hideUrl: true,
    apiKeyHelp: ({ url }) => ({
      prefix: "Found in ",
      text: "Admin Console > Settings > Keys",
      link:
        url === "https://api.tailscale.com"
          ? "https://login.tailscale.com/admin/settings/keys"
          : null,
    }),
  },
  netbird: {
    urlPlaceholder: "https://api.netbird.io",
    apiKeyLabel: "API Token (Personal Access Token)",
    defaultUrl: "https://api.netbird.io",
    apiKeyHelp: ({ url }) => ({
      prefix: "Create one in ",
      text: "the NetBird dashboard, or Team > Users > Personal Access Tokens",
      link:
        url === "https://api.netbird.io"
          ? "https://app.netbird.io/team/user-settings?tab=tokens"
          : null,
    }),
  },
  maintainerr: {
    urlPlaceholder: defaultUrl,
    apiKeyLabel: "API Key",
    apiKeyHelp: foundIn("Settings", "/settings/main"),
  },
  qui: {
    urlPlaceholder: "http://localhost:7476",
    apiKeyLabel: "API Key",
    apiKeyHelp: foundIn("Settings > API Key", "/settings"),
  },
  general: {
    urlPlaceholder: "Enter full URL including health endpoint",
    apiKeyLabel: "API Key",
    credential: "optional",
    testBeforeSave: false,
    apiKeyHelp: () => ({
      prefix: "Optional - ",
      text: "api token for authentication if required",
      link: null,
    }),
  },
  other: {
    urlPlaceholder: defaultUrl,
    apiKeyLabel: "API Key",
    testBeforeSave: false,
    apiKeyHelp: noHelp,
  },
};

export const serviceFormFor = (type: string): ServiceForm =>
  Object.prototype.hasOwnProperty.call(serviceForms, type)
    ? serviceForms[type as ServiceType]
    : serviceForms.other;

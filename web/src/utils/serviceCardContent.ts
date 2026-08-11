/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import type { Service, ServiceStatus } from "../types/service";

const ACTIONABLE_STATUSES = new Set<ServiceStatus>([
  "warning",
  "error",
  "offline",
]);

const hasText = (value?: string) => Boolean(value?.trim());

const hasActionableMessage = (service: Service): boolean =>
  ACTIONABLE_STATUSES.has(service.status) &&
  (hasText(service.message) || hasText(service.health?.message));

export const SERVICE_CARD_LAYOUT = {
  compact: {
    bodyMarginClass: "mt-1",
    footerSpacingClass: "mt-2 pt-2",
  },
  regular: {
    bodyMarginClass: "mt-2",
    footerSpacingClass: "mt-4 pt-4",
  },
} as const;

export type ServiceCardLayoutMode = keyof typeof SERVICE_CARD_LAYOUT;

export const getServiceCardLayoutClasses = (
  mode: ServiceCardLayoutMode
) => SERVICE_CARD_LAYOUT[mode];

export const hasMeaningfulServiceContent = (service: Service): boolean => {
  if (hasActionableMessage(service)) {
    return true;
  }

  switch (service.type) {
    case "autobrr":
      // Autobrr always renders persistent stat tiles.
      return true;
    case "plex":
      return (
        (service.details?.plex?.activeStreams ?? 0) > 0 ||
        (service.stats?.plex?.sessions?.length ?? 0) > 0
      );
    case "jellyfin":
      return (
        (service.details?.jellyfin?.activeStreams ?? 0) > 0 ||
        (service.stats?.jellyfin?.summary?.sessions?.length ?? 0) > 0
      );
    case "uptimekuma":
      // Uptime Kuma cards render persistent monitor stat tiles.
      return true;
    case "sonarr":
      return (service.stats?.sonarr?.queue?.totalRecords ?? 0) > 0;
    case "radarr":
      return (service.stats?.radarr?.queue?.totalRecords ?? 0) > 0;
    case "lidarr":
      return (service.stats?.lidarr?.queue?.totalRecords ?? 0) > 0;
    case "readarr":
      return (service.stats?.readarr?.queue?.totalRecords ?? 0) > 0;
    case "bazarr":
      // Bazarr cards always render summary tiles.
      return true;
    case "sabnzbd":
      // SABnzbd cards render queue/failure summary tiles.
      return true;
    case "nzbget":
      // NZBGet cards render queue/failure summary tiles.
      return true;
    case "prowlarr":
      return (service.stats?.prowlarr?.indexers?.length ?? 0) > 0;
    case "traefik":
      return (
        (service.details?.traefik?.routerTotal ?? 0) > 0 ||
        (service.stats?.traefik?.summary?.issueRouters?.length ?? 0) > 0
      );
    case "seerr":
      return (service.stats?.seerr?.requests?.length ?? 0) > 0;
    case "maintainerr":
      return (service.stats?.maintainerr?.collections?.length ?? 0) > 0;
    case "qui":
      return (
        Boolean(service.details?.qui?.summary) ||
        (service.stats?.qui?.transfers?.length ?? 0) > 0
      );
    case "tailscale":
      return (service.stats?.tailscale?.devices?.length ?? 0) > 0;
    case "general":
    case "other":
    default:
      return false;
  }
};

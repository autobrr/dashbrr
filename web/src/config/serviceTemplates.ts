/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { Service } from '../types/service';

export const serviceTemplates: Omit<Service, "id" | "instanceId">[] = [
  {
    name: "Autobrr",
    displayName: "",
    type: "autobrr",
    status: "offline",
    url: "",
    accessUrl: "",
    healthEndpoint: "/api/health/autobrr",
  },
  {
    name: "Radarr",
    displayName: "",
    type: "radarr",
    status: "offline",
    url: "",
    accessUrl: "",
    healthEndpoint: "/api/health/radarr",
  },
  {
    name: "Sonarr",
    displayName: "",
    type: "sonarr",
    status: "offline",
    url: "",
    accessUrl: "",
    healthEndpoint: "/api/health/sonarr",
  },
  {
    name: "Lidarr",
    displayName: "",
    type: "lidarr",
    status: "offline",
    url: "",
    accessUrl: "",
    healthEndpoint: "/api/health/lidarr",
  },
  {
    name: "Readarr",
    displayName: "",
    type: "readarr",
    status: "offline",
    url: "",
    accessUrl: "",
    healthEndpoint: "/api/health/readarr",
  },
  {
    name: "Prowlarr",
    displayName: "",
    type: "prowlarr",
    status: "offline",
    url: "",
    accessUrl: "",
    healthEndpoint: "/api/health/prowlarr",
  },
  {
    name: "Overseerr",
    displayName: "",
    type: "overseerr",
    status: "offline",
    url: "",
    accessUrl: "",
    healthEndpoint: "/api/health/overseerr",
  },
  {
    name: "Plex",
    displayName: "",
    type: "plex",
    status: "offline",
    url: "",
    accessUrl: "",
    healthEndpoint: "/api/health/plex",
  },
  {
    name: "Tailscale",
    displayName: "",
    type: "tailscale",
    status: "offline",
    url: "",
    healthEndpoint: "/api/health/tailscale",
  },
  {
    name: "Maintainerr",
    displayName: "",
    type: "maintainerr",
    status: "offline",
    url: "",
    accessUrl: "",
    healthEndpoint: "/api/health/maintainerr",
  },
  {
    name: "Qui",
    displayName: "",
    type: "qui",
    status: "offline",
    url: "",
    accessUrl: "",
    healthEndpoint: "/api/health/qui",
  },
  {
    name: "General Service",
    displayName: "",
    type: "general",
    status: "offline",
    url: "",
    accessUrl: "",
    healthEndpoint: "",
    apiKey: undefined,
  },
];

export default serviceTemplates;

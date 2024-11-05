import { Service } from '../types/service';

export const serviceTemplates: Omit<Service, "id" | "instanceId">[] = [
  {
    name: "Autobrr",
    displayName: "Autobrr",
    type: "autobrr",
    status: "offline",
    url: "",
    healthEndpoint: "/api/health/autobrr",
  },
  {
    name: "Omegabrr",
    displayName: "Omegabrr",
    type: "omegabrr",
    status: "offline",
    url: "",
    healthEndpoint: "/api/health/omegabrr",
  },
  {
    name: "Radarr",
    displayName: "Radarr",
    type: "radarr",
    status: "offline",
    url: "",
    healthEndpoint: "/api/health/radarr",
  },
  {
    name: "Sonarr",
    displayName: "Sonarr",
    type: "sonarr",
    status: "offline",
    url: "",
    healthEndpoint: "/api/health/sonarr",
  },
  {
    name: "Prowlarr",
    displayName: "Prowlarr",
    type: "prowlarr",
    status: "offline",
    url: "",
    healthEndpoint: "/api/health/prowlarr",
  },
  {
    name: "Overseerr",
    displayName: "Overseerr",
    type: "overseerr",
    status: "offline",
    url: "",
    healthEndpoint: "/api/health/overseerr",
  },
  {
    name: "Plex",
    displayName: "Plex",
    type: "plex",
    status: "offline",
    url: "",
    healthEndpoint: "/api/health/plex",
  },
  {
    name: "Tailscale",
    displayName: "Tailscale",
    type: "tailscale",
    status: "offline",
    url: "",
    healthEndpoint: "/api/health/tailscale",
  },
  {
    name: "Maintainerr",
    displayName: "Maintainerr",
    type: "maintainerr",
    status: "offline",
    url: "",
    healthEndpoint: "/api/health/maintainerr",
  },
];

export default serviceTemplates;

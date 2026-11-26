/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { ServiceType } from "../types/service";

export const releasePages: Record<ServiceType, string | null> = {
  autobrr: "https://github.com/autobrr/autobrr/releases/",
  radarr: "https://github.com/Radarr/Radarr/releases",
  sonarr: "https://github.com/Sonarr/Sonarr/releases",
  whisparr: "https://github.com/Whisparr/Whisparr/releases",
  lidarr: "https://github.com/Lidarr/Lidarr/releases",
  readarr: "https://github.com/Readarr/Readarr/releases",
  bazarr: "https://github.com/morpheus65535/bazarr/releases",
  sabnzbd: "https://github.com/sabnzbd/sabnzbd/releases",
  nzbget: "https://github.com/nzbgetcom/nzbget/releases",
  prowlarr: "https://github.com/Prowlarr/Prowlarr/releases",
  traefik: "https://github.com/traefik/traefik/releases",
  overseerr: "https://github.com/sct/overseerr/releases/",
  plex: "https://www.plex.tv/media-server-downloads/",
  jellyfin: "https://github.com/jellyfin/jellyfin/releases",
  emby: "https://github.com/MediaBrowser/Emby.Releases/releases",
  uptimekuma: "https://github.com/louislam/uptime-kuma/releases",
  tailscale: "https://tailscale.com/changelog",
  maintainerr: "https://github.com/jorenn92/Maintainerr/releases/",
  qui: "https://github.com/autobrr/qui/releases/",
  general: null,
  other: null,
};

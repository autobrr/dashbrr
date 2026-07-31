# Supported Services Matrix

This matrix shows the support for each service: CLI commands, discovery, credentials, and poller detail jobs.

Notes:

- All configured services also receive baseline health polling.
- The poll intervals in the table apply to the service-specific detail jobs in `internal/api/handlers/poller.go`.
- The CLI command group for the generic service is `generic`. The discovery key is `general`.

| Service | CLI Group | Discovery Key | Credential | Required | Detail Endpoint(s) | Poll Interval |
| --- | --- | --- | --- | --- | --- | --- |
| Autobrr | `autobrr` | `autobrr` | API key | Yes | `/api/autobrr/stats`, `/api/autobrr/irc`, `/api/autobrr/releases` | 120s |
| Bazarr | `bazarr` | `bazarr` | API key | Yes | `/api/bazarr/summary` | 90s |
| General | `generic` | `general` | API key/token | Optional | none (the health poll also shows top-level JSON fields on the card) | n/a |
| Jellyfin | `jellyfin` | `jellyfin` | API key | Yes | `/api/jellyfin/summary` | 10s |
| Lidarr | `lidarr` | `lidarr` | API key | Yes | `/api/lidarr/queue` | 60s |
| Maintainerr | `maintainerr` | `maintainerr` | API key | Yes | `/api/maintainerr/collections` | 10m |
| NZBGet | `nzbget` | `nzbget` | Control password or `user:pass` | Yes | `/api/nzbget/summary` | 45s |
| Overseerr | `overseerr` | `overseerr` | API key | Yes | `/api/overseerr/requests` | 60s |
| Plex | `plex` | `plex` | Plex token | Yes | `/api/plex/sessions` | 10s |
| Prowlarr | `prowlarr` | `prowlarr` | API key | Yes | `/api/prowlarr/stats`, `/api/prowlarr/indexers` | 120s |
| Qui | `qui` | `qui` | API key | Yes | `/api/qui/overview` | 20s |
| Radarr | `radarr` | `radarr` | API key | Yes | `/api/radarr/queue` | 60s |
| Readarr | `readarr` | `readarr` | API key | Yes | `/api/readarr/queue` | 60s |
| SABnzbd | `sabnzbd` | `sabnzbd` | API key | Yes | `/api/sabnzbd/summary` | 45s |
| Sonarr | `sonarr` | `sonarr` | API key | Yes | `/api/sonarr/queue` | 60s |
| Tailscale | `tailscale` | `tailscale` | API token | Yes | `/api/tailscale/devices` | 60s |
| Traefik | `traefik` | `traefik` | Auth token or `user:pass` | Optional | `/api/traefik/summary` | 30s |
| Uptime Kuma | `uptimekuma` | `uptimekuma` | API key or `user:pass` | Yes | `/api/uptimekuma/summary` | 30s |

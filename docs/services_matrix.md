# Supported Services Matrix

This matrix shows the support for each service: CLI commands, discovery, credentials, and poller detail jobs.

Notes:

- All configured services also receive baseline health polling.
- The poll intervals in the table apply to the service-specific detail jobs in `internal/api/handlers/poller.go`.
- The CLI command group for the generic service is `generic`. The discovery key is `general`.

| Service | CLI Group | Discovery Key | Credential | Required | Detail Job(s) | Poll Interval |
| --- | --- | --- | --- | --- | --- | --- |
| Autobrr | `autobrr` | `autobrr` | API key | Yes | `autobrr_stats`, `autobrr_irc_status`, `autobrr_releases` | 120s |
| Bazarr | `bazarr` | `bazarr` | API key | Yes | `bazarr_summary` | 90s |
| General | `generic` | `general` | API key/token | Optional | none (the health poll also shows top-level JSON fields on the card) | n/a |
| Jellyfin | `jellyfin` | `jellyfin` | API key | Yes | `jellyfin_summary` | 10s |
| Lidarr | `lidarr` | `lidarr` | API key | Yes | `lidarr_queue` | 60s |
| Maintainerr | `maintainerr` | `maintainerr` | API key | Yes | `maintainerr_collections` | 10m |
| NZBGet | `nzbget` | `nzbget` | Control password or `user:pass` | Yes | `nzbget_summary` | 45s |
| Overseerr | `overseerr` | `overseerr` | API key | Yes | `overseerr_requests` | 60s |
| Plex | `plex` | `plex` | Plex token | Yes | `plex_sessions` | 10s |
| Prowlarr | `prowlarr` | `prowlarr` | API key | Yes | `prowlarr_stats`, `prowlarr_indexers` | 120s |
| Qui | `qui` | `qui` | API key | Yes | `qui_overview` | 20s |
| Radarr | `radarr` | `radarr` | API key | Yes | `radarr_queue` | 60s |
| Readarr | `readarr` | `readarr` | API key | Yes | `readarr_queue` | 60s |
| SABnzbd | `sabnzbd` | `sabnzbd` | API key | Yes | `sabnzbd_summary` | 45s |
| Sonarr | `sonarr` | `sonarr` | API key | Yes | `sonarr_queue` | 60s |
| Tailscale | `tailscale` | `tailscale` | API token | Yes | `tailscale_devices` | 60s |
| Traefik | `traefik` | `traefik` | Auth token or `user:pass` | Optional | `traefik_summary` | 30s |
| Uptime Kuma | `uptimekuma` | `uptimekuma` | API key or `user:pass` | Yes | `uptimekuma_summary` | 30s |
| Whisparr | `whisparr` | `whisparr` | API key | Yes | `whisparr_queue` | 60s |

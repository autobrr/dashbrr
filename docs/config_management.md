# Service Discovery and Configuration Management

## Overview

Dashbrr supports automatic service discovery and configuration management through:

- Docker container labels
- Kubernetes service annotations
- External configuration files (YAML/JSON)

Reference files:

- Service matrix: [`docs/services_matrix.md`](services_matrix.md)
- Kubernetes manifest bundle: [`docs/k8s_discovery_example.yaml`](k8s_discovery_example.yaml)

## Command Usage

### Service Discovery

```bash
# Discover services from Docker containers
dashbrr config discover --docker

# Discover services from Kubernetes
dashbrr config discover --k8s

# Discover from Kubernetes and auto-confirm service import
dashbrr config discover --k8s --yes

# Discover from both Docker and Kubernetes
dashbrr config discover
```

### Configuration Import/Export

```bash
# Import services from configuration file
dashbrr config import services.yaml

# Export current configuration
dashbrr config export --format=yaml --mask-secrets --output=services.yaml
```

## Docker Label Configuration

Configure services using Docker container labels:

```yaml
labels:
  com.dashbrr.service.type: "radarr" # Required: Service type
  com.dashbrr.service.url: "http://radarr:7878" # Required: Service URL
  com.dashbrr.service.apikey: "${RADARR_API_KEY}" # Usually required: API key/token (supports env vars)
  com.dashbrr.service.name: "My Radarr" # Optional: Custom display name
  com.dashbrr.service.enabled: "true" # Optional: Enable/disable service
```

Example docker-compose.yml:

```yaml
version: "3"
services:
  radarr:
    image: linuxserver/radarr
    labels:
      com.dashbrr.service.type: "radarr"
      com.dashbrr.service.url: "http://radarr:7878"
      com.dashbrr.service.apikey: "${RADARR_API_KEY}"
      com.dashbrr.service.name: "Movies"
```

## Kubernetes Annotation Configuration

Configure services using Kubernetes service annotations:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: radarr
  annotations:
    com.dashbrr.service.type: "radarr"
    com.dashbrr.service.url: "http://radarr.media.svc:7878"
    com.dashbrr.service.apikey: "${RADARR_API_KEY}" # Optional for general/traefik
    com.dashbrr.service.name: "Movies"
    com.dashbrr.service.enabled: "true"
spec:
  ports:
    - port: 7878
  selector:
    app: radarr
```

Notes:

- Dashbrr uses annotations for Kubernetes discovery because URLs and API-key placeholders are not valid Kubernetes label values.
- When Dashbrr runs inside Kubernetes, discovery uses in-cluster credentials automatically.
- Dashbrr discovers Services across all namespaces, so its ServiceAccount needs list/read access to Services cluster-wide.
- Traefik certificate expiry insights use the `traefik_tls_certs_not_after` Prometheus metric from `/metrics`.
  Make sure Traefik metrics are reachable from Dashbrr (same URL or a reachable `:9100` metrics port on the same host).

Minimal RBAC for in-cluster discovery:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: dashbrr-discovery
rules:
  - apiGroups: [""]
    resources: ["services"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: dashbrr-discovery
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: dashbrr-discovery
subjects:
  - kind: ServiceAccount
    name: dashbrr
    namespace: your-namespace
```

## Configuration File Format

Services can be configured using YAML or JSON files:

```yaml
services:
  radarr:
    - url: "http://radarr:7878"
      apikey: "${RADARR_API_KEY}"
      name: "Movies" # Optional
  sonarr:
    - url: "http://sonarr:8989"
      apikey: "${SONARR_API_KEY}"
      name: "TV Shows"
  prowlarr:
    - url: "http://prowlarr:9696"
      apikey: "${PROWLARR_API_KEY}"
```

## Environment Variables

When using environment variables for API keys/tokens (`${SERVICE_API_KEY}`), the following naming convention is used:

- `DASHBRR_AUTOBRR_API_KEY`
- `DASHBRR_BAZARR_API_KEY`
- `DASHBRR_GENERAL_API_KEY` (optional)
- `DASHBRR_JELLYFIN_API_KEY`
- `DASHBRR_LIDARR_API_KEY`
- `DASHBRR_MAINTAINERR_API_KEY`
- `DASHBRR_NZBGET_API_KEY`
- `DASHBRR_OVERSEERR_API_KEY`
- `DASHBRR_PLEX_API_KEY`
- `DASHBRR_PROWLARR_API_KEY`
- `DASHBRR_QUI_API_KEY`
- `DASHBRR_RADARR_API_KEY`
- `DASHBRR_READARR_API_KEY`
- `DASHBRR_SABNZBD_API_KEY`
- `DASHBRR_SONARR_API_KEY`
- `DASHBRR_TAILSCALE_API_KEY`
- `DASHBRR_TRAEFIK_API_KEY` (optional)
- `DASHBRR_UPTIMEKUMA_API_KEY`

## Supported Discovery Service Types

Discovery/import currently supports these service type keys:

- `autobrr`
- `bazarr`
- `general`
- `jellyfin`
- `lidarr`
- `maintainerr`
- `nzbget`
- `overseerr`
- `plex`
- `prowlarr`
- `qui`
- `radarr`
- `readarr`
- `sabnzbd`
- `sonarr`
- `tailscale`
- `traefik`
- `uptimekuma`

## Security Considerations

- API keys can be provided via environment variables for enhanced security
- Use `--mask-secrets` when exporting configurations to avoid exposing API keys
- Exported configurations with masked secrets will use environment variable references
- Ensure proper access controls for configuration files containing sensitive information

## Best Practices

1. Service Discovery:

   - Use consistent naming conventions for services
   - Group related services in the same namespace/network
   - Use environment variables for API keys

2. Configuration Management:

   - Keep a backup of your configuration
   - Use version control for configuration files
   - Document any custom service configurations

3. Kubernetes:
   - Start from [`docs/k8s_discovery_example.yaml`](k8s_discovery_example.yaml) for RBAC + annotation shape.
   - Keep discovery credentials in environment variables on the Dashbrr workload, not inline in annotations.

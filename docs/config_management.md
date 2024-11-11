# Service Discovery and Configuration Management

## Overview

Dashbrr supports automatic service discovery and configuration management through:

- Docker container labels
- Kubernetes service labels
- External configuration files (YAML/JSON)

## Command Usage

### Service Discovery

```bash
# Discover services from Docker containers
dashbrr run config discover --docker

# Discover services from Kubernetes
dashbrr run config discover --k8s

# Discover from both Docker and Kubernetes
dashbrr run config discover
```

### Configuration Import/Export

```bash
# Import services from configuration file
dashbrr run config import services.yaml

# Export current configuration
dashbrr run config export --format=yaml --mask-secrets --output=services.yaml
```

## Docker Label Configuration

Configure services using Docker container labels:

```yaml
labels:
  com.dashbrr.service.type: "radarr" # Required: Service type
  com.dashbrr.service.url: "http://radarr:7878" # Required: Service URL
  com.dashbrr.service.apikey: "${RADARR_API_KEY}" # Required: API key (supports env vars)
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

## Kubernetes Label Configuration

Configure services using Kubernetes service labels:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: radarr
  labels:
    com.dashbrr.service.type: "radarr"
    com.dashbrr.service.url: "http://radarr.media.svc:7878"
    com.dashbrr.service.apikey: "${RADARR_API_KEY}"
    com.dashbrr.service.name: "Movies"
    com.dashbrr.service.enabled: "true"
spec:
  ports:
    - port: 7878
  selector:
    app: radarr
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

When using environment variables for API keys (${SERVICE_API_KEY}), the following naming convention is used:

- `DASHBRR_RADARR_API_KEY`
- `DASHBRR_SONARR_API_KEY`
- `DASHBRR_PROWLARR_API_KEY`
- `DASHBRR_OVERSEERR_API_KEY`
- `DASHBRR_MAINTAINERR_API_KEY`
- `DASHBRR_TAILSCALE_API_KEY`
- `DASHBRR_PLEX_API_KEY`
- `DASHBRR_AUTOBRR_API_KEY`
- `DASHBRR_OMEGABRR_API_KEY`

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

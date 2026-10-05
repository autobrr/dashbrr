# Service Discovery and Configuration Management

## Overview

Dashbrr supports automatic service discovery and configuration management through:

- Docker container labels
- Kubernetes service annotations
- External configuration files (YAML/JSON)

Reference files:

- Service matrix: [`docs/services_matrix.md`](services_matrix.md)
- Kubernetes manifest bundle: [`docs/k8s_discovery_example.yaml`](k8s_discovery_example.yaml)

## Base path

By default, dashbrr is served at the root of the host. To serve it below a path, for example `https://example.com/dashbrr/`, set `base_path`:

```toml
[server]
base_path = "/dashbrr"
```

You can also set `DASHBRR__BASE_PATH=/dashbrr`. Dashbrr reads the value at start. To change it, restart dashbrr.

With a base path:

- All pages and API routes are below the base path. `/` and `/dashbrr` send a `307` redirect to `/dashbrr/`.
- `/health` answers at the root and below the base path.
- The OIDC `redirect_url` must include the base path, for example `https://example.com/dashbrr/api/auth/oidc/callback`. Use the same URL in your OIDC provider.

The reverse proxy must send the full path to dashbrr. Do not remove the prefix. An nginx example:

```nginx
location /dashbrr/ {
    # No path after the port, so nginx keeps /dashbrr/ in the request.
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
    # Live updates use server-sent events. Do not buffer them.
    proxy_buffering off;
}
```

## Command Usage

### Service Discovery

```bash
# Discover services from Docker containers
dashbrr config discover --docker

# Sync services from Kubernetes (the same sync that serve runs)
dashbrr config discover --k8s

# Sync from Kubernetes and apply the changes without a question
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

Configure services with annotations on Kubernetes Services:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: radarr
  namespace: media
  annotations:
    com.dashbrr.service.type: "radarr"
    com.dashbrr.service.url: "http://radarr.media.svc:7878" # Optional: see "Inferred URL" below
    com.dashbrr.service.apikey: "${DASHBRR_RADARR_API_KEY}" # Optional for general/traefik. Ignored for Plex: use the Plex sign-in in the UI
    com.dashbrr.service.name: "Movies"
    com.dashbrr.service.access_url: "https://radarr.example.com" # Optional: see "Access URL from an HTTPRoute" below
    com.dashbrr.service.enabled: "true"
spec:
  ports:
    - port: 7878
  selector:
    app: radarr
```

### Inferred URL

The `url` annotation is optional. When a Service has no `url` annotation, discovery builds the URL from the Service: `<scheme>://<service>.<namespace>.svc[:<port>]`.

Discovery selects the port in this order:

1. The port named `https`, with the scheme `https`.
2. The port named `http`, with the scheme `http`.
3. The first port, with the scheme `http`.

The URL does not include the port when the port is the default port for the scheme (80 for `http`, 443 for `https`). The URL does not include a cluster domain, because the DNS search list of the pod resolves `<service>.<namespace>.svc`.

For the Service above, without the `url` annotation, the URL is `http://radarr.media.svc:7878`. The port has no name, so discovery uses the first port.

When you set the `url` annotation, discovery always uses it.

The inferred URL resolves only inside the cluster. If dashbrr runs outside the cluster with a kubeconfig, set the `url` annotation. If the TLS certificate of the Service does not include `<service>.<namespace>.svc`, set the `url` annotation.

### Access URL from an HTTPRoute

The `access_url` annotation is optional. When a Service has no `access_url` annotation, discovery looks for a Gateway API `HTTPRoute` in the same namespace that has a `backendRef` to the Service. The access URL is `https://<first hostname>` of that route. Discovery skips a wildcard hostname such as `*.example.com`, because a browser cannot open it. Discovery also skips a route that the Gateway rejected. A rejected route has a parent status, and no parent has the condition `Accepted=True`. If more than one route matches, discovery uses the route whose name is first in alphabetical order.

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: radarr
  namespace: media
spec:
  hostnames: ["radarr.example.com"] # The access URL is https://radarr.example.com
  rules:
    - backendRefs:
        - name: radarr
          port: 7878
```

When you set the `access_url` annotation, discovery always uses it.

If the cluster does not have the HTTPRoute CRD, or RBAC does not give access to `httproutes`, discovery does not use routes. It logs a debug message and the sync continues.

Notes:

- Dashbrr uses annotations for Kubernetes discovery because URLs and API-key placeholders are not valid Kubernetes label values.
- When Dashbrr runs inside Kubernetes, discovery uses in-cluster credentials automatically.
- Traefik certificate expiry insights use the `traefik_tls_certs_not_after` Prometheus metric from `/metrics`.
  Make sure Traefik metrics are reachable from Dashbrr (same URL or a reachable `:9100` metrics port on the same host).

### Sync in serve

When you enable the sync, `serve` reads the annotations at startup and then at each interval. The default interval is 5 minutes.

```toml
[discovery.kubernetes]
enabled = true
namespaces = ["media"] # Use ["*"] for all namespaces.
interval_minutes = 5
```

The same settings are available as environment variables. Refer to [`docs/env_vars.md`](env_vars.md).

The annotations own each discovered service. A discovered service is a service that has an instance ID in the form `<type>-k8s-<namespace>.<service>`. Each sync does these steps:

- It adds a discovered service for each Service that has a `type` annotation.
- It updates a discovered service when an annotation changes. When you remove an annotation, the field becomes empty or gets its default value. The sync ignores an `apikey` annotation on a Plex Service and keeps the token from the Plex sign-in.
- It deletes a discovered service when its Service is gone, when its `type` annotation is gone, when its `enabled` annotation is `false`, or when dashbrr no longer scans its namespace.
- It does not change a service that you added in the UI or imported from a file.
- If a list from the Kubernetes API fails, it logs the error and changes nothing.
- If the annotations of a Service are not valid, for example when an env var for `apikey` is not set, it logs a warning and deletes the discovered service. The next sync with valid annotations adds it again.

A discovered service is read-only. The UI shows the Kubernetes icon on its card and has no edit or delete controls for it. The API refuses to edit or delete it. To change a discovered service, change its annotations.

A discovered Plex service is the one exception. Plex gets its token from the Plex sign-in in the UI, so its card keeps the gear. The gear opens a form that has only the Plex sign-in. A save changes only the token.

`dashbrr config discover --k8s` runs the same sync one time. It shows the changes and asks before it applies them.

### Namespaces

- A list of namespaces: dashbrr scans only these namespaces.
- `*`: dashbrr scans all namespaces. This needs a `ClusterRole`.
- Empty (the default): dashbrr scans only the namespace of its pod. It reads the namespace from the service account file `/var/run/secrets/kubernetes.io/serviceaccount/namespace`. Outside a cluster, an empty setting is an error.

For the default, a `Role` in the namespace of dashbrr is enough:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: dashbrr-discovery
  namespace: media
rules:
  - apiGroups: [""]
    resources: ["services"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["gateway.networking.k8s.io"]
    resources: ["httproutes"]
    verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: dashbrr-discovery
  namespace: media
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: dashbrr-discovery
subjects:
  - kind: ServiceAccount
    name: dashbrr
    namespace: media
```

For a list of namespaces, add the same `Role` and `RoleBinding` in each namespace. For `*`, use a `ClusterRole` and a `ClusterRoleBinding` with the same rules. Refer to [`docs/k8s_discovery_example.yaml`](k8s_discovery_example.yaml).

### API keys

Do not put an API key in an annotation. Put a `${ENV_VAR}` reference in the `apikey` annotation, and set the env var on the dashbrr container. Dashbrr reads the value from its own environment.

For example, an ExternalSecret can create a Secret, and the dashbrr Deployment can read the env var from it:

```yaml
# On the Service
metadata:
  annotations:
    com.dashbrr.service.apikey: "${DASHBRR_RADARR_API_KEY}"
---
# On the dashbrr container
env:
  - name: DASHBRR_RADARR_API_KEY
    valueFrom:
      secretKeyRef:
        name: dashbrr-api-keys # For example, a Secret that an ExternalSecret creates
        key: radarr
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

## Ignored *arr Health Checks

Some *arr health checks report a known state that you want to keep. For example, `RemovedSeriesCheck` reports series that are on disk but no longer in TheTVDB. To stop such a check from putting the service in the warning state, add its name to `config.toml`:

```toml
[arr]
ignored_health_checks = ["RemovedSeriesCheck", "RemovedMovieCheck"]
```

The list applies to all *arr services (Radarr, Sonarr, Lidarr, Readarr, Whisparr, and Prowlarr). The name is the `source` field of the *arr health API. Letter case and outer spaces do not matter. An ignored check does not show in the health message. If dashbrr ignores all checks, the service shows `Healthy`.

You can also set the list with the `DASHBRR__ARR_IGNORED_HEALTH_CHECKS` environment variable, as a comma-separated list. See [`docs/env_vars.md`](env_vars.md).

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
- `DASHBRR_WHISPARR_API_KEY`

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
- `whisparr`

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

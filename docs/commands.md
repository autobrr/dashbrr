# Command Line Interface Documentation

This document outlines all available CLI commands in Dashbrr.

## Startup Flags

When starting Dashbrr, you can use the following flags to control its configuration:

```bash
# Start Dashbrr with default settings
dashbrr serve

# Specify a custom config file location
dashbrr serve --config=/path/to/config.toml

# Specify a custom database location
dashbrr serve --db-file=/path/to/database.db

# Specify a custom listen address
dashbrr serve --listen-addr=:8081
```

By default:

- The config file is loaded from `./config.toml`
- The database file is created in the same directory as the config file at `<config_dir>/data/dashbrr.db`
- The server listens on port 8080

For example:

```bash
# Using config in /etc/dashbrr
dashbrr serve --config=/etc/dashbrr/config.toml
# Database will be created at /etc/dashbrr/data/dashbrr.db

# Override default database location
dashbrr serve --config=/etc/dashbrr/config.toml --db-file=/var/lib/dashbrr/dashbrr.db
```

## Core Commands

### User Management

```bash
# Create a new user
dashbrr user create <username> <password> [email]
Example: dashbrr user create admin password123
Example: dashbrr user create admin password123 admin@example.com

# Change user password
dashbrr user change-password <username> <new_password>
Example: dashbrr user change-password admin newpassword123
```

### Health Checks

```bash
# Check system and service health
dashbrr health [--services] [--system] [--json]

Options:
  --services  Check health of configured services
  --system    Check system health (database and config)
  --json      Output results in JSON format

Example: dashbrr health --services --system
Example: dashbrr health --json
```

The health command provides information about:

- System health:
  - Database connection status and type
  - Configuration file validity
- Service health:
  - Status of all configured services
  - Individual service health checks

### Version Information

```bash
# Display version information
dashbrr version [--check-github] [--json]

Options:
  --check-github  Check for latest version on GitHub
  --json         Output results in JSON format

Example: dashbrr version --check-github
Example: dashbrr version --json
```

The version command shows:

- Current version
- Build commit hash
- Build date
- Latest release information (when using --check-github)

## Service Management Commands

Each service type supports the following operations:

- `add`: Add a new service configuration
- `remove`: Remove an existing service configuration
- `list`: List all configured services of that type

Currently supported service command groups:

- `autobrr`
- `bazarr`
- `generic` (general health endpoint service)
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

### Autobrr

```bash
# Add an Autobrr service
dashbrr service autobrr add <url> <api-key>
Example: dashbrr service autobrr add http://localhost:7474 your-api-key

# Remove an Autobrr service
dashbrr service autobrr remove <url>
Example: dashbrr service autobrr remove http://localhost:7474

# List Autobrr services
dashbrr service autobrr list
```

### Bazarr

```bash
# Add a Bazarr service
dashbrr service bazarr add <url> <api-key>
Example: dashbrr service bazarr add http://localhost:6767 your-api-key

# Remove a Bazarr service
dashbrr service bazarr remove <url>
Example: dashbrr service bazarr remove http://localhost:6767

# List Bazarr services
dashbrr service bazarr list
```

### Generic (General) Services

```bash
# Add a Generic service
dashbrr service generic add <url> <name> [api-key]
Example: dashbrr service generic add http://my.general.service/healthz/liveness MyService
Example: dashbrr service generic add http://my.general.service/healthz/liveness MyService optional-api-key

# Remove a Generic service
dashbrr service generic remove <url>
Example: dashbrr service generic remove http://my.general.service/healthz/liveness

# List Generic services
dashbrr service generic list
```

### Jellyfin

```bash
# Add a Jellyfin service
dashbrr service jellyfin add <url> <api-key>
Example: dashbrr service jellyfin add http://localhost:8096 your-api-key

# Remove a Jellyfin service
dashbrr service jellyfin remove <url>
Example: dashbrr service jellyfin remove http://localhost:8096

# List Jellyfin services
dashbrr service jellyfin list
```

### Lidarr

```bash
# Add a Lidarr service
dashbrr service lidarr add <url> <api-key>
Example: dashbrr service lidarr add http://localhost:8686 your-api-key

# Remove a Lidarr service
dashbrr service lidarr remove <url>
Example: dashbrr service lidarr remove http://localhost:8686

# List Lidarr services
dashbrr service lidarr list
```

### Maintainerr

```bash
# Add a Maintainerr service
dashbrr service maintainerr add <url> <api-key>
Example: dashbrr service maintainerr add http://localhost:6246 your-api-key

# Remove a Maintainerr service
dashbrr service maintainerr remove <url>
Example: dashbrr service maintainerr remove http://localhost:7476

# List Maintainerr services
dashbrr service maintainerr list
```

### Overseerr

```bash
# Add an Overseerr service
dashbrr service overseerr add <url> <api-key>
Example: dashbrr service overseerr add http://localhost:5055 your-api-key

# Remove an Overseerr service
dashbrr service overseerr remove <url>
Example: dashbrr service overseerr remove http://localhost:5055

# List Overseerr services
dashbrr service overseerr list
```

### Plex

```bash
# Add a Plex service
dashbrr service plex add <url> <token>
Example: dashbrr service plex add http://localhost:32400 your-plex-token

# Remove a Plex service
dashbrr service plex remove <url>
Example: dashbrr service plex remove http://localhost:32400

# List Plex services
dashbrr service plex list
```

### Prowlarr

```bash
# Add a Prowlarr service
dashbrr service prowlarr add <url> <api-key>
Example: dashbrr service prowlarr add http://localhost:9696 your-api-key

# Remove a Prowlarr service
dashbrr service prowlarr remove <url>
Example: dashbrr service prowlarr remove http://localhost:9696

# List Prowlarr services
dashbrr service prowlarr list
```

### Qui

```bash
# Add a Qui service
dashbrr service qui add <url> <api-key>
Example: dashbrr service qui add http://localhost:7476 your-api-key

# Remove a Qui service
dashbrr service qui remove <url>
Example: dashbrr service qui remove http://localhost:7476

# List Qui services
dashbrr service qui list
```

### Radarr

```bash
# Add a Radarr service
dashbrr service radarr add <url> <api-key>
Example: dashbrr service radarr add http://localhost:7878 your-api-key

# Remove a Radarr service
dashbrr service radarr remove <url>
Example: dashbrr service radarr remove http://localhost:7878

# List Radarr services
dashbrr service radarr list
```

### Readarr

```bash
# Add a Readarr service
dashbrr service readarr add <url> <api-key>
Example: dashbrr service readarr add http://localhost:8787 your-api-key

# Remove a Readarr service
dashbrr service readarr remove <url>
Example: dashbrr service readarr remove http://localhost:8787

# List Readarr services
dashbrr service readarr list
```

### SABnzbd

```bash
# Add a SABnzbd service
dashbrr service sabnzbd add <url> <api-key>
Example: dashbrr service sabnzbd add http://localhost:8080 your-api-key

# Remove a SABnzbd service
dashbrr service sabnzbd remove <url>
Example: dashbrr service sabnzbd remove http://localhost:8080

# List SABnzbd services
dashbrr service sabnzbd list
```

### Sonarr

```bash
# Add a Sonarr service
dashbrr service sonarr add <url> <api-key>
Example: dashbrr service sonarr add http://localhost:8989 your-api-key

# Remove a Sonarr service
dashbrr service sonarr remove <url>
Example: dashbrr service sonarr remove http://localhost:8989

# List Sonarr services
dashbrr service sonarr list
```

### NZBGet

```bash
# Add an NZBGet service
dashbrr service nzbget add <url> <control-password-or-user:pass>
Example: dashbrr service nzbget add http://localhost:6789 your-control-password

# Remove an NZBGet service
dashbrr service nzbget remove <url>
Example: dashbrr service nzbget remove http://localhost:6789

# List NZBGet services
dashbrr service nzbget list
```

### Tailscale

```bash
# Add a Tailscale service
dashbrr service tailscale add <api-key>
Example: dashbrr service tailscale add tskey-api-xxxxxxxx

# Remove a Tailscale service
dashbrr service tailscale remove <url>
Example: dashbrr service tailscale remove https://api.tailscale.com

# List Tailscale services
dashbrr service tailscale list
```

### Traefik

```bash
# Add a Traefik service (auth token optional)
dashbrr service traefik add <url> [auth-token]
Example: dashbrr service traefik add http://localhost:8080
Example: dashbrr service traefik add http://localhost:8080 your-auth-token

# Remove a Traefik service
dashbrr service traefik remove <url>
Example: dashbrr service traefik remove http://localhost:8080

# List Traefik services
dashbrr service traefik list
```

### Uptime Kuma

```bash
# Add an Uptime Kuma service
dashbrr service uptimekuma add <url> <api-key-or-user:pass>
Example: dashbrr service uptimekuma add http://localhost:3001 your-api-key

# Remove an Uptime Kuma service
dashbrr service uptimekuma remove <url>
Example: dashbrr service uptimekuma remove http://localhost:3001

# List Uptime Kuma services
dashbrr service uptimekuma list
```

## Common Parameters

- `<url>`: The base URL of the service (must include http:// or https://)
- `<api-key>`: API key for authentication with the service
- `[name]`: Optional display name for the service (defaults to service type)
- `[api-key]`: Optional API key for services that don't require authentication
- `tailscale add`: does not accept a URL; it always targets `https://api.tailscale.com`
- `generic`: command name is `generic` (service type shown in UI is `general`)

## Notes

- All services require a valid HTTP or HTTPS URL
- The system performs a health check when adding services to verify connectivity
- Each service is assigned a unique instance ID automatically
- You can run multiple instances of the same service type with different URLs
- Service health and version information is displayed when listing services (if available)
- User passwords must be at least 8 characters long
- Usernames must be between 3 and 32 characters

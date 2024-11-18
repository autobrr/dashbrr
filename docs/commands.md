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

### General Services

```bash
# Add a General service
dashbrr service general add <url> [name] [api-key]
Example: dashbrr service general add http://my.general.service/healthz/liveness MyService
Example: dashbrr service general add http://my.general.service/healthz/liveness MyService optional-api-key

# Remove a General service
dashbrr service general remove <url>
Example: dashbrr service general remove http://localhost:7475

# List General services
dashbrr service general list
```

### Maintainerr

```bash
# Add a Maintainerr service
dashbrr service maintainerr add <url> <api-key>
Example: dashbrr service maintainerr add http://localhost:7476 your-api-key

# Remove a Maintainerr service
dashbrr service maintainerr remove <url>
Example: dashbrr service maintainerr remove http://localhost:7476

# List Maintainerr services
dashbrr service maintainerr list
```

### Omegabrr

```bash
# Add an Omegabrr service
dashbrr service omegabrr add <url> <api-key>
Example: dashbrr service omegabrr add http://localhost:7477 your-api-key

# Remove an Omegabrr service
dashbrr service omegabrr remove <url>
Example: dashbrr service omegabrr remove http://localhost:7477

# List Omegabrr services
dashbrr service omegabrr list
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

### Tailscale

```bash
# Add a Tailscale service
dashbrr service tailscale add <url> <api-key>
Example: dashbrr service tailscale add http://localhost:8088 your-api-key

# Remove a Tailscale service
dashbrr service tailscale remove <url>
Example: dashbrr service tailscale remove http://localhost:8088

# List Tailscale services
dashbrr service tailscale list
```

## Common Parameters

- `<url>`: The base URL of the service (must include http:// or https://)
- `<api-key>`: API key for authentication with the service
- `[name]`: Optional display name for the service (defaults to service type)
- `[api-key]`: Optional API key for services that don't require authentication

## Notes

- All services require a valid HTTP or HTTPS URL
- The system performs a health check when adding services to verify connectivity
- Each service is assigned a unique instance ID automatically
- You can run multiple instances of the same service type with different URLs
- Service health and version information is displayed when listing services (if available)
- User passwords must be at least 8 characters long
- Usernames must be between 3 and 32 characters

# Command Line Interface Documentation

This document outlines all available CLI commands in Dashbrr.

## Core Commands

### User Management

```bash
# Create a new user
dashbrr run user create <username> <password> [email]
Example: dashbrr run user create admin password123
Example: dashbrr run user create admin password123 admin@example.com

# Change user password
dashbrr run user change-password <username> <new_password>
Example: dashbrr run user change-password admin newpassword123
```

### Health Checks

```bash
# Check system and service health
dashbrr run health [--services] [--system] [--json]

Options:
  --services  Check health of configured services
  --system    Check system health (database and config)
  --json      Output results in JSON format

Example: dashbrr run health --services --system
Example: dashbrr run health --json
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
dashbrr run version [--check-github] [--json]

Options:
  --check-github  Check for latest version on GitHub
  --json         Output results in JSON format

Example: dashbrr run version --check-github
Example: dashbrr run version --json
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
dashbrr run service autobrr add <url> <api-key>
Example: dashbrr run service autobrr add http://localhost:7474 your-api-key

# Remove an Autobrr service
dashbrr run service autobrr remove <url>
Example: dashbrr run service autobrr remove http://localhost:7474

# List Autobrr services
dashbrr run service autobrr list
```

### General Services

```bash
# Add a General service
dashbrr run service general add <url> [name] [api-key]
Example: dashbrr run service general add http://my.general.service/healthz/liveness MyService
Example: dashbrr run service general add http://my.general.service/healthz/liveness MyService optional-api-key

# Remove a General service
dashbrr run service general remove <url>
Example: dashbrr run service general remove http://localhost:7475

# List General services
dashbrr run service general list
```

### Maintainerr

```bash
# Add a Maintainerr service
dashbrr run service maintainerr add <url> <api-key>
Example: dashbrr run service maintainerr add http://localhost:7476 your-api-key

# Remove a Maintainerr service
dashbrr run service maintainerr remove <url>
Example: dashbrr run service maintainerr remove http://localhost:7476

# List Maintainerr services
dashbrr run service maintainerr list
```

### Omegabrr

```bash
# Add an Omegabrr service
dashbrr run service omegabrr add <url> <api-key>
Example: dashbrr run service omegabrr add http://localhost:7477 your-api-key

# Remove an Omegabrr service
dashbrr run service omegabrr remove <url>
Example: dashbrr run service omegabrr remove http://localhost:7477

# List Omegabrr services
dashbrr run service omegabrr list
```

### Overseerr

```bash
# Add an Overseerr service
dashbrr run service overseerr add <url> <api-key>
Example: dashbrr run service overseerr add http://localhost:5055 your-api-key

# Remove an Overseerr service
dashbrr run service overseerr remove <url>
Example: dashbrr run service overseerr remove http://localhost:5055

# List Overseerr services
dashbrr run service overseerr list
```

### Plex

```bash
# Add a Plex service
dashbrr run service plex add <url> <token>
Example: dashbrr run service plex add http://localhost:32400 your-plex-token

# Remove a Plex service
dashbrr run service plex remove <url>
Example: dashbrr run service plex remove http://localhost:32400

# List Plex services
dashbrr run service plex list
```

### Prowlarr

```bash
# Add a Prowlarr service
dashbrr run service prowlarr add <url> <api-key>
Example: dashbrr run service prowlarr add http://localhost:9696 your-api-key

# Remove a Prowlarr service
dashbrr run service prowlarr remove <url>
Example: dashbrr run service prowlarr remove http://localhost:9696

# List Prowlarr services
dashbrr run service prowlarr list
```

### Radarr

```bash
# Add a Radarr service
dashbrr run service radarr add <url> <api-key>
Example: dashbrr run service radarr add http://localhost:7878 your-api-key

# Remove a Radarr service
dashbrr run service radarr remove <url>
Example: dashbrr run service radarr remove http://localhost:7878

# List Radarr services
dashbrr run service radarr list
```

### Sonarr

```bash
# Add a Sonarr service
dashbrr run service sonarr add <url> <api-key>
Example: dashbrr run service sonarr add http://localhost:8989 your-api-key

# Remove a Sonarr service
dashbrr run service sonarr remove <url>
Example: dashbrr run service sonarr remove http://localhost:8989

# List Sonarr services
dashbrr run service sonarr list
```

### Tailscale

```bash
# Add a Tailscale service
dashbrr run service tailscale add <url> <api-key>
Example: dashbrr run service tailscale add http://localhost:8088 your-api-key

# Remove a Tailscale service
dashbrr run service tailscale remove <url>
Example: dashbrr run service tailscale remove http://localhost:8088

# List Tailscale services
dashbrr run service tailscale list
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

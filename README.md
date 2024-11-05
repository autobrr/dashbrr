# Dashbrr

A sleek, modern dashboard for monitoring and managing your media stack services.

![Main Dashboard](.github/assets/dashboard.png)

## Table of Contents

- [Features](#features)
- [Supported Services](#supported-services)
- [Tech Stack](#tech-stack)
- [Installation](#installation)
  - [Docker Installation](#docker-installation)
  - [Manual Installation](#manual-installation)
- [Configuration](#configuration)
  - [Environment Variables](#environment-variables)
  - [Authentication](#built-in-authentication)
- [Screenshots](#screenshots)

## Features

- Real-time service health monitoring
- Service-specific data display and management
- Cached data with live updates
- SSE (Server-Sent Events)
- Built-in authentication system
  - With optional **OpenID Connect (OIDC)** support
- Responsive and modern UI
- Docker support

## Supported Services

### Plex

- Active streams monitoring
- Version check

### Sonarr & Radarr

- Queue monitoring
- Stuck downloads detection
- Indexer and download client error reporting
- Version check

### Autobrr

- IRC network health monitoring
- Release statistics tracking
- Version check

### Overseerr

- Pending requests monitoring
- Version check

### Prowlarr

- Indexer health monitoring

### Maintainerr

- Rule matching statistics
- Scheduled deletion monitoring
- Version check

### Omegabrr

- Service health monitoring
- Trigger manual runs of ARRs and Lists
- Version check

### Tailscale

- Device status monitoring (online/offline)
- Device information tracking (IP, version, type)
- Update availability notifications
- Tag management and filtering
- Quick access to device details
- Device search functionality

## Tech Stack

- **Backend**
  - Go
  - Gin web framework
  - Redis for caching
  - SQLite database
- **Frontend**
  - React
  - TypeScript
  - Vite
  - TailwindCSS
  - PNPM package manager

## Installation

### Docker Installation

We provide a distroless container image for enhanced security and smaller size. You can either use our pre-built image or build it yourself.

#### Using Pre-built Image

Create a `docker-compose.yml` file:

```yaml
services:
  app:
    container_name: dashbrr
    image: ghcr.io/autobrr/dashbrr:latest
    ports:
      - "8080:8080"
    environment:
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - DASHBRR__DB_PATH=/data/dashbrr.db
      - DASHBRR__LISTEN_ADDR=0.0.0.0:8080
      #- OIDC_ISSUER=optional
      #- OIDC_CLIENT_ID=optional
      #- OIDC_CLIENT_SECRET=optional
      #- OIDC_REDIRECT_URL=optional
    volumes:
      - ./data:/data
    depends_on:
      - redis
    restart: unless-stopped
    networks:
      - dashbrr-network

  redis:
    container_name: dashbrr-redis
    image: redis:7-alpine
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes --save 60 1 --loglevel warning
    restart: unless-stopped
    networks:
      - dashbrr-network
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 3

volumes:
  redis_data:
    name: dashbrr_redis_data

networks:
  dashbrr-network:
    name: dashbrr-network
    driver: bridge
```

Start the containers:

```bash
docker-compose up -d
```

#### Building Your Own Image

Clone the repository and use either:

```bash
# Using Docker directly
docker build -t dashbrr .

# OR using Make (builds and runs everything)
make run
```

### Manual Installation

1. Install dependencies:

   - Go 1.23 or later
   - Node.js LTS
   - PNPM
   - Redis

2. Build and run:

```bash
git clone https://github.com/autobrr/dashbrr.git
cd dashbrr

# Development mode (runs frontend, backend, and Redis)
make dev

# OR Production build
make run
```

For more build options:

```bash
make help
```

## Configuration

### Environment Variables

#### Required

- `REDIS_HOST`: Redis host address (default: localhost)
- `REDIS_PORT`: Redis port number (default: 6379)
- `DASHBRR__DB_PATH`: Path to SQLite database file

#### Optional

- `DASHBRR__LISTEN_ADDR`: Listen address for the server (default: 0.0.0.0:8080)
  - Format: `<host>:<port>` (e.g., 0.0.0.0:8080)

#### Built-in Authentication

- Default authentication system
- User management through the application
- No additional configuration required

#### OpenID Connect (OIDC)

To enable OIDC authentication, set the following environment variables:

- `OIDC_ISSUER`: Your OIDC provider's issuer URL
- `OIDC_CLIENT_ID`: Client ID from your OIDC provider
- `OIDC_CLIENT_SECRET`: Client secret from your OIDC provider
- `OIDC_REDIRECT_URL`: Callback URL for OIDC authentication (default: http://localhost:3000/auth/callback)

It has been tested and working with https://auth0.com/

## Screenshots

![Main Dashboard](.github/assets/dashboard.png)
_Main dashboard showing service health monitoring and status cards_

![Draggable Card Support](.github/assets/draggable.png)
_Cards can be dragged and sorted to your liking. You can also collapse them if you wish._

![Tailscale Integration](.github/assets/tailscale.png)
_Tailscale device management and monitoring_

![Login Screen](.github/assets/login.png)
_Built-in authentication system_

![OIDC Login](.github/assets/oidc.png)
_Optional OpenID Connect (OIDC) authentication support_

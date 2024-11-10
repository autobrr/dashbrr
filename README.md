<h1 align="center">
  <img alt="autobrr logo" src=".github/assets/logo.png" width="160px"/><br/>
  Dashbrr
</h1>

<p align="center">
<a href="https://github.com/autobrr/dashbrr/releases/latest"><img alt="GitHub release (latest by date)" src="https://img.shields.io/github/v/release/autobrr/dashbrr?style=for-the-badge"></a>&nbsp;
<a href="https://goreportcard.com/report/github.com/autobrr/dashbrr"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/autobrr/dashbrr?style=for-the-badge"></a>&nbsp;
<a href="https://hub.docker.com/r/autobrr/dashbrr"><img alt="Docker Pulls" src="https://img.shields.io/docker/pulls/autobrr/dashbrr?style=for-the-badge"></a>
</p>

<p align="center">
A sleek, modern dashboard for monitoring and managing your media stack services.<br>
Dashbrr provides real-time monitoring, service health checks, and unified management for your entire media server ecosystem.
</p>

<p align="center">
<img src=".github/assets/dashboard.png" alt="Main Dashboard">
</p>

## Table of Contents

- [Features](#features)
- [Supported Services](#supported-services)
- [Installation](#installation)
  - [Docker Installation](#docker-installation)
  - [Binary Installation](#binary-installation)
- [Configuration](#configuration)
  - [Configuration File](#configuration-file)
  - [Environment Variables](#environment-variables)
  - [Command Line Interface](#command-line-interface)
  - [Authentication](#authentication)
- [Tech Stack](#tech-stack)
- [Screenshots](#screenshots)

## Features

- Real-time service health monitoring
- Service-specific data display and management
- Cached data with live updates via SSE (Server-Sent Events)
- Flexible authentication options:
  - Built-in authentication system
  - OpenID Connect (OIDC) support
- Responsive and modern UI with draggable cards
- Docker support
- Multiple database support (SQLite & PostgreSQL)
- Flexible caching system (In-memory or Redis)
- Comprehensive CLI for service management and system operations

## Supported Services

### Media Management

- **Plex**: Active streams monitoring, version check
- **Sonarr & Radarr**:
  - Comprehensive queue management:
    - Monitor active downloads
    - Stuck downloads detection and resolution
  - Error reporting for indexers and download clients
  - Version check and update notifications
- **Overseerr**: Request management, pending requests monitoring

### Download Management

- **Autobrr**: IRC network health, release statistics
- **Prowlarr**: Indexer health monitoring
- **Maintainerr**: Rule matching, scheduled deletion monitoring
- **Omegabrr**: Service health, manual ARR triggers

### Network

- **Tailscale**: Device status, information tracking, tag overview

## Installation

### Docker Installation

```bash
# Using memory cache (default)
docker compose up -d

# Using Redis cache
docker compose -f docker-compose.redis.yml up -d
```

Both configurations use PostgreSQL as the database by default. If you want to use SQLite instead, uncomment the SQLite configuration lines and comment out the PostgreSQL ones in your chosen compose file. See example configurations in [docker-compose.yml](docker-compose.yml) and [docker-compose.redis.yml](docker-compose.redis.yml).

### Binary Installation

#### Linux/macOS

Download the latest release:

```bash
wget $(curl -s https://api.github.com/repos/autobrr/dashbrr/releases/latest | grep download | grep linux_x86_64 | cut -d\" -f4)
```

#### Unpack

Run with `root` or `sudo`. If you do not have root, place the binary in your home directory (e.g., `~/.bin`).

```bash
tar -C /usr/local/bin -xzf dashbrr*.tar.gz
```

#### Systemd Service (Linux)

Create a systemd service file:

```bash
sudo nano /etc/systemd/system/dashbrr@.service
```

Add the following content:

```systemd
[Unit]
Description=dashbrr service for %i
After=syslog.target network-online.target

[Service]
Type=simple
User=%i
Group=%i
ExecStart=/usr/local/bin/dashbrr --config=/home/%i/.config/dashbrr/config.toml

[Install]
WantedBy=multi-user.target
```

Enable and start the service:

```bash
systemctl enable -q --now --user dashbrr@$USER
```

## Configuration

### Configuration File

Dashbrr uses a simple TOML configuration file. Default location: `./config.toml`

```toml
[server]
listen_addr = ":8080"

[database]
type = "sqlite"
path = "./data/dashbrr.db"
```

### Environment Variables

For a complete list of available environment variables and their configurations, see our [Environment Variables Documentation](docs/env_vars.md).

Key configuration options include:

- Server settings (listen address, ports)
- Cache configuration (Memory/Redis)
- Database settings (SQLite/PostgreSQL)
- Authentication (Built-in/OIDC)

### Command Line Interface

Dashbrr provides a CLI for managing services, user, and system operations. For detailed information about available commands and their usage, see our [Command Line Interface Documentation](docs/commands.md).

Key features include:

- Service management (add, remove, list)
- User management
- Health checks
- Version information

### Authentication

Dashbrr offers two authentication methods:

#### Built-in Authentication (Default)

Simple username/password authentication with user management through the application.

![Built-in Login](.github/assets/built-in-login.png)

![Built-in Register](.github/assets/built-in-register.png)

#### OpenID Connect (OIDC)

Enterprise-grade authentication with support for providers like Auth0.

![OIDC Login](.github/assets/OIDC-login.png)

Required OIDC environment variables:

```bash
OIDC_ISSUER=https://your-provider.com
OIDC_CLIENT_ID=your-client-id
OIDC_CLIENT_SECRET=your-client-secret
OIDC_REDIRECT_URL=http://localhost:3000/auth/callback
```

## Tech Stack

### Backend

- Go with Gin web framework
- Flexible caching: In-memory or Redis
- Database: SQLite or PostgreSQL

### Frontend

- React with TypeScript
- Vite & TailwindCSS
- PNPM package manager

## Screenshots

![Main Dashboard](.github/assets/dashboard.png)
_Main dashboard with service health monitoring and status cards_

![Built-in Authentication](.github/assets/built-in-login.png)
_Built-in authentication system_

![Built-in Register](.github/assets/built-in-register.png)
_Registration form_

![OIDC Login](.github/assets/OIDC-login.png)
_OpenID Connect (OIDC) authentication support_

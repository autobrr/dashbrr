# Environment Variables Documentation

## Server Configuration

- `DASHBRR__LISTEN_ADDR`
  - Purpose: Listen address for the server
  - Format: `<host>:<port>`
  - Default: `0.0.0.0:8080`

## Configuration Path

- `DASHBRR__CONFIG_PATH`
  - Purpose: Path to the configuration file
  - Default: `config.toml`
  - Priority: Environment variable > User config directory > Command line flag > Default value
  - Note: The application will check the following locations for the configuration file:
    1. The path specified by the `DASHBRR__CONFIG_PATH` environment variable.
    2. The user config directory (e.g., `~/.config/dashbrr`).
    3. The current working directory for `config.toml`, `config.yaml`, or `config.yml`.
    4. The `-config` command line flag can also be used to specify a different path.

## Cache Configuration

- `CACHE_TYPE`
  - Purpose: Cache implementation to use
  - Values: `"redis"` or `"memory"`
  - Default: `"memory"` (if Redis settings not configured)

### Redis Settings

(Only applicable when `CACHE_TYPE="redis"`)

- `REDIS_HOST`

  - Purpose: Redis host address
  - Default: `localhost`

- `REDIS_PORT`
  - Purpose: Redis port number
  - Default: `6379`

## Database Configuration

### SQLite Configuration

(When `DASHBRR__DB_TYPE="sqlite"`)

- `DASHBRR__DB_TYPE`
  - Set to: `"sqlite"`
- `DASHBRR__DB_PATH`
  - Purpose: Path to SQLite database file
  - Example: `/data/dashbrr.db`
  - Note: If not set, the database will be created in a 'data' subdirectory of the config file's location. This can be overridden by:
    1. Using the `-db` flag when starting dashbrr
    2. Setting this environment variable
    3. Specifying the path in the config file
  - Priority: Command line flag > Environment variable > Config file > Default location

### PostgreSQL Configuration

(When `DASHBRR__DB_TYPE="postgres"`)

- `DASHBRR__DB_TYPE`
  - Set to: `"postgres"`
- `DASHBRR__DB_HOST`
  - Purpose: PostgreSQL host address
  - Default: `postgres` (in Docker)
- `DASHBRR__DB_PORT`
  - Purpose: PostgreSQL port
  - Default: `5432`
- `DASHBRR__DB_USER`
  - Purpose: PostgreSQL username
  - Default: `dashbrr` (in Docker)
- `DASHBRR__DB_PASSWORD`
  - Purpose: PostgreSQL password
  - Default: `dashbrr` (in Docker)
- `DASHBRR__DB_NAME`
  - Purpose: PostgreSQL database name
  - Default: `dashbrr` (in Docker)

## Authentication (OIDC)

(Optional OpenID Connect configuration)

- `OIDC_ISSUER`

  - Purpose: Your OIDC provider's issuer URL
  - Required if using OIDC

- `OIDC_CLIENT_ID`

  - Purpose: Client ID from your OIDC provider
  - Required if using OIDC

- `OIDC_CLIENT_SECRET`

  - Purpose: Client secret from your OIDC provider
  - Required if using OIDC

- `OIDC_REDIRECT_URL`
  - Purpose: Callback URL for OIDC authentication
  - Example: `http://localhost:3000/auth/callback`
  - Required if using OIDC

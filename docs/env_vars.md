# Environment Variables Documentation

## Server Configuration

- `DASHBRR__LISTEN_ADDR`
  - Purpose: Listen address for the server
  - Format: `<host>:<port>`
  - Default: `0.0.0.0:8080`

### CORS (Optional)

Only needed if you serve the web UI from a different origin than the API (different host/port).

- `DASHBRR__CORS_ORIGINS`
  - Purpose: Comma-separated list of allowed origins (no wildcard when using cookies)
  - Example: `http://localhost:3000,https://dash.example.com`
  - Default: unset (allow all origins; credentials disabled)

- `DASHBRR__CORS_ALLOW_CREDENTIALS`
  - Purpose: Allow credentialed requests (cookies), required for browser auth + SSE across origins
  - Values: `true|false`
  - Default: `true` when `DASHBRR__CORS_ORIGINS` is set to an explicit allowlist; otherwise `false`

- `DASHBRR__CORS_HEADERS`
  - Purpose: Comma-separated list of allowed request headers
  - Default: `Origin,Authorization,Content-Type,Accept,X-Requested-With`

- `DASHBRR__CORS_METHODS`
  - Purpose: Comma-separated list of allowed methods
  - Default: `GET,POST,PUT,PATCH,DELETE,OPTIONS`

- `DASHBRR__CORS_MAX_AGE_HOURS`
  - Purpose: Preflight cache max-age, in hours
  - Default: `12`

## Logging

- `DASHBRR__LOG_LEVEL`
  - Purpose: Lowest log level that dashbrr writes
  - Values: `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`
  - Default: `info`
  - Note: You can also set this level in `config.toml`, as `level` under `[log]`. The environment variable has priority.

## Configuration Path

- `DASHBRR__CONFIG_PATH`
  - Purpose: Path to the configuration file
  - Default: `config.toml`
  - Priority: Environment variable > User config directory > Command line flag > Default value
  - Note: The application will check the following locations for the configuration file:
    1. The path specified by the `DASHBRR__CONFIG_PATH` environment variable.
    2. The user config directory (e.g., `~/.config/dashbrr`).
    3. The current working directory for `config.toml`, `config.yaml`, or `config.yml`.
    4. The `--config` command line flag can also be used to specify a different path.

## Database Configuration

### SQLite Configuration

(When `DASHBRR__DB_TYPE="sqlite"`)

- `DASHBRR__DB_TYPE`
  - Set to: `"sqlite"`
- `DASHBRR__DB_PATH`
  - Purpose: Path to SQLite database file
  - Example: `/data/dashbrr.db`
  - Note: If not set, the database will be created in a 'data' subdirectory of the config file's location. This can be overridden by:
    1. Using the `--db-file` flag when starting dashbrr
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

CAUTION: These four variables had no prefix before. If you use `OIDC_ISSUER`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, or `OIDC_REDIRECT_URL`, add the `DASHBRR__` prefix. Dashbrr ignores the old names and writes a warning at startup.

You can also set these four values in `config.toml`, under `[auth.oidc]`, as `issuer`, `client_id`, `client_secret`, and `redirect_url`. The environment variables have priority.

- `DASHBRR__OIDC_ISSUER`

  - Purpose: Your OIDC provider's issuer URL
  - Required if using OIDC

- `DASHBRR__OIDC_CLIENT_ID`

  - Purpose: Client ID from your OIDC provider
  - Required if using OIDC

- `DASHBRR__OIDC_CLIENT_SECRET`

  - Purpose: Client secret from your OIDC provider
  - Required if using OIDC

- `DASHBRR__OIDC_REDIRECT_URL`
  - Purpose: Callback URL for OIDC authentication
  - Example: `http://localhost:3000/api/auth/oidc/callback` (legacy `/api/auth/callback` also works)
  - Required if using OIDC

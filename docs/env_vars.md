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

## *arr Health Checks

- `DASHBRR__ARR_IGNORED_HEALTH_CHECKS`
  - Purpose: Comma-separated list of *arr health check names to ignore. An ignored check does not show in the health message and does not put the service in the warning state.
  - Applies to: Radarr, Sonarr, Lidarr, Readarr, Whisparr, and Prowlarr
  - Example: `RemovedSeriesCheck,RemovedMovieCheck`
  - Default: unset (dashbrr shows all health checks)
  - Note: The name is the `source` field of the *arr health API. Letter case and outer spaces do not matter.
  - Note: You can also set this list in `config.toml`, as `ignored_health_checks` under `[arr]`. The environment variable has priority.

## Logging

- `DASHBRR__LOG_LEVEL`
  - Purpose: Lowest log level that dashbrr writes
  - Values: `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`
  - Default: `info`
  - Note: You can also set this level in `config.toml`, as `level` under `[log]`. The environment variable has priority.

## Configuration Path

- `DASHBRR__CONFIG_PATH`
  - Purpose: Path to the configuration file
  - Priority: `--config` flag > this environment variable > user config directory > `/config` > `./config.toml`
  - Note: Without `--config` and this variable, dashbrr uses `config.toml` in the user config directory (for example `~/.config/dashbrr`), then in `/config`. If it finds none, it uses `./config.toml`.

## Database Configuration

### SQLite Configuration

(When `DASHBRR__DB_TYPE="sqlite"`)

- `DASHBRR__DB_TYPE`
  - Set to: `"sqlite"`
- `DASHBRR__DB_PATH`
  - Purpose: Path to SQLite database file
  - Example: `/data/dashbrr.db`
  - Note: This variable works alone. You do not need to set `DASHBRR__DB_TYPE` or `DASHBRR__LISTEN_ADDR` with it. A relative value is relative to the working directory.
  - Priority: `--db-file` flag > this environment variable > `[database] path` in the config file > `<config directory>/data/dashbrr.db`
  - Note: A relative `[database] path` in the config file is relative to the directory of the config file.
  - Note: If the config directory is read-only, set this variable to a writable location. Else SQLite cannot create the default database.

### PostgreSQL Configuration

(When `DASHBRR__DB_TYPE="postgres"`)

- `DASHBRR__DB_TYPE`
  - Set to: `"postgres"`
- `DASHBRR__DB_DSN`
  - Purpose: Standard PostgreSQL connection URI
  - Example: `postgres://dashbrr:password@postgres:5432/dashbrr?sslmode=verify-full&sslrootcert=/certs/root.crt&sslcert=/certs/client.crt&sslkey=/certs/client.key`
  - Optional. When set, this takes priority over the separate PostgreSQL settings below.
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

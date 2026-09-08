# Custom services

A **custom service** (service type `general`) lets Dashbrr monitor and
control an application that has no dedicated integration, by describing its
HTTP API in a small JSON definition instead of Go code.

The definition is stored as the `config` field on the service configuration
and is validated on save. Secrets inside it (`auth.password`, `auth.token`,
`login.body`) are redacted before the API ever returns the config to a
client — the raw values only ever live in the database and in outgoing
requests to the target service.

## Schema

A custom service definition has up to six top-level keys. All of them are
optional except where noted.

### `auth`

How every request (other than the optional login step) authenticates
against the service.

| Field        | Type   | Notes |
|--------------|--------|-------|
| `mode`       | string | Required when `auth` is present: `none`, `header`, `query`, `basic`, or `bearer`. |
| `headerName` | string | Header name to send, when `mode` is `header`. |
| `queryParam` | string | Query string parameter name, when `mode` is `query`. |
| `username`   | string | Used for `basic` mode, and as the `{{username}}` value in a `login.body`. |
| `password`   | string | Used for `basic` mode, and as the `{{password}}` value in a `login.body`. |
| `token`      | string | The header/query value (`header`/`query` modes) or bearer token (`bearer` mode). |

`mode: "none"` means no per-request auth is added — used for services that
are open on the network, or that only need the one-time `login` exchange
below.

### `login`

An optional one-time request performed before the first health/stat/action
call, used for services with a session/cookie login flow (for example
qBittorrent). The result is cached and reused for later requests until it
expires or a request fails auth.

| Field             | Type   | Notes |
|-------------------|--------|-------|
| `method`          | string | `GET` or `POST`. |
| `path`            | string | Path on the service, relative to its base URL. |
| `contentType`     | string | e.g. `application/x-www-form-urlencoded`. |
| `body`            | string | Request body. May contain the literal placeholders `{{username}}` and `{{password}}`, substituted from `auth.username`/`auth.password`. |
| `captureCookie`   | string | Name of a `Set-Cookie` cookie to capture from the login response. Accepts `*` wildcards (glob) for services that suffix the cookie name — the actual matched cookie name (not the pattern) is what gets injected on later requests. For example qBittorrent names its session cookie `QBT_SID_<port>` as of 5.1 (plain `SID` on older builds), so `captureCookie: "*SID*"` matches either. |
| `captureJSONPath` | string | gjson path into the login response body to capture a token instead of a cookie. |
| `injectAs`        | string | Where the captured value goes on later requests: `header`, `query`, `cookie`, or `bearer`. |
| `injectName`      | string | Name to inject the captured value under. Behavior when empty depends on `injectAs`: for `header` or `query`, the captured value is silently dropped and not applied to later requests — `injectName` is effectively required for those two modes; for `cookie`, it falls back to `captureCookie`'s name; for `bearer`, it is ignored entirely (the value always goes on the `Authorization: Bearer` header). |

### `health`

The request used to determine the service's status, and (optionally) its
version.

| Field         | Type     | Notes |
|---------------|----------|-------|
| `method`      | string   | `GET` or `POST`. |
| `path`        | string   | **Required.** Path on the service, relative to its base URL. |
| `body`        | string   | Request body, for `POST`. |
| `statusPath`  | string   | gjson path into the response body to read a status value from. If omitted, a 2xx response alone means "online". |
| `okValues`    | []string | Values at `statusPath` that count as online. If both `okValues` and `warnValues` are omitted, any non-empty value at `statusPath` (with a 2xx response) counts as online and a missing or empty value is offline. When either list is set, a value that matches neither list is offline. |
| `warnValues`  | []string | Values at `statusPath` that count as degraded rather than online or offline. |
| `versionPath` | string   | gjson path into the response body to read the version string from. Empty means "use the whole response body as the version". |

Stats (below) are read from this same health response — a custom service
makes exactly one request per poll cycle.

### `stats`

Up to 8 small values pulled from the health response and shown on the
service's card.

| Field    | Type   | Notes |
|----------|--------|-------|
| `label`  | string | **Required.** Display label. |
| `path`   | string | **Required.** gjson path into the health response body. |
| `unit`   | string | Optional display unit. |
| `format` | string | `number`, `bytes`, `duration`, `percent`, or `text`. |

### `actions`

Up to 8 buttons shown on the service's card that fire a request against the
service.

| Field     | Type   | Notes |
|-----------|--------|-------|
| `id`      | string | **Required.** Must match `^[a-z0-9-]+$`. |
| `label`   | string | **Required.** Button label. |
| `method`  | string | `GET`, `POST`, `PUT`, or `DELETE`. |
| `path`    | string | **Required.** Path on the service, relative to its base URL. |
| `body`    | string | Request body. |
| `confirm` | bool   | If true, the UI asks for confirmation before firing the action. |

### `timeoutSeconds`

Per-request timeout, in seconds. Defaults to 10 when omitted or 0.

## gjson paths

`statusPath`, `versionPath`, and every `stats[].path`/action response are
read with [gjson](https://github.com/tidwall/gjson) syntax against the JSON
response body — plain dotted paths for nested objects, `#` for array
length, `0` for an index, etc. For example, against
`{"application":{"version":"1.2.3","upTime":"2d"}}`, the path
`application.version` reads `"1.2.3"`.

## Importing a preset

Seven ready-to-use presets live in [`presets/`](../presets) in this repo,
one per supported application. Each is a plain JSON file matching the
schema above, with no secrets filled in — auth values, tokens, and
usernames/passwords are added when you configure the service instance, not
baked into the preset.

**Via the UI:** add a service (or edit an existing one) and pick the
**General Service** template (service type `general`). In the "Custom
service" section that appears, open **Import JSON** and paste the contents
of the preset file you want (e.g. `presets/cleanuparr.json`), then click
**Validate & import** — this fills in the Authentication, Login step,
Health check, Stats, and Actions subsections from the preset. Fill in the
base URL field and any credentials the preset leaves blank: the header/query
value or bearer token under Authentication, or the username/password fields
(which appear whenever a login step is configured, since the login body's
`{{username}}`/`{{password}}` placeholders are filled from them). Optionally
click **Test** in the same section to run the health/stats request once
without saving, then **Save**.

**Via the CLI:**

```sh
dashbrr service generic add <url> <name> [apiKey] --config presets/cleanuparr.json
```

`<url>` is the service's base URL (e.g. `http://cleanuparr.local:11011`),
`<name>` is the display name, the optional positional `[apiKey]` is the
service's API key/token, and `--config` points at the preset (or a
hand-written definition following the same schema) — when given, it
supplies the whole definition and overrides the individual flags below.

Instead of `--config`, a definition can be built up from flags: `--auth-mode`,
`--header-name`, `--query-param`, `--username`, `--password`, `--bearer`,
`--health-path`, `--status-path`, `--ok`, `--warn`, `--version-path`, and a
repeatable `--stat "Label=json.path[:unit[:format]]"`. There is no
per-flag way to configure a `login` step — services that need one (like
qBittorrent) must be added with `--config`.

Use `dashbrr service generic test <url> [--config file.json] [apiKey]` to
run a preset's health/stats request once and print the resulting
status/version/stats without saving anything — useful for checking a preset
against a real instance before running `add`.

## Presets

| Preset | File | Auth needed | Notes |
|--------|------|-------------|-------|
| Cleanuparr | `presets/cleanuparr.json` | API key (`X-Api-Key` header) | Health also reports version and uptime/memory stats. |
| Home Assistant | `presets/home-assistant.json` | Long-lived access token (bearer) | Health only; `/api/` returns `"message": "API running."` when healthy. |
| Audiobookshelf | `presets/audiobookshelf.json` | API token (bearer) | Health only — `/healthcheck` returns a plain `OK` body. |
| slskd | `presets/slskd.json` | API key (`X-API-Key` header) | Reports the Soulseek connection state as a stat. |
| AzuraCast | `presets/azuracast.json` | None | Public station status endpoint; per-station stats are left out since they vary by install. |
| Dozzle | `presets/dozzle.json` | None | Health only. |
| qBittorrent | `presets/qbittorrent.json` | Username/password (session login) | Uses the login flow to capture the session cookie via the wildcard pattern `*SID*` (matches `QBT_SID_<port>` on 5.1+ or plain `SID` on older builds); reports transfer speed and DHT nodes; adds Pause All / Resume All actions. |

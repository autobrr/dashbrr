# Prometheus metrics in qui, and what dashbrr can copy

Date: 2026-10-17. Scope: how qui exposes Prometheus metrics (configuration, listener, basic auth, registry, collectors, metric names, tests, user docs), compared with autobrr, and how dashbrr can copy the setup.

## Summary

qui copies the autobrr pattern. A separate, opt-in HTTP listener serves `/metrics` from a custom Prometheus registry. One difference is important for dashbrr. qui does not use bcrypt. It compares plaintext passwords. autobrr uses bcrypt hashes. The dashbrr decision "bcrypt hashes in autobrr/qui format" matches autobrr only.

- Configuration. qui has four flat camelCase keys: `metricsEnabled` (default `false`), `metricsHost` (default `127.0.0.1`), `metricsPort` (default `9074`), and `metricsBasicAuthUsers` (default empty). Host and port are separate keys. The env vars are `QUI__METRICS_ENABLED`, `QUI__METRICS_HOST`, `QUI__METRICS_PORT`, and `QUI__METRICS_BASIC_AUTH_USERS`. autobrr uses the same key names, the same defaults, and the `AUTOBRR__` prefix.
- Listener. qui runs a chi router on its own `http.Server`, with one route, `GET /metrics`. `ReadHeaderTimeout` is 15 s. qui has no graceful shutdown for this listener. If the metrics port cannot bind, the whole qui process stops. autobrr logs the error and keeps running.
- Basic auth. Both apps use comma-separated `user:secret` entries and the realm `metrics`. A failure returns `401` with `WWW-Authenticate: Basic realm="metrics"`. qui stores plaintext and uses chi's `middleware.BasicAuth`, which compares in constant time. autobrr stores bcrypt hashes and calls `bcrypt.CompareHashAndPassword` on each request, with no cache. In both apps, a configured value with no valid entry rejects every request.
- Library. Both apps use `github.com/prometheus/client_golang v1.24.1`. Neither has another direct Prometheus dependency.
- Registry. Both apps use a custom `prometheus.NewRegistry()` with the Go collector and the process collector. qui serves it with `promhttp.HandlerFor(..., HandlerOpts{EnableOpenMetrics: true})`. autobrr uses empty `HandlerOpts{}`.
- Collectors. Both apps use custom `prometheus.Collector` types that build const metrics in `Collect`, with no `GaugeVec`. The qui torrent collector does not only read a snapshot. At scrape time it lists instances from SQLite, can run a qBittorrent health check, and can refresh a stale sync cache.
- Metric names. qui uses the `qbittorrent_` prefix for 15 instance metrics and `qui_` for one database metric. Instance metrics carry both `instance_id` and `instance_name`. qui has no `qui_info` and no `qui_up` metric. autobrr has `autobrr_info`.
- Tests. qui uses `httptest` against the router for auth, and `testutil.CollectAndCount` for the registry. It does not use `testutil.CollectAndCompare`.
- User docs. qui has one page, `documentation/docs/advanced/metrics.md`, rows in the configuration reference and the env var page, and a Grafana dashboard JSON file.

## Sources used

- qui source: `~/github/autobrr/qui`, branch `develop`, commit `5b7e0939` (2026-10-17). A file reference without a repo name is in qui at this commit.
- autobrr source: `~/github/autobrr/autobrr`, branch `develop`, commit `c6b78bdd`. These references start with `autobrr:`.
- Library source in the Go module cache: chi `v5.3.2` (`middleware/basic_auth.go`), client_golang `v1.24.1`, prometheus/common `v0.70.1` (`expfmt/openmetrics_create.go`), and golang.org/x/crypto `v0.57.0` (`bcrypt/bcrypt.go`).
- qui git history for `internal/metrics/server.go`: commits `e740612c` (#147), `f818025d` (#2431), and `572ed6c7` (#2633).
- dashbrr source at `develop` `3a1e7da`, for the last section only.

This research made no network requests. It did not run qui or its tests.

## 1. Configuration keys

| Key | Type | Default | Env var | Source |
|---|---|---|---|---|
| `metricsEnabled` | bool | `false` | `QUI__METRICS_ENABLED` | `internal/config/config.go:152`, `:263` |
| `metricsHost` | string | `"127.0.0.1"` | `QUI__METRICS_HOST` | `internal/config/config.go:153`, `:264` |
| `metricsPort` | int | `9074` | `QUI__METRICS_PORT` | `internal/config/config.go:154`, `:265` |
| `metricsBasicAuthUsers` | string | `""` | `QUI__METRICS_BASIC_AUTH_USERS` | `internal/config/config.go:155`, `:266` |

- The keys are at the top level of the TOML file, not in a section. The struct tags are `toml:"metricsEnabled"` and so on (`internal/domain/config.go:52-55`).
- The env prefix is `QUI__` (`internal/config/config.go:34`). Viper reads the values at `internal/config/config.go:395-398`.
- Host and port are separate keys. The server joins them with `fmt.Sprintf("%s:%d", host, port)` (`internal/metrics/server.go:70`). autobrr does the same (`autobrr: internal/http/metrics_server.go:56`).
- The comments in the configuration template (`internal/config/config.go:683-703`) say this:
  - Enabled: "Enable Prometheus metrics on a separate port". The default is false.
  - Host: "bind address for metrics endpoint". Set `0.0.0.0` to bind to all interfaces.
  - Port: "separate from main web interface", "9074 (standard Prometheus range)".
  - Basic auth: `"username:password"` or `"user1:password1,user2:password2"`. "Passwords are plaintext and can contain colons. Usernames cannot contain colons. Commas cannot appear in usernames or passwords. Protect this configuration file." The example is `"prometheus:secret"`. An empty value turns auth off.
- autobrr has the same four keys and defaults (`autobrr: internal/domain/config.go:41-44`, `autobrr: internal/config/config.go:288-291`). Its env vars are `AUTOBRR__METRICS_ENABLED`, `AUTOBRR__METRICS_HOST`, `AUTOBRR__METRICS_PORT`, and `AUTOBRR__METRICS_BASIC_AUTH_USERS` (`autobrr: internal/config/config.go:28`, `:435-449`). The autobrr string env reader also accepts a `_FILE` suffix (`autobrr: internal/config/config.go:452-460`). The autobrr template says: "Comma separate list of user:password. Password must be htpasswd bcrypt hashed. Use autobrrctl to generate." (`autobrr: internal/config/config.go:131-149`).

## 2. The metrics server

- Start. qui builds the server only when `metricsEnabled` is true (`cmd/qui/main.go:967-981`). It starts after the main HTTP server reports ready (`cmd/qui/main.go:953-962`). A goroutine calls `ListenAndServe` and sends the result to `errorChannel`.
- Router. qui uses chi (`github.com/go-chi/chi/v5 v5.3.2`). The middleware is `RequestID`, `RealIP`, `Recoverer`, and then `BasicAuth` when auth is set (`internal/metrics/server.go:45-55`).
- Path. The only route is `GET /metrics` (`internal/metrics/server.go:65-68`). Other paths get the chi 404.
- Timeouts. The server sets `ReadHeaderTimeout: 15 * time.Second`. It sets no read, write, or idle timeout (`internal/metrics/server.go:71-78`).
- Logging. qui writes one info line at start, "Starting Prometheus metrics server", with the address (`internal/metrics/server.go:84-86`). It writes one debug line per scrape, "Serving Prometheus metrics" (`internal/metrics/server.go:66`). It writes no access log.
- Shutdown. The shutdown function stops only the main HTTP server (`cmd/qui/main.go:1010-1031`). qui never calls `Shutdown` on the metrics server. The process ends with `os.Exit` (`cmd/qui/main.go:1049`).
- Listen failure. `ListenAndServe` sends the bind error to `errorChannel`. The main `select` reads it and logs "got unexpected error from server". Then qui runs the shutdown and exits with code 0 (`cmd/qui/main.go:1036-1037`, `:1045-1049`). So a busy metrics port stops all of qui.
- autobrr. `Listen` binds first and returns the error (`autobrr: internal/http/metrics_server.go:55-70`). The metrics component has `optional: true` (`autobrr: internal/application/application.go:263-279`). When an optional component fails to start, autobrr logs the error and skips the component (`autobrr: internal/application/component.go:39-43`). autobrr has a real `Shutdown` with a `Close` fallback (`autobrr: internal/http/metrics_server.go:80-92`). It adds a zerolog request logger (`autobrr: internal/http/metrics_server.go:97-101`).

## 3. Basic auth

### qui

- Format. The value is a comma-separated list. Each entry is `user:password`. qui trims spaces around each entry. Then it splits on the first colon with `strings.Cut`, so a password can contain colons (`internal/metrics/server.go:34-43`).
- Bad entry. qui logs an entry with no colon as a warning and skips it (`internal/metrics/server.go:40`).
- Hash algorithm. There is none. The password is plaintext.
- Compare. qui uses chi `middleware.BasicAuth("metrics", users)` (`internal/metrics/server.go:53-55`). chi looks up the user in the map and compares with `subtle.ConstantTimeCompare` (chi `middleware/basic_auth.go:19-20`).
- Failure. The response is `401` with `WWW-Authenticate: Basic realm="metrics"` and an empty body (chi `middleware/basic_auth.go:30-33`).
- Fail closed. qui turns on the middleware when the string is not empty, even if no entry is valid. Then every request gets 401 (`internal/metrics/server.go:27`, `:53`). The test `TestMetricsServerMalformedBasicAuthFailsClosed` covers this (`internal/metrics/server_test.go:45-58`).
- History. The template said bcrypt until #2431, but the code always called the chi plaintext `BasicAuth`. Commit `f818025d` (#2431) changed the docs to say plaintext. The old wrapper had the comment "matches autobrr implementation", but it did not hash (`git show f818025d^:internal/metrics/server.go`).

### autobrr

- Format. The value is a comma-separated list of `user:hash`. autobrr splits each entry with `strings.Split(cred, ":")`. It keeps an entry only if it has exactly two parts. It does not trim spaces, and it skips bad entries without a log line (`autobrr: internal/http/middleware.go:168-179`).
- Hash algorithm. autobrr uses bcrypt. It calls `bcrypt.CompareHashAndPassword` on each request (`autobrr: internal/http/middleware.go:189-196`). It has no cache.
- Unknown user. autobrr skips bcrypt and returns 401 at once (`autobrr: internal/http/middleware.go:190-198`). A known user costs one bcrypt compare.
- Failure. The response is `401` with `WWW-Authenticate: Basic realm="metrics"` (`autobrr: internal/http/middleware.go:203-206`).
- Hash tool. `autobrrctl htpasswd` reads a password and prints `bcrypt.GenerateFromPassword(..., DefaultCost)` (`autobrr: cmd/autobrrctl/main.go:312-324`, `:432-442`).
- Apache hashes. A `$2y$` hash from `htpasswd -B` works. x/crypto `bcrypt` accepts any minor version letter (`bcrypt/bcrypt.go:265-279`).

## 4. Library

- qui has `github.com/prometheus/client_golang v1.24.1` as a direct dependency (`go.mod:35`).
- The indirect dependencies are `client_model v0.6.2`, `common v0.70.1`, and `procfs v0.21.1` (`go.mod:88-90`).
- autobrr has the same four versions (`autobrr: go.mod:45`, `:102-104`).

## 5. Registry and handler

- qui uses a custom registry, `prometheus.NewRegistry()`, not the default registry (`internal/metrics/metrics.go:23`).
- It registers `collectors.NewGoCollector()` and `collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})` (`internal/metrics/metrics.go:26-27`). Then it registers the torrent collector and the database collector (`internal/metrics/metrics.go:30-32`).
- The handler is `promhttp.HandlerFor(registry, promhttp.HandlerOpts{EnableOpenMetrics: true})` (`internal/metrics/server.go:58-63`). `ErrorHandling` keeps its default, `HTTPErrorOnError`. With that default, a gather error returns HTTP 500 (client_golang `promhttp/http.go:312-314`, `:481`).
- autobrr has the same registry setup. It also has a `GaugeFunc` named `autobrr_info` with the const labels `version`, `build_time`, and `revision`, and the value 1 (`autobrr: internal/metrics/metrics.go:41-62`). Its handler uses empty `HandlerOpts{}` (`autobrr: internal/http/metrics_server.go:107`).

## 6. Collector design

- Type. qui uses custom collectors that implement `Describe` and `Collect`. The constructor builds one `*prometheus.Desc` for each metric (`internal/metrics/collector/torrent.go:42-139`). `Describe` sends all 15 descs (`internal/metrics/collector/torrent.go:141-157`). `Collect` sends `prometheus.MustNewConstMetric` values. qui uses no `GaugeVec`.
- Data source at scrape time. All references in this list are in `internal/metrics/collector/torrent.go:170-381` unless named:
  - `Collect` makes its own `context.WithTimeout(context.Background(), 30*time.Second)` (`:171`).
  - It lists instances from SQLite through `ClientPool.GetAllInstances` (`:189`, `internal/qbittorrent/metrics.go:18-35`).
  - For each active instance, it calls `ClientPool.GetClient` (`:200`). If the client is unhealthy and not in backoff, `GetClient` can run a live health check against qBittorrent (`internal/qbittorrent/pool.go:291-330`).
  - It calls `SyncManager.GetTorrentsWithFilters` with limit 100000 and no filters (`:239`). With a background context, the main-data read uses the checked getter. A code comment says this getter "refreshes a stale cache" (`internal/qbittorrent/maindata.go:116-122`, `:139-142`). So a scrape can start a qBittorrent sync.
- Concurrency. The collector has no lock. It reads the instances one after another. The registry can run different collectors in parallel.
- Instance down. qui always emits `qbittorrent_instance_connection_status` (1 or 0) for each instance (`:211-217`). A disabled or disconnected instance emits nothing else (`:219-230`).
- Source failure. If `GetTorrentsWithFilters` fails, qui logs a warning. Then it emits `qbittorrent_scrape_errors_total{type="torrent_counts"}` with the value 1 (`:159-168`, `:240-247`). This metric has the counter type but keeps no count. Its value is 1 on a failed scrape, and the series is absent on a good scrape. The docs say this and tell users to use `sum_over_time`, not `rate` (`documentation/docs/advanced/metrics.md:72`, `:141-151`).
- Nil dependencies. `Collect` returns with no metrics when the client pool is nil (`:174-177`).
- Database collector. The database code increments a package-level `atomic.Uint64`. The collector reads it at scrape time (`internal/database/metrics.go:12-43`). This collector only reads a snapshot.
- autobrr. autobrr has one collector for each domain: release, IRC, feed, list, and filter. Each collector calls its service with `context.TODO()`. It reports a failure with `prometheus.NewInvalidMetric` (for example `autobrr: internal/metrics/collector/irc.go:42-46`). With the default handler options, one failed service makes the whole scrape return HTTP 500.

## 7. Metric names

The table lists all qui metrics, from `internal/metrics/collector/torrent.go:48-137` and `internal/database/metrics.go:24-29`.

| Name | Type | Labels | Help |
|---|---|---|---|
| `qbittorrent_torrents_downloading` | gauge | `instance_id`, `instance_name` | Number of downloading torrents by instance |
| `qbittorrent_torrents_seeding` | gauge | same | Number of seeding torrents by instance |
| `qbittorrent_torrents_paused` | gauge | same | Number of paused torrents by instance |
| `qbittorrent_torrents_error` | gauge | same | Number of torrents in error state by instance |
| `qbittorrent_torrents_checking` | gauge | same | Number of torrents being checked by instance |
| `qbittorrent_session_download_bytes` | counter | same | Total downloaded data in bytes per session by instance |
| `qbittorrent_session_upload_bytes` | counter | same | Total uploaded data in bytes per session by instance |
| `qbittorrent_alltime_download_bytes` | counter | same | Total downloaded data in bytes by instance |
| `qbittorrent_alltime_upload_bytes` | counter | same | Total uploaded data in bytes by instance |
| `qbittorrent_instance_connection_status` | gauge | same | Connection status of qBittorrent instance (1=connected, 0=disconnected) |
| `qbittorrent_scrape_errors_total` | counter | `instance_id`, `instance_name`, `type` | Total number of scrape errors by instance |
| `qbittorrent_tracker_torrents` | gauge | `instance_id`, `instance_name`, `tracker_name` | Number of torrents by tracker |
| `qbittorrent_tracker_uploaded_bytes` | gauge | same | Total uploaded data in bytes by tracker |
| `qbittorrent_tracker_downloaded_bytes` | gauge | same | Total downloaded data in bytes by tracker |
| `qbittorrent_tracker_total_size_bytes` | gauge | same | Total content size in bytes by tracker |
| `qui_db_wedged_transaction_total` | counter | none | Number of times BeginTx detected a wedged transaction (indicates a bug) |

qui also exports the standard `go_*` and `process_*` series.

- Prefix. qui uses `qbittorrent_` for instance data and `qui_` for its own state. It has no `qui_info` and no `qui_up`.
- Labels. `instance_id` is the database ID as a string. `instance_name` is the current name (`internal/metrics/collector/torrent.go:194-195`).
- Rename and delete. qui builds all series from the current instance list on each scrape. A renamed instance gets a new series with the new `instance_name`, and the old series goes stale in Prometheus. A deleted instance stops appearing. qui has no special code for either case.
- autobrr. autobrr names start with `autobrr_`. Many gauges end in `_total`, for example `autobrr_irc_total` and `autobrr_filter_enabled_total`. Each collector has an `autobrr_<domain>_collector_error` desc for invalid metrics (`autobrr: internal/metrics/collector/*.go`).

## 8. Tests

- Auth. `internal/metrics/server_test.go` calls `NewMetricsServer` with an empty registry. Then it sends `httptest.NewRequest` to `server.server.Handler` and reads an `httptest.NewRecorder`. One test covers a password with colons (`:15-43`). One test covers a malformed value that fails closed (`:45-58`).
- Registry. `internal/metrics/manager_test.go` tests that the manager builds, that two managers have separate registries, and that `testutil.CollectAndCount(registry)` is positive (`:75-85`).
- Collector. `TestTorrentCollector_Describe` expects 15 descs (`internal/metrics/manager_test.go:87-102`). `TestTorrentCollector_CollectWithNilDependencies` expects 0 metrics (`:104-114`). There are also two benchmarks.
- Tracker grouping. A table-style unit test covers the pure function `groupTrackerTransfersForMetrics` (`internal/metrics/collector/torrent_test.go:15-53`).
- No test uses `testutil.CollectAndCompare`. No test reads real metric values from a running collector.

## 9. User docs

- Main page. `documentation/docs/advanced/metrics.md`, with the sidebar label "Metrics". Its sections are a security warning (no auth means an open endpoint), "Enable metrics" (TOML and env examples), the auth format, "Configure Prometheus" (scrape configuration), "qui metrics" (a table with type, labels, and reset behavior), "Tracker metric limits", "PromQL examples", and "Grafana dashboard".
- Example scrape configuration (`documentation/docs/advanced/metrics.md:44-54`):

  ```yaml
  scrape_configs:
    - job_name: qui
      static_configs:
        - targets: ["localhost:9074"]
      metrics_path: /metrics
      scrape_interval: 30s
      # basic_auth:
      #   username: prometheus
      #   password: yourpassword
  ```

- The page tells users to watch `up{job="qui"}`, because qui cannot report that its own endpoint is unreachable (`documentation/docs/advanced/metrics.md:159`).
- Configuration reference rows: `documentation/docs/configuration/reference.md:79-82`. Env var block: `documentation/docs/configuration/environment.md:130-136`.
- Grafana dashboard: `documentation/static/examples/qui-grafana-dashboard.json`. The metrics page links to it.
- The README does not mention metrics.

## Problems in qui and autobrr

These are facts from the source. Each item says what goes wrong.

1. qui never hashed metrics passwords, but the template said bcrypt until #2431. A user who followed the old template and pasted a bcrypt hash had to send the hash string itself as the password.
2. A busy metrics port stops all of qui, and the exit code is 0 (`cmd/qui/main.go:979`, `:1036-1049`). autobrr logs the error and continues.
3. Both apps join host and port with `fmt.Sprintf("%s:%d")` (`internal/metrics/server.go:70`, `autobrr: internal/http/metrics_server.go:56`). An IPv6 host such as `::1` gives `::1:9074`, which does not parse. `net.JoinHostPort` gives the correct form.
4. `qbittorrent_scrape_errors_total` is a counter that keeps no count. Its value is always 1 when present. `rate()` and `increase()` give wrong results. The docs warn about this, but the type and the name are still wrong.
5. qui turns on `EnableOpenMetrics`. The OpenMetrics encoder gives the type `unknown` to a counter whose name does not end in `_total` (prometheus/common `expfmt/openmetrics_create.go:84-88`). So when Prometheus asks for OpenMetrics, the four `qbittorrent_*_bytes` counters lose their counter type.
6. A qui scrape can do live work: a database read, a health check, and a sync refresh. The collector timeout (30 s) is longer than the Prometheus default scrape timeout (10 s).
7. qui logs a malformed auth entry with `redact.BasicAuthUser`. That function returns the input unchanged when it has no colon (`pkg/redact/redact.go:180-189`). If a user forgets the colon, the log shows the whole secret.
8. autobrr names many gauges with `_total`, for example `autobrr_irc_total`. The Prometheus naming convention keeps `_total` for counters.
9. The autobrr template says auth is "Only enabled if correctly set with user:pass". In fact, any non-empty value turns auth on. If no entry parses, every request gets 401.

## What dashbrr should copy / adapt

This section gives recommendations, not facts. It assumes the decisions that soup already made: an opt-in separate listener, optional bcrypt basic auth, and client_golang with a custom collector that reads an in-memory snapshot.

The spec in #216 and ADR 0004 replaced some of these recommendations. Dashbrr stores plain-text passwords, turns on `EnableOpenMetrics`, and uses the labels `instance_id` and `instance_name`.

1. Configuration (from qui and autobrr, adapted). Add a `[metrics]` section with `enabled` (default `false`), `listen_addr` (default `127.0.0.1:9075`), and `basic_auth_users` (default empty). The env vars are `DASHBRR__METRICS_ENABLED`, `DASHBRR__METRICS_LISTEN_ADDR`, and `DASHBRR__METRICS_BASIC_AUTH_USERS`. One listen address matches `[server] listen_addr` and avoids problem 3. Port 9075 does not clash with autobrr and qui on 9074.
2. Listener (from autobrr). Run a second `http.Server` with `ReadHeaderTimeout: 15s`. Bind with `net.Listen` first, as `internal/api/server.go:44` does, so a bind error appears at start. If the bind fails, log the error and keep the dashboard running. This is the autobrr behavior, not the qui behavior. Shut the listener down next to `srv.Shutdown` in `cmd/dashbrr/main.go`. A plain `http.ServeMux` with the one route `GET /metrics` is enough. gin and chi add nothing for one route. Log one info line with the address at start.
3. Basic auth (autobrr format, qui parsing). Use a comma-separated list of `user:bcrypthash`. Trim spaces and split on the first colon. Accept any hash that x/crypto bcrypt accepts. This includes `$2y$` from `htpasswd -nbB user pass`, which prints a complete entry. Compare with `bcrypt.CompareHashAndPassword`. On failure, return 401 with `WWW-Authenticate: Basic realm="metrics"`. If the value is set, fail closed. Neither app tests the hashes at load. dashbrr can test each hash with `bcrypt.Cost` when it loads the configuration, and stop with a clear error if an entry is malformed or not a bcrypt hash. Then a user who pastes a qui-style plaintext password gets an error at start, not a 401 on every scrape. Do not log the entry. Do not cache bcrypt results, because one compare per scrape every 15 to 60 s costs little CPU. The fast 401 for an unknown user shows only whether a username exists, which is acceptable for single-user software. dashbrr already imports `golang.org/x/crypto/bcrypt`. A `dashbrr` subcommand that prints a hash, like `autobrrctl htpasswd`, is a small addition if soup wants it.
4. Library (from qui). Use `github.com/prometheus/client_golang`. qui and autobrr use v1.24.1. Read the release time before `go get`, because of the 7-day rule.
5. Registry (from qui). Use `prometheus.NewRegistry()` with `collectors.NewGoCollector()` and `collectors.NewProcessCollector(...)`. Serve it with `promhttp.HandlerFor(reg, promhttp.HandlerOpts{})`. Leave `EnableOpenMetrics` off, unless every counter name ends in `_total` (problem 5). Add a `dashbrr_info{version,revision}` gauge with the value 1, as autobrr does.
6. Collector (from the qui database collector). Write one custom collector. `Describe` sends fixed descs. `Collect` copies the snapshot under a read lock and emits `MustNewConstMetric` values. It makes no upstream call and needs no context timeout. A down service is data in the snapshot (for example an `up` value of 0), not a scrape error. So do not use `NewInvalidMetric` for it, because the autobrr pattern makes the whole scrape fail. The poller's `Broadcaster.latest` map (`internal/api/handlers/broadcast.go:15-20`) is the obvious source. It has no delete path, so a deleted service keeps its series until dashbrr restarts. To fix this, add a remove call when a service is deleted, or join the snapshot with the current service list.
7. Names (qui label style). Use the prefix `dashbrr_`. Use a stable ID label and a display name label, as qui does with `instance_id` and `instance_name`. For example, use `service_id`, `service_name`, and `service_type`. Then a rename makes a new series and the old one goes stale, which is the qui behavior. Use base units (`_seconds`, `_bytes`). Put `_total` only on real counters. Do not copy `qbittorrent_scrape_errors_total` or the autobrr `_total` gauges.
8. Tests (qui, improved). Use `httptest` against the metrics handler for auth. Cover no auth, a good hash, a wrong password, an unknown user, and a malformed value at load. For the collector, fill a snapshot and use `testutil.CollectAndCompare` with the expected exposition text. This tests names, labels, and values. qui does not do this.
9. Docs (from the qui page). Add a page under `docs/`, for example `docs/metrics.md`, in the same order as the qui page. Put a security warning first. Then give the enable steps (TOML and env), the auth format with the `htpasswd -nbB` example, and a Prometheus scrape configuration with `basic_auth`. Then add a metric table with type, labels, and reset behavior, a note to watch `up{job="dashbrr"}`, and PromQL examples. Add rows to `docs/env_vars.md` and `docs/config_management.md`. A Grafana dashboard can come later.

Do not copy these qui behaviors:

- The plaintext password format (problem 1).
- Exiting the app on a metrics bind error (problem 2).
- Separate host and port keys (problem 3).
- A counter that is always 1 (problem 4), and `EnableOpenMetrics` with counters that do not end in `_total` (problem 5).
- Live work during a scrape (problem 6).
- Logging a malformed auth entry (problem 7).

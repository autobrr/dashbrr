# Prometheus metrics

Dashbrr can serve Prometheus metrics for each configured service. The metrics come from the health checks that dashbrr already runs. A scrape reads the last result of each health check from memory. A scrape does not send a request to a service.

The metrics listener is off by default. When you turn it on, dashbrr serves `GET /metrics` on a separate address, not on the address of the dashboard.

## Turn on metrics

Do not bind the metrics listener to `0.0.0.0` without basic auth. Any host on your network can then read the names and versions of your services.

Add a `[metrics]` section to `config.toml`:

```toml
[metrics]
enabled = true
listen_addr = "127.0.0.1:9075"
# basic_auth_users = "prometheus:secret"
```

You can also use environment variables. An environment variable has priority over `config.toml`. An empty environment variable counts as not set.

| Key | Environment variable | Default |
| --- | --- | --- |
| `enabled` | `DASHBRR__METRICS_ENABLED` | `false` |
| `listen_addr` | `DASHBRR__METRICS_LISTEN_ADDR` | `127.0.0.1:9075` |
| `basic_auth_users` | `DASHBRR__METRICS_BASIC_AUTH_USERS` | empty |

The default port is 9075, because autobrr and qui use 9074. In Docker, set `listen_addr` to `0.0.0.0:9075` and publish the port. Basic auth does not encrypt the password. Publish the port only on a network that you trust, or put a TLS proxy in front of it. If dashbrr cannot bind the address, it writes an error to the log and runs without metrics. Dashbrr reads `[metrics]` at start. To apply a change, restart dashbrr.

## Basic auth

`basic_auth_users` is a comma-separated list of `user:password` entries, for example `prometheus:secret,grafana:other`. An empty value turns basic auth off.

- A password can contain colons. A user cannot.
- A user or a password cannot contain commas.
- The passwords are in plain text, so keep the configuration file private. [ADR 0004](adr/0004-metrics-basic-auth-stores-plain-text-passwords.md) gives the reason.

If an entry has no colon, an empty user, or an empty password, dashbrr does not start. The error gives the position of the entry, not the entry.

A request with no credentials or with wrong credentials gets `401` with `WWW-Authenticate: Basic realm="metrics"`.

## Configure Prometheus

```yaml
scrape_configs:
  - job_name: dashbrr
    static_configs:
      - targets: ["localhost:9075"]
    basic_auth:
      username: prometheus
      password: secret
```

If you did not set `basic_auth_users`, remove `basic_auth`.

## Metrics

Each service metric has these labels:

- `instance_id`: the ID of the service in dashbrr, for example `radarr-1`.
- `instance_name`: the display name of the service.
- `service_type`: the service type, for example `radarr`.

| Name | Value |
| --- | --- |
| `dashbrr_service_up` | 1 when the service status is `online` or `warning`, else 0 |
| `dashbrr_service_status` | One series for each service status, with a `status` label: `online`, `warning`, `offline`, `error`, `pending`, or `unknown`. The series of the current status is 1. The others are 0. |
| `dashbrr_service_response_time_seconds` | Response time of the last health check |
| `dashbrr_service_update_available` | 1 when an update is available, else 0 |
| `dashbrr_service_info` | Always 1. The `version` label holds the version of the service. It is empty when dashbrr does not know the version. |
| `dashbrr_service_last_check_timestamp_seconds` | Unix time of the last health check |
| `dashbrr_info` | Always 1. The `version` label holds the version of dashbrr. This metric has no service labels. |

All metrics are gauges. The endpoint also serves the standard `go_*` and `process_*` metrics.

- A service gets no series until its first health check ends, for example just after dashbrr starts.
- When you delete a service, its series stop on the next scrape.
- When you rename a service, the `instance_name` label changes. Prometheus then starts a new series.
- The text of a warning is not in the metrics. Use the `status="warning"` series.

If dashbrr cannot read the service list from its database, the scrape answers HTTP 500. Prometheus then sets `up{job="dashbrr"}` to 0.

## Alert rule example

This rule fires when a service is down for 5 minutes:

```yaml
groups:
  - name: dashbrr
    rules:
      - alert: ServiceDown
        expr: dashbrr_service_up == 0
        for: 5m
        annotations:
          summary: "{{ $labels.instance_name }} is down"
```

Also alert on `up{job="dashbrr"} == 0`. Dashbrr cannot report that Prometheus cannot reach it.

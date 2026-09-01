# Operations

Operational extras layered on top of the core product.

## Prometheus metrics

`GET /metrics` on the control plane (unauthenticated; firewall to your monitoring
network in prod) exposes:

| Metric                          | Type      | Notes                                    |
|---------------------------------|-----------|------------------------------------------|
| `cc_http_requests_total`        | counter   | Labels: `method`, `path`, `status`.      |
| `cc_http_request_duration_seconds` | histogram | Labels: `method`, `path`. Standard Prom buckets. |
| `cc_agents_connected`           | gauge     | Agents with an open AgentStream.         |
| `cc_runs_total`                 | counter   | Label: `status` (succeeded/failed/...).  |
| `cc_log_subscribers`            | gauge     | Active SSE log subscribers across runs.  |
| `go_*`, `process_*`             | various   | Default Go runtime + process metrics.    |

Scrape config:

```yaml
- job_name: croncompose
  static_configs:
    - targets: ["control-plane:8080"]
  metrics_path: /metrics
```

## OIDC SSO

Opt in by setting the four `OIDC_*` env vars on the control plane:

| Variable             | Purpose                                                   |
|----------------------|-----------------------------------------------------------|
| `OIDC_ISSUER_URL`    | e.g. `https://login.example.com`. Discovery doc is read at startup. |
| `OIDC_CLIENT_ID`     | OIDC client id.                                           |
| `OIDC_CLIENT_SECRET` | OIDC client secret (omit for public clients).            |
| `OIDC_REDIRECT_URL`  | e.g. `https://cc.example.com/api/v1/auth/oidc/callback`. |
| `OIDC_DEFAULT_ROLE`  | role assigned on first SSO login. Default `viewer`.       |

Flow:

1. `GET /api/v1/auth/oidc/start` redirects the browser to the provider with a fresh
   state cookie.
2. Provider calls `/api/v1/auth/oidc/callback`. The control plane validates state,
   exchanges the code, verifies the `id_token`, reads `email` + `name` claims.
3. User is looked up by email; missing users are auto-provisioned with
   `OIDC_DEFAULT_ROLE` and an empty password hash (SSO-only).
4. Session cookie is set and the browser is redirected to `/` (or the saved `next`).

The web UI reads `GET /api/v1/auth/config` on the login page and shows a
"Sign in with SSO" button when OIDC is enabled. Password login keeps working
alongside SSO.

## Agent packaging

`.deb` and `.apk` packages are built on tag push by
`.github/workflows/release.yml` for `linux/amd64` and `linux/arm64`:

```
croncompose-agent_<version>_<arch>.deb
croncompose-agent_<version>_<arch>.apk
SHA256SUMS-<arch>.txt
```

Install:

```sh
# Debian / Ubuntu
sudo dpkg -i croncompose-agent_v0.1.0_amd64.deb
sudo systemctl edit /etc/croncompose/agent.env   # set CONTROL_PLANE_*
sudo systemctl enable --now croncompose-agent

# Alpine
sudo apk add --allow-untrusted croncompose-agent_v0.1.0_amd64.apk
sudo vi /etc/croncompose/agent.env
sudo rc-service croncompose-agent start
```

Each package:

- Installs `/usr/local/bin/croncompose-agent`.
- Installs the systemd unit at `/lib/systemd/system/croncompose-agent.service`.
- Drops `/etc/croncompose/agent.env.example` as a config template.
- Postinstall creates the `croncompose` system user and `/var/lib/croncompose`
  (mode 0700) if they don't exist.
- Preremove stops + disables the service.

The unit hardens the runtime: `NoNewPrivileges`, `ProtectSystem=full`,
`ProtectHome=true`, `PrivateTmp=true`.

For a one-off install (no apt/apk available), the existing `scripts/install-agent.sh`
still works.

## Metrics

Prometheus at `/metrics`, outside `/api/v1`. Open by default; set `METRICS_TOKEN` to
require `Authorization: Bearer <token>`.

| Metric | Type | Labels | Reads as |
|---|---|---|---|
| `cc_http_requests_total` | counter | method, path, status | REST traffic. Path is the route template, so cardinality stays bounded. |
| `cc_http_request_duration_seconds` | histogram | method, path | |
| `cc_agents_connected` | gauge | | Agents holding an open stream. A drop here is the first sign of a network problem. |
| `cc_runs_total` | counter | status | |
| `cc_run_duration_seconds` | histogram | status | Bucketed from half a second to two hours. |
| `cc_run_log_bytes_total` | counter | | Bytes received, including bytes the per-run cap dropped. |
| `cc_log_subscribers` | gauge | | Live SSE viewers. |
| `cc_connector_operations_total` | counter | op, status | |
| `cc_notifications_total` | counter | kind, outcome | A rising `failed` here means alerts are not arriving. |
| `cc_retention_deleted_total` | counter | table | Flat after configuring a window means the pruner is not running. |

The two worth alerting on first: `cc_agents_connected` dropping below your fleet size,
and `cc_notifications_total{outcome="failed"}` rising, because that one is the failure
that hides every other failure.

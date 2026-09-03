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

GitHub Releases for CronCompose are **notes-only**: tagging `v*` publishes
`RELEASE_NOTES.md` plus a baked `install-agent.sh` (and a Windows stub). No agent
binaries or `.deb`/`.apk` packages are built in CI.

Install an agent on Linux or macOS (needs `git` and Go 1.25+):

```sh
curl -sSL https://github.com/workvar/cron-compose/releases/latest/download/install-agent.sh | \
  sudo TOKEN=<token> \
       CONTROL_PLANE_HTTP=https://<host>/api/v1 \
       CONTROL_PLANE_ADDR=<host>:9090 \
       bash
```

The script clones the release tag, builds the agent, enrolls it, installs the
service, then deletes the source tree.

The control-plane host (from `install/install.sh`) keeps a git checkout. Settings →
Updates polls GitHub about once a day; **Update** tells the local agent to check out
the tag, rebuild web + control plane + agent via `update.sh`, restart, and strip
build inputs again.

On Linux the installer also writes `/etc/sudoers.d/croncompose-agent` with grants
for `systemctl`, `systemd-analyze`, `ss`, and `lsof` when that file does not already
exist.

## Agent socket inspection (Ports page)

The Ports page asks the agent to list TCP listen sockets and attribute them to systemd
units or pm2 processes. Unprivileged `ss -p` often hides process columns; the agent
falls back to `lsof` and then passwordless `sudo` for `ss` / `lsof` when configured.

Grant the **user that runs the agent** (not necessarily root):

| Install style | Typical agent user | Sudoers file |
|---------------|-------------------|--------------|
| `scripts/install-agent.sh` | `croncompose` | Created automatically |
| Source install (`pm2` / `systemd-setup.sh`) | Your login user (e.g. `pi`) | Add manually |

Example for a Raspberry Pi source install running the agent as `pi`:

```sh
sudo tee /etc/sudoers.d/croncompose-agent <<'EOF'
pi ALL=(root) NOPASSWD: /usr/bin/ss, /usr/bin/lsof
EOF
sudo chmod 0440 /etc/sudoers.d/croncompose-agent
sudo visudo -c
```

Verify:

```sh
sudo -n ss -H -lntp | head
```

Listen lines should include `users:(("process",pid=N,...))`.

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

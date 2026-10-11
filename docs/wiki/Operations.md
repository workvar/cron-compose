# Operations

Day-two extras on top of [Deployment](Deployment.md). Spec: [`docs/operations.md`](https://github.com/workvar/cron-compose/blob/main/docs/operations.md).

## Prometheus metrics

`GET /metrics` on the HTTP port (outside `/api/v1`). Open by default; set `METRICS_TOKEN` to require `Authorization: Bearer <token>`. Firewall it to your monitoring network.

```yaml
- job_name: croncompose
  static_configs:
    - targets: ["control-plane:8080"]
  metrics_path: /metrics
```

| Metric | Type | Reads as |
|--------|------|----------|
| `cc_http_requests_total` | counter | REST traffic (`method`, `path` template, `status`) |
| `cc_http_request_duration_seconds` | histogram | |
| `cc_agents_connected` | gauge | Agents with an open stream. First sign of a network problem |
| `cc_runs_total` | counter | `status` label |
| `cc_run_duration_seconds` | histogram | 0.5s–2h buckets |
| `cc_run_log_bytes_total` | counter | Bytes received, including those the cap dropped |
| `cc_log_subscribers` | gauge | Live SSE viewers |
| `cc_connector_operations_total` | counter | `op`, `status` |
| `cc_notifications_total` | counter | `kind`, `outcome`. Rising `failed` means alerts are not arriving |
| `cc_retention_deleted_total` | counter | `table`. Flat after configuring a window means the pruner is not running |
| `go_*`, `process_*` | | Go runtime |

Alert first on `cc_agents_connected` dropping below fleet size, and `cc_notifications_total{outcome="failed"}` rising — that failure hides every other failure.

## OIDC SSO

The default installer does not ask for SSO. Use **`--advanced`** (or edit `.env` after install):

```sh
./install/install.sh --advanced
```

Put the application **client id** and **secret** in when prompted (or set the env vars below and restart).

| Variable | Purpose |
|----------|---------|
| `OIDC_ISSUER_URL` | e.g. `https://gitlab.com` or `https://login.example.com`. Discovery at startup |
| `OIDC_CLIENT_ID` | Application / client id |
| `OIDC_CLIENT_SECRET` | Application secret. Omit for public clients |
| `OIDC_REDIRECT_URL` | Register `https://<host>/api/v1/auth/oidc/callback` with the IdP. Derived from `PUBLIC_BASE_URL` if unset |
| `OIDC_DEFAULT_ROLE` | First SSO login. Default `viewer` |

GitLab Applications work as OIDC. GitHub OAuth Apps / GitHub Apps do not.

Flow: `/auth/oidc/start` → IdP → `/auth/oidc/callback` (state cookie, code exchange, `id_token`, email+name claims). Missing users are auto-provisioned with an empty password hash (SSO-only). Session cookie is set; browser redirects to `/` or saved `next`.

The login page reads `/auth/config` and shows “Sign in with SSO” when enabled. Password login keeps working.

## Agent packaging

On tag `v*`, CI builds for `linux/amd64` and `linux/arm64`:

```
croncompose-agent-linux-amd64
croncompose-agent-linux-arm64
croncompose-agent_<version>_<arch>.deb
croncompose-agent_<version>_<arch>.apk
SHA256SUMS-<arch>.txt
```

Each package:

- Installs `/usr/local/bin/croncompose-agent`
- Installs systemd unit `/lib/systemd/system/croncompose-agent.service`
- Drops `/etc/croncompose/agent.env.example`
- Postinstall creates `croncompose` system user and `/var/lib/croncompose` (0700)
- Preremove stops + disables the service

The unit hardens the runtime: `NoNewPrivileges`, `ProtectSystem=full`, `ProtectHome=true`, `PrivateTmp=true`.

One-off without apt/apk: `scripts/install-agent.sh` (also writes sudoers for `ss`/`lsof` on Linux when missing).

## Agent socket inspection (Ports page)

Since **v0.0.3**, the Ports page needs the agent to run `ss` or `lsof` with enough privilege to see process IDs on listen sockets. Installers configure this automatically via `install/lib/agent_sudoers.sh`:

| Install | Agent user | Sudoers |
|---------|------------|---------|
| `./install/install.sh` | user running pm2 | prompted during install |
| `sudo ./install/systemd-setup.sh` | service unit user | at setup |
| `scripts/install-agent.sh` | `croncompose` | at install |
| `.deb` / `.apk` | `croncompose` | postinstall |
| `./update.sh` | pm2 user when `CC_ENABLE_AGENT=1` | after restart |

Refresh manually:

```sh
sudo ./install/lib/agent_sudoers.sh <agent-user>
```

Verify: `sudo -n ss -H -lntp | head` should show `users:(("name",pid=...))` on listen lines.

## Process management (source install)

`ecosystem.config.js` is the process list. `croncompose-ctl.sh` wraps pm2:

```sh
./croncompose-ctl.sh status | logs web | restart | stop | start | boot
```

| Process | Starts when |
|---------|-------------|
| `croncompose-control-plane` | always |
| `croncompose-web` | `CC_ENABLE_WEB != 0` and the standalone bundle exists |
| `croncompose-agent` | `CC_ENABLE_AGENT=1` and `$CC_RUNTIME_DIR/agent/identity.json` exists |

Logs: `$CC_RUNTIME_DIR/logs/<process>.{out,err}.log` (default `.run/logs`). Add `pm2-logrotate` for rotation.

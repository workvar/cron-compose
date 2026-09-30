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

The default installer does not ask for SSO. Use **`--advanced`** (or edit `.env` after
install) to put in an identity provider's client id and secret:

```sh
./install/install.sh --advanced
```

When prompted **Configure OIDC single sign-on now?**, enter:

| Variable             | Purpose                                                   |
|----------------------|-----------------------------------------------------------|
| `OIDC_ISSUER_URL`    | e.g. `https://gitlab.com` or `https://login.example.com`. Discovery doc is read at startup. |
| `OIDC_CLIENT_ID`     | Application / client id from the IdP.                     |
| `OIDC_CLIENT_SECRET` | Application secret (omit for public clients).            |
| `OIDC_REDIRECT_URL`  | Register `https://<host>/api/v1/auth/oidc/callback` with the IdP. Derived from `PUBLIC_BASE_URL` if unset. |
| `OIDC_DEFAULT_ROLE`  | role assigned on first SSO login. Default `viewer`.       |

GitLab Applications work as OIDC. GitHub OAuth Apps / GitHub Apps do not (not OIDC).

You can set the same keys in the repo-root `.env` (mode `600`) and restart
(`./croncompose-ctl.sh restart`) instead of re-running the installer.

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
for connector/Ports binaries plus the two `agent-privctl` commands:

```
<agent-user> ALL=(root) NOPASSWD: /usr/libexec/croncompose/agent-privctl elevate, /usr/libexec/croncompose/agent-privctl demote
```

Never grant `NOPASSWD: ALL`. The installer skips the file when a non-managed drop-in
already exists.

## Agent socket inspection (Ports page)

The Ports page asks the agent to list TCP listen sockets and attribute them to systemd
units or pm2 processes. Unprivileged `ss -p` often omits process columns; the agent
falls back to `lsof` and then passwordless `sudo` for `ss` / `lsof` when configured.

**Installers configure this automatically** on Linux:

| Path | What runs |
|------|-----------|
| `./install/install.sh` | `install/lib/agent_sudoers.sh` for the user running pm2 |
| `sudo ./install/systemd-setup.sh` | same for the service unit user |
| `scripts/install-agent.sh` | `croncompose` system user |
| `.deb` / `.apk` postinstall | `croncompose` via `/usr/share/croncompose/agent_sudoers.sh` |
| `./update.sh` | refreshes sudoers when `CC_ENABLE_AGENT=1` |

To install or refresh manually:

```sh
sudo ./install/lib/agent_sudoers.sh <agent-user>
```

The file written is `/etc/sudoers.d/croncompose-agent` (managed marker in the file).
If you already have a custom sudoers drop-in with another name, merge the paths from
`install/lib/agent_sudoers.sh` or remove the conflict.

## Agent root access

An admin/owner can enable **Agent root access** per server after a passkey (WebAuthn)
step-up. The agent then runs `sudo -n /usr/libexec/croncompose/agent-privctl elevate`,
which writes a systemd drop-in `User=root` and restarts `croncompose-agent.service`.
Turning the switch off sends `demote` and restores the original service user.

Agents under pm2 (no systemd) cannot run as root; the helper fails with
`systemd required to run agent as root under pm2`.

Before writing the drop-in, `agent-privctl elevate` checks that the calling agent
lives in the `croncompose-agent.service` cgroup, and after `daemon-reload` that the
unit resolves `User=root`. Either failure is reported to the UI instead of a silent
60s timeout.

**"Agent did not report root after 60s"** — on the host:

```sh
systemctl show -p User,MainPID,DropInPaths croncompose-agent   # User=root, MainPID = the agent
pgrep -af 'agent run|croncompose-agent'                        # no pm2/nohup copy
ls /etc/systemd/system/croncompose-agent.service.d/            # root.conf present
sudo -u <agent-user> sudo -n -l | grep agent-privctl           # grant present
journalctl -u croncompose-agent -n 100
```

The packaged unit uses `ProtectSystem=full`; it needs
`ReadWritePaths=-/etc/systemd/system/croncompose-agent.service.d` (shipped in the
unit; add it with `systemctl edit` on older installs) or the helper cannot write
the drop-in. Manual enable/disable steps are on the wiki page
[Agent root access](https://github.com/workvar/cron-compose/wiki/Agent-Root-Access).

### Installing or reinstalling as root

There are two ways to get a root agent. The toggle above needs a connected agent and
the sudoers grant. The **root install command** (`AGENT_RUN_AS_ROOT=1`, shown when
adding a server and as **Reinstall as root** on an existing server's page) skips the
toggle: the agent runs as root from the start.

The Linux installer replaces a running agent instead of leaving it alone. It stops
`croncompose-agent.service` before re-enrolling, restarts it afterwards, and prints who
the agent runs as:

```text
==> agent pid 4242 runs as uid 0
```

`systemctl enable --now` on its own never restarts a unit that is already active, so
without the restart the old process kept running under its old user and the server
page kept offering **Reinstall as root**. The installer now warns when the running
process does not match what was asked for, and when another agent process that
systemd does not manage (pm2, nohup) is running.

The enrolling process runs as root exactly when the root install command was used, and
it tells the control plane (`run_as_root` in the enroll request). The control plane
stores that as the desired Agent root access flag, so a root install shows **On (root)**
instead of **Off** followed by a false "still running as root after demote" error.
Re-enrolling with the dedicated-user command clears the flag. Agents that predate
`run_as_root` leave the flag unchanged.

**"Reinstall as root" still shows after reinstalling.** The panel shows while the
control plane last heard the agent report non-root. On the host:

```sh
systemctl show -p User,MainPID croncompose-agent
ps -o user,pid,cmd -p "$(systemctl show -p MainPID --value croncompose-agent)"
pgrep -af 'croncompose-agent run'      # exactly one process, the MainPID
```

The MainPID process must run as root. If it does not, `sudo systemctl restart
croncompose-agent` makes it pick up the current unit.

### Toggle changed while the agent was offline

The desired flag is saved before the command is sent. If the agent was offline or
reconnecting at that moment the command was lost, and the switch sat on **On (waiting)**
until the 60s timeout. When the agent next connects, the control plane re-sends the
command if the agent's reported privileges still disagree with the flag.

It re-sends once per desired value. A host that can never elevate (pm2, missing
sudoers grant) reports its error instead of restarting in a loop; flip the switch
again to retry. The record is in memory, so a control plane restart allows one more
re-send.

WebAuthn relying-party ID is the hostname of `PUBLIC_BASE_URL` (or `PUBLIC_HTTP_URL`).
Set it to the URL operators open in the browser.

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

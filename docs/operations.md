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
       AGENT_ENROLL_HTTP=https://<host>/api/v1 \
       AGENT_GRPC_ADDR=<host>:9090 \
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

### Agent variable names

The agent reads `AGENT_GRPC_ADDR`, `AGENT_GRPC_SNI` and `AGENT_ENROLL_HTTP`, which cannot
be mistaken for the control plane's own `GRPC_ADDR` / `PUBLIC_GRPC_ADDR` when both share
one `.env`. The old `CONTROL_PLANE_ADDR`, `_SNI` and `_HTTP` names still work as a
fallback, so installed agents keep running. If both spellings are set to different
values the new name wins, and the agent logs a warning naming the ignored one. The
install command shown in the UI still uses the old names; the installer accepts both.

### Agents behind Cloudflare (edge mode)

Cloudflare's proxy ends TLS, so an agent cannot present its client certificate through
it, and plain gRPC to `grpc.example.com` fails with "certificate signed by unknown
authority". Edge mode lets clients install and connect with nothing extra:

- The agent dials the hostname on 443 over ordinary TLS and verifies Cloudflare's public
  certificate. It proves who it is with a per-server secret that the control plane gave
  it at enrollment (saved in `identity.json`, mode 0600; only a SHA-256 is stored on the
  server).
- The control plane runs a second gRPC listener for this, plain HTTP/2 on loopback.
  cloudflared only speaks HTTP/2 to an `https://` origin, so `install.sh` and
  `update.sh` install nginx on that same loopback address. nginx presents the
  control plane certificate and forwards gRPC to the plain listener.

You do not edit `.env`. `install.sh` and `update.sh` check whether the gRPC hostname
answers with Cloudflare's headers. A Tunnel TCP route (`tcp://localhost:9077`) does
not speak HTTPS, so that probe fails even though Cloudflare is in front; they then
check the public URL (`PUBLIC_BASE_URL`) the same way. When the gRPC hostname is
Cloudflare, or that probe fails and the public URL is Cloudflare, they write
`PUBLIC_GRPC_MODE=edge`, `EDGE_GRPC_ADDR=127.0.0.1:<free port>` and a bare
`PUBLIC_GRPC_ADDR=<hostname>`. A previously saved `PUBLIC_GRPC_MODE=mtls` is upgraded
in that case, because it is what a failed probe used to leave behind. `update.sh`
then restarts the stack. Set `CC_GRPC_MODE=edge` or `mtls` when running either script
to override the detection for that run. Docker Compose installs are not covered; set
the three variables in the compose environment yourself.

The one step the scripts cannot do is in Cloudflare, and they print it: turn on
**Network, gRPC** for the zone, and point the hostname's tunnel route at
`https://localhost:<nginx port>` (`EDGE_NGINX_PORT` in `.env`). Under additional
application settings, TLS, turn **Use HTTP/2 to origin** on and **Disable TLS
certificate verification** on. A hostname has one route, so this replaces an
`http://` or `tcp://localhost:9077` route on the same name. "Use HTTP/2 to origin"
does nothing while the URL is `http://`, which is why the origin is nginx rather
than the plain listener. The direct `GRPC_ADDR` listener stays mutual TLS and is
unchanged.

Then create or reinstall servers as usual. The install command carries the bare hostname,
and the enrolled agent records `edge` as its mode (`AGENT_GRPC_MODE` overrides it). An
agent enrolled before this feature has no secret and must be reinstalled with a new token.

What you give up: Cloudflare can read the traffic between the agent and the control
plane, and a stolen secret logs in as that server until it is replaced by re-enrolling.
The edge listener refuses to bind anything but loopback, since it has no TLS of its own.

### GRPC hostname without a port

If the gRPC hostname reaches the gRPC port on 443 through a plain TCP forward (a load
balancer or Cloudflare Spectrum, say), give the agent just the hostname:
`AGENT_GRPC_ADDR=grpc.example.com`. A bare host means port 443. `host:port` still works
and is needed when the endpoint is not on 443. Agents older than v0.0.25 need the
explicit `:443`.

A Cloudflare Tunnel application route (`tcp://localhost:9077` on the tunnel) is not that
case. It is not plain TCP on 443: remote agents must run
`cloudflared access tcp --hostname grpc.example.com --url 127.0.0.1:<local port>` and use
`AGENT_GRPC_ADDR=127.0.0.1:<local port>` with `AGENT_GRPC_SNI=grpc.example.com`.

### Explicit address beats the enrolled address

At enrollment the control plane hands the agent its public gRPC address, and the agent
saves it in `identity.json`. An `AGENT_GRPC_ADDR` (or old `CONTROL_PLANE_ADDR`) that you
set explicitly now takes precedence over it, so a host that reaches the control plane
differently, such as loopback on the control plane host, can say so. Before v0.0.26 the
saved address always won and the variable was ignored. On those versions, edit
`control_plane_grpc_addr` in `identity.json` and restart the agent.

### Agent on the same host as the control plane

The install command bakes the public gRPC address into the unit. Behind Cloudflare's
proxy that address cannot carry raw gRPC, so an agent on the control plane host
never connects and the page keeps its old privileges. After starting, the installer
warns when the address cannot be reached, when a systemd drop-in overrides
`AGENT_GRPC_ADDR`, and when another agent process (any path) is running. Fix it
with `AGENT_GRPC_ADDR=127.0.0.1:<grpc port>` and `AGENT_GRPC_SNI=<cert host>`
in the install command, and stop any hand-started agent so only one runs.

### Toggle changed while the agent was offline

The desired flag is saved before the command is sent. If the agent was offline or
reconnecting at that moment the command was lost, and the switch sat on **On (waiting)**
until the 60s timeout. When the agent next connects, the control plane re-sends the
command if the agent's reported privileges still disagree with the flag.

It re-sends once per desired value. A host that can never elevate (pm2, missing
sudoers grant) reports its error instead of restarting in a loop; flip the switch
again to retry. The record is in memory, so a control plane restart allows one more
re-send.

A turned-off flag is only re-sent as a demote when an operator switched it off.
Agents installed as root before this behaviour existed (and macOS root agents) have a
flag nobody set, and are left running as root.

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

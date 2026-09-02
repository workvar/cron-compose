# Deployment

How to deploy the CronCompose control plane and agents to production. Two paths are
supported:

- **Docker Compose** ([docker-compose.prod.yml](docker-compose.prod.yml)): the
  recommended way to run the control plane, web UI, and Postgres on one host or
  orchestrator.
- **From source** ([install/install.sh](install/install.sh)): an interactive installer
  that builds everything and supervises it with pm2 (`croncompose-ctl.sh` wraps it). See
  [install/README.md](install/README.md) for the full reference.

For deeper operational topics this guide links out rather than repeating:
[operations.md](docs/operations.md) (metrics, OIDC, agent packaging),
[upgrades.md](docs/upgrades.md) (per-migration detail), and
[security.md](docs/security.md) (auth, mTLS, secrets, sandbox).

## Topology

The **control plane is the single entry point**. It exposes two ports:

```
                       :8080  (HTTP, public)        :9090  (gRPC mTLS, public)
  browsers / REST ───────────►┐                          ▲
                              │  control plane            │
                  /app  ──────┼──► web UI (Next.js, internal :3000)
                  /api  ──────┘    REST API (/api -> /api/v1)
                                                          │
  agents ─────────────────────────────────────────────────┘  (dial out only)

                          control plane ──► Postgres (internal)
```

- **HTTP port** (`HTTP_ADDR`, default `:8080`) serves the REST API under `/api` and
  reverse-proxies `/app` to the internal Next.js server; the bare `/` redirects to
  `/app`. The control plane reaches the UI via `WEB_UPSTREAM`.
- **gRPC port** (`GRPC_ADDR`, default `:9090`) is the agent mTLS endpoint. Agents dial
  it directly; they never accept inbound connections.
- **Web UI** runs internally and is only reachable through the control plane, so it
  binds loopback (source install) or stays on the internal network (Compose).
- **Postgres** is the only stateful dependency.

There is no separate single-port proxy. Browser TLS is terminated in front of the HTTP
port (see [TLS and PKI](#tls-and-pki)); the gRPC port carries mTLS end to end.

## Prerequisites

- A **public DNS name** that both browsers and agents use to reach the host (for
  example `cc.example.com`).
- **PostgreSQL 16** (managed, or via Compose).
- A **TLS certificate** for the public name, terminated by a reverse proxy or load
  balancer in front of the HTTP port.
- Docker + Compose v2 (Compose path), or Go 1.25+, Node 20+, and npm (source path).

## Configuration

The control plane reads its configuration from the environment. On a from-source host
that environment is a `0600` `.env` at the repo root, written by `./install/install.sh`
and read by `ecosystem.config.js` (and so by `croncompose-ctl.sh`). For a manual install,
or to look up a key, start from the annotated template:

```sh
cp .env.example .env && chmod 600 .env
```

Edit `.env` in place afterwards; it is the source of truth for the stack, and both
process managers cache the environment, so restart after any change. Under Docker
Compose the same values come from `docker-compose.prod.yml`'s `x-env` block or a `.env`
beside it. The most important variables:

| Variable             | Purpose                                                                 |
|----------------------|-------------------------------------------------------------------------|
| `DATABASE_URL`       | Postgres DSN. Required.                                                  |
| `HTTP_ADDR`          | HTTP listener. Default `:8080`. Serves `/app` and `/api`.               |
| `GRPC_ADDR`          | Agent mTLS gRPC listener. Default `:9090`.                              |
| `WEB_UPSTREAM`       | Internal Next.js address the control plane proxies `/app` to. Empty disables the UI proxy (API-only). |
| `PUBLIC_BASE_URL`    | Single source of truth for the external address. Derives the public REST URL, the advertised gRPC address, the OIDC redirect, and a TLS SAN. |
| `SESSION_SECRET`     | HMAC key for session cookies. Required, 16+ chars. Generate with `openssl rand -hex 32`. |
| `SECRETS_MASTER_KEY` | 32-byte hex key (64 hex chars) wrapping stored secrets. **Set this in prod**; the default is a clearly marked dev key. |
| `SEED_ADMIN_EMAIL` / `SEED_ADMIN_PASSWORD` | Bootstrap admin, upserted on every boot (see the hardening note). |
| `TLS_DIR`            | Where the agent-facing CA and server cert live. Persist this.            |
| `TLS_HOSTS`          | SANs the server cert covers. Must include the public name agents dial.   |
| `LOG_LEVEL`          | `debug` \| `info` \| `warn` \| `error`. Default `info`.                  |
| `OIDC_*`             | Optional SSO. See [operations.md](docs/operations.md).                  |
| `METRICS_TOKEN`      | When set, `/metrics` requires `Authorization: Bearer <token>`. Empty leaves it open, which is fine on a private network and not on the internet. |
| `RUN_LOG_MAX_BYTES`  | Per-run log storage cap. Default 5 MiB. The run still completes and its exit status is still reported; only the log tail is dropped, with a note in the log. |
| `RETENTION_RUN_LOG_DAYS` | Delete run logs older than this. `0` (default) never prunes. |
| `RETENTION_RUN_DAYS` | Delete runs older than this. `0` (default) never prunes. Set it longer than the log window: runs are small and answer "has this been failing all month". |
| `RETENTION_AUDIT_DAYS` | Delete audit entries older than this. `0` (default) never prunes. |
| `RETENTION_OPERATION_DAYS` | Delete connector operations older than this. `0` (default) never prunes. |
| `AGENT_UPDATE_VERSION` | Target agent version. Agents reporting a different version on connect are offered an update. Empty (default) disables agent self-update entirely. |
| `AGENT_UPDATE_URL`   | Download URL template. `{version}`, `{os}` and `{arch}` are substituted. Must be `https`. |
| `AGENT_UPDATE_SHA256` | Pinned sha256 of the binary: a bare hex digest, or a JSON object mapping `"os/arch"` to a digest. **Required** for updates to be offered; there is no unverified path. |
| `AGENT_UPDATE_RESTART` | `1` (default) tells the agent to exit after swapping so its supervisor restarts it. `0` leaves the new binary to take effect on the next restart. |
| `GITHUB_RELEASE_REPO` | GitHub `owner/repo` to poll for agent releases (default `workvar/cron-compose`). Disabled when a manual `AGENT_UPDATE_*` policy is configured. |
| `AGENT_UPDATE_POLL_MINUTES` | How often to check GitHub for a new tag (default `15`). |

`PUBLIC_BASE_URL` is the easiest knob: set it once (for example
`https://cc.example.com`) and the control plane derives `PUBLIC_HTTP_URL`
(`<base>/api/v1`), `PUBLIC_GRPC_ADDR` (`<host>:<gRPC port>`), the OIDC redirect, and
adds the host to `TLS_HOSTS`. The Compose file instead sets `PUBLIC_HTTP_URL` and
`PUBLIC_GRPC_ADDR` explicitly; either approach works.

Generate the two secrets before first boot:

```sh
openssl rand -hex 32   # SESSION_SECRET
openssl rand -hex 32   # SECRETS_MASTER_KEY
```

Back up `SECRETS_MASTER_KEY` somewhere safe. Losing it makes every stored secret
unrecoverable.

## Path A: Docker Compose

[docker-compose.prod.yml](docker-compose.prod.yml) runs Postgres, the control plane,
and the internal web UI. The control plane publishes the two public ports; the web
container has no published ports.

1. **Create `.env`** next to the compose file. The required keys fail fast if unset:

   ```sh
   SESSION_SECRET=<openssl rand -hex 32>
   SECRETS_MASTER_KEY=<openssl rand -hex 32>
   SEED_ADMIN_EMAIL=you@example.com
   SEED_ADMIN_PASSWORD=<strong password>

   # Public address (used in the agent install command and advertised to agents).
   PUBLIC_HTTP_URL=https://cc.example.com/api/v1
   PUBLIC_GRPC_ADDR=cc.example.com:9090
   TLS_HOSTS=localhost,control-plane,cc.example.com

   # Published host ports (optional; these are the defaults).
   PUBLIC_PORT=8080
   PUBLIC_GRPC_PORT=9090
   ```

2. **Build and run migrations** (the control-plane image bundles the migrate tool and
   the SQL files):

   ```sh
   docker compose -f docker-compose.prod.yml build
   docker compose -f docker-compose.prod.yml up -d postgres
   docker compose -f docker-compose.prod.yml run --rm --no-deps \
     --entrypoint /usr/local/bin/migrate control-plane -dir /migrations
   ```

3. **Start the stack:**

   ```sh
   docker compose -f docker-compose.prod.yml up -d
   ```

The UI is then at `https://cc.example.com/app` (behind your TLS terminator, see below),
the REST API at `/api`, and agents enroll against `cc.example.com:9090`.

`./update.sh` automates rebuild, migrate, and restart in Compose mode (it auto-detects
the compose file).

## Path B: From source on a single host

Use this for a Raspberry Pi, a VM, or any single box without containers.

```sh
git clone <repo> && cd croncompose
./install/install.sh
```

The installer asks four questions (public URL, HTTP port, database, admin login) and
derives the rest; `--advanced` asks the long form. It generates secrets, builds the binaries
and the UI, applies migrations, and writes a `0600` `.env`. Everything runs under
**pm2** (installed automatically if missing), defined by `ecosystem.config.js`.
`croncompose-ctl.sh` is a thin wrapper over it:

```sh
./croncompose-ctl.sh status     # what's running
./croncompose-ctl.sh logs web   # tail a log (control-plane | web | agent)
./croncompose-ctl.sh restart    # restart everything, re-reading .env
./croncompose-ctl.sh boot       # survive reboots (pm2 startup + save)
```

Plain pm2 commands work too: `pm2 status`, `pm2 monit`, `pm2 logs croncompose-web`.

Pull updates and roll forward with `./update.sh` (source mode). Remove the install with
`./uninstall.sh` (`--dry-run` to preview, `--keep-db` to spare the database). Full flag
and environment reference: [install/README.md](install/README.md).

## Path C: pm2 without the installer

`ecosystem.config.js` is the single source of truth for the process list, with restart
policies, log rotation and boot persistence. It reads the repo-root `.env`.

```sh
npm install -g pm2
make pm2-start                  # start control-plane (+ web, + agent if enrolled)
make pm2-save                   # persist the process list
pm2 startup                     # print the boot command; run it once as instructed
```

Only building, not installing? The config needs three things: `.env` at the repo root,
`control-plane/bin/control-plane`, and `web/.next/standalone/server.js`.

```sh
cp .env.example .env && chmod 600 .env   # then fill in the secrets and DATABASE_URL
make control-plane migrate-tool
cd web && npm ci && API_BASE=http://127.0.0.1:8080/api/v1 npm run build && cd ..
cp -r web/.next/static web/.next/standalone/.next/static
```

`API_BASE` is inlined at build time by `next.config.ts`, so it must be set for the build,
not just at runtime.

### Which processes start

| Process | Condition |
| --- | --- |
| `croncompose-control-plane` | always |
| `croncompose-web` | `CC_ENABLE_WEB != 0` and the standalone bundle exists |
| `croncompose-agent` | `CC_ENABLE_AGENT=1` and `$CC_RUNTIME_DIR/agent/identity.json` exists |

Enroll the local agent first (the installer does this, or use the UI plus
`agent enroll --token=...`), then `make pm2-restart` to pick it up.

### Day two

```sh
make pm2-status
make pm2-logs                   # or: pm2 logs croncompose-control-plane
make pm2-restart                # re-reads .env (--update-env)
make pm2-delete                 # remove from pm2
```

Logs go to `$CC_RUNTIME_DIR/logs/<process>.{out,err}.log` (default `.run/logs`). Add
rotation with `pm2 install pm2-logrotate`.

After `./update.sh` or a manual rebuild, run `make pm2-restart`. Changing a value in
`.env` also requires a restart, since pm2 caches the environment.

## TLS and PKI

There are two distinct TLS surfaces; treat them differently.

**Browser and REST (HTTP port).** The control plane speaks cleartext HTTP on
`HTTP_ADDR`. For production, put a TLS-terminating reverse proxy or load balancer
(nginx, Caddy, a cloud LB) in front and forward to the control plane's HTTP port. A
minimal nginx example:

```nginx
server {
  listen 443 ssl;
  server_name cc.example.com;
  ssl_certificate     /etc/ssl/cc.example.com.crt;
  ssl_certificate_key /etc/ssl/cc.example.com.key;

  location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $remote_addr;
    proxy_set_header X-Forwarded-Proto $scheme;
  }
}
```

**Agents (gRPC port).** The control plane runs its own CA, auto-created under `TLS_DIR`,
and presents a server certificate whose SAN must include the public name agents dial
(`TLS_HOSTS`, or the `PUBLIC_BASE_URL` host). Agent mTLS is end to end, so **do not
terminate TLS on the gRPC port**. If a load balancer sits in front, use L4/TCP
passthrough, not HTTP termination.

Operational notes:

- **Persist `TLS_DIR`** (a Docker volume `cp-tls` in Compose). If the CA is regenerated,
  enrolled agents no longer trust the server and must re-enroll.
- **Changing the public name:** update `PUBLIC_BASE_URL` (or `PUBLIC_HTTP_URL` +
  `PUBLIC_GRPC_ADDR`) and `TLS_HOSTS`, then delete the contents of `TLS_DIR` to
  regenerate a cert whose SAN covers the new name, and restart.

See [security.md](docs/security.md) for the trust model, enrollment, and secret
handling in full.

## Database and migrations

Provide a reachable `DATABASE_URL` (PostgreSQL 16). Migrations live in
[migrations/](migrations) and are applied by the bundled `migrate` tool, which records
applied files in a `schema_migrations` table and is safe to re-run.

- **Compose:** `docker compose -f docker-compose.prod.yml run --rm --no-deps
  --entrypoint /usr/local/bin/migrate control-plane -dir /migrations`.
- **Source:** `control-plane/bin/migrate -dir migrations` (reads `$DATABASE_URL`).

Always **migrate before** starting or upgrading the control plane: it reads columns that
older schemas do not have. Take a `pg_dump -Fc` backup first. Per-migration detail and
the roll-forward sequence are in [upgrades.md](docs/upgrades.md).

## Agents

Agents run jobs on your servers and only ever dial out, so target hosts need no inbound
ports for CronCompose. Install on Linux/macOS via the `.deb` / `.apk` packages or
`scripts/install-agent.sh` (see [operations.md](docs/operations.md)), then configure
`/etc/croncompose/agent.env`:

```sh
CONTROL_PLANE_ADDR=cc.example.com:9090        # mTLS gRPC endpoint
CONTROL_PLANE_HTTP=https://cc.example.com/api/v1   # one-time enroll call
CONTROL_PLANE_SNI=cc.example.com              # server name verified in TLS
DATA_DIR=/var/lib/croncompose                 # identity, certs, job cache, outbox
```

Enrollment: create a server in the UI to mint a one-time token, run `agent enroll
--token=...`, and the agent exchanges it once for a client certificate and switches to
mTLS. Run the agent as a dedicated unprivileged user, never root, unless a specific
job's `run_as_user` requires it.

## Production hardening checklist

- [ ] **Secrets set and backed up.** `SESSION_SECRET` and `SECRETS_MASTER_KEY` are real
  random 32-byte hex values, not the dev defaults. `SECRETS_MASTER_KEY` is backed up
  offline.
- [ ] **TLS in front of HTTP**, with the gRPC port on TCP passthrough (never HTTP
  terminated).
- [ ] **Admin credential strategy.** `SEED_ADMIN_*` re-applies the admin password and
  `owner` role on every boot, so a password changed in the UI is overwritten on restart.
  Either keep `SEED_ADMIN_*` as the managed source of truth (rotate by editing env and
  restarting) or unset them after first login so UI-managed credentials persist.
- [ ] **`/metrics` is unauthenticated.** Firewall it to your monitoring network.
- [ ] **Persistent volumes** for Postgres and `TLS_DIR`, both backed up.
- [ ] **Agents run as a non-root system user**, with the hardened systemd unit from the
  packages.
- [ ] **`LOG_LEVEL=info`** (or `warn`) in production.

## Health and observability

- `GET /healthz` returns liveness plus a Postgres ping (use it for container and LB
  health checks).
- `GET /metrics` exposes Prometheus metrics (request counts and latency, connected
  agents, run totals, log subscribers). Scrape config and the full metric list are in
  [operations.md](docs/operations.md).

## Upgrades

The safe sequence is: back up the database, apply migrations, roll the control plane
forward, then the web UI; agents reconnect on their own. `./update.sh` performs this for
both Compose and source installs (it auto-detects the mode). See
[upgrades.md](docs/upgrades.md).

## Retention and log volume

Two independent controls, because they solve different problems.

**The per-run cap** (`RUN_LOG_MAX_BYTES`, 5 MiB default) stops one runaway job from
filling the database on its own. It bounds stored bytes, not the job: the script keeps
running, its exit status is reported truthfully, and a single line goes into the log
saying the rest was not stored. Live SSE subscribers still see everything, so somebody
watching a run does not have the stream cut off under them.

**The retention pruner** bounds total history and is off by default, because silently
deleting a user's history is not a thing to start doing unasked. Once any window is set
it sweeps hourly, deleting in 5000-row batches so the statements stay short enough that
normal traffic does not notice. A sensible starting point:

    RETENTION_RUN_LOG_DAYS=14
    RETENTION_RUN_DAYS=90
    RETENTION_AUDIT_DAYS=365
    RETENTION_OPERATION_DAYS=365

Logs are the bulk of the data and are read within hours. Runs are small and are what
trend questions are answered from, so they should outlive their logs. Audit entries are
tiny and are the thing you most regret having deleted.

Watch `cc_retention_deleted_total` to confirm the pruner is doing something, and
`cc_run_log_bytes_total` against the database size to see the cap working.

## Agent self-update

Off unless all three of `AGENT_UPDATE_VERSION`, `AGENT_UPDATE_URL` and
`AGENT_UPDATE_SHA256` are set. There is deliberately no unverified path: downloading a
binary and running it as whatever the agent runs as is the most dangerous thing this
system does, and the pinned checksum is what makes it a controlled action rather than a
remote code execution primitive.

    AGENT_UPDATE_VERSION=1.4.0
    AGENT_UPDATE_URL=https://dl.example.com/croncompose/{version}/agent-{os}-{arch}
    AGENT_UPDATE_SHA256='{"linux/amd64":"<hex>","linux/arm64":"<hex>"}'

The offer is made when an agent says hello with a different version. The agent verifies
the digest as it streams the download, keeps the old binary as `<name>.old`, swaps
atomically, and exits 0 so its supervisor restarts it. On a box where nothing would
bring the agent back, set `AGENT_SELF_UPDATE=0` on the agent and update it by hand.

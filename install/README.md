# CronCompose installer

Interactive, from-source installers that stand up the whole CronCompose **control
plane** (PostgreSQL + the Go API + the Next.js web UI) on one machine, and on
Linux/macOS optionally enroll a local **agent** so the box is also a job runner.

- **Linux / macOS:** `install/install.sh`
- **Windows:** `install/install.ps1` (control plane only: see [Agent](#agent))

The installer asks four questions (URL, port, database, admin login), generates the
secrets for you, builds everything, creates the database schema, starts the services
under pm2, and prints how to sign in. `--advanced` asks the long form.

The **control plane is the single entry point**: its HTTP port serves the REST API under
`/api` and reverse-proxies `/app` to the web UI, and the bare URL redirects to `/app`.
Agents connect to the control plane's mTLS gRPC port directly. The web UI runs internally
on loopback and is reached through the control plane.

## Prerequisites

You need a working toolchain because this builds from source:

- **Go 1.25+**: builds the API, the migration tool, the agent, and the `cc` CLI.
- **Node 20+** and **npm**: builds and runs the web UI.
- **PostgreSQL**: optional to install yourself. If one is already running locally the
  installer offers to use it; otherwise it can **install and configure it for you** via
  your OS package manager (apt, dnf/yum, pacman, apk, zypper, or Homebrew), or you can
  paste a connection string for a server elsewhere. Docker Postgres stays available
  behind `--advanced`.

The installer checks all of this up front and stops with a clear message if something
is missing.

## Quick start

Linux / macOS:

```sh
git clone <repo> && cd croncompose
./install/install.sh
```

Windows (PowerShell):

```powershell
git clone <repo>; cd croncompose
./install/install.ps1
```

Answer the prompts and you'll end up with the UI on the control plane's HTTP port you
chose (open `http://<host>:<http port>`; it redirects to `/app`).

## What it asks you

Four questions:

1. **Public URL**: where people will reach CronCompose. A hostname, an IP, or a full
   URL (`https://cron.example.com`). When you give a scheme, the printed URLs omit the
   listener port, on the assumption that a TLS proxy fronts it; add an explicit port
   (`https://cron.example.com:8443`) to keep one.
2. **HTTP port**: the single public port, serving the UI at `/app` and the REST API at
   `/api`. A free port is suggested and occupied ones are flagged. The agent gRPC port
   and the internal web UI port are chosen automatically from the first free port.
3. **Database**: the installer looks for a PostgreSQL already running on this machine
   and offers to use it (creating a `croncompose` database and role in it, with your
   confirmation). If there is none, it offers to install PostgreSQL for you, and failing
   that asks for a connection string (see [Database options](#database-options)).
4. **Admin account**: the email and password you'll sign in with. Leave the password
   blank and one is generated and shown to you.

Everything else is derived: the runtime directory (`./.run`), the gRPC and web ports,
the log level, and the TLS SANs. `SESSION_SECRET` and `SECRETS_MASTER_KEY` are generated
with a CSPRNG, or **reused from an existing `.env`** so re-running the installer never
orphans your stored secrets. It all lands in a `.env` (mode `600`) at the repo root.

Run `./install/install.sh --advanced` to be asked the long form instead: runtime
directory, all three ports, the database method menu, log level, OIDC SSO, and free-form
`KEY=VALUE` extras. Every one of those is also settable non-interactively through a
`CC_*` environment variable.

## Database options

The default path is two steps only:

1. **Local PostgreSQL** (checked first): probes `127.0.0.1` / `localhost` on
   `CC_DB_PORT`, `5432` and `5433`. If a server answers, one yes/no asks whether to use
   it. On yes, the installer creates the `croncompose` database and owning role with a
   generated password (override via `CC_DB_NAME` / `CC_DB_USER` / `CC_DB_PASS`), using
   your OS user as superuser on macOS or `postgres` on Linux. A superuser password is
   prompted later only if the server requires one. Migrations run later in the same
   install. Needs `psql` on PATH; without it the installer says so and falls through.
2. **Remote connection string**: if nothing is listening, you decline the local offer,
   or `psql` is missing, you paste a `DATABASE_URL` for a database that already exists.
   Nothing is created; migrations are applied to whatever you point at. A quick
   reachability check surfaces typos immediately.

**Install via package manager** and **Docker** are not offered on the default path. Use
`./install/install.sh --advanced` (menu items 3 and 4), or set `CC_DB_METHOD=native` /
`CC_DB_METHOD=docker`. Native install supports `apt`, `dnf`, `yum`, `pacman`, `apk`,
`zypper`, and Homebrew (`brew`); preview with `CC_DB_DRY_RUN=1`. Docker starts Postgres
from `docker-compose.yml` and waits until it is ready. (Running *CronCompose itself* in
Docker is separate: see `docker-compose.prod.yml` and `update.sh --mode compose`.)

Migrations are applied by a small bundled tool (`control-plane/cmd/migrate`) that talks
to Postgres directly, so **no `psql` client is required** on Windows or macOS. It records
applied files in a `schema_migrations` table, so re-running the installer is safe.

## Uninstalling

`./uninstall.sh` removes everything the installer put on the machine, wherever it
landed: pm2 processes and their boot entry, systemd units, the runtime directory, the
control script, built artifacts, `.env`, and the database. It asks before destroying
data, and leaves the checkout, PostgreSQL, Node and pm2 in place.

```sh
./uninstall.sh              # interactive; type the database name to confirm the drop
./uninstall.sh --dry-run    # show what would be removed, touch nothing
./uninstall.sh --keep-db    # everything except the database
./uninstall.sh --yes        # no prompts, database included
```

## Managing the stack

Services run under [pm2](https://pm2.keymetrics.io/), defined by `ecosystem.config.js`
at the repo root. The installer installs pm2 if it is missing and generates
`croncompose-ctl.sh` next to `.env` as a thin wrapper, so there is one command you keep
using afterwards:

```sh
./croncompose-ctl.sh status       # what's running
./croncompose-ctl.sh logs web     # tail a service log (control-plane | web | agent)
./croncompose-ctl.sh restart      # restart everything, re-reading .env
./croncompose-ctl.sh stop         # stop everything
./croncompose-ctl.sh start        # start (or resume) everything
./croncompose-ctl.sh boot         # pm2 startup + save, so it comes back after a reboot
```

pm2's own commands work unchanged: `pm2 status`, `pm2 monit`, `pm2 logs croncompose-web`.

On Windows the equivalent is `./croncompose-ctl.ps1 <action>` (Windows still uses the
built-in process management, not pm2).

## Changing the external address (single point of change)

Browsers and the REST API reach the box on the control plane's HTTP port; agents reach
it on the gRPC port. Use any hostname that reaches the box (`localhost`,
`raspberrypi.local`, your domain).

The address matters where the control plane advertises *itself* (the agent install
command, the gRPC address agents dial, the OIDC redirect, the TLS SAN). The installer
writes one `.env` line that drives all of them:

```
PUBLIC_BASE_URL=http://raspberrypi.local:8080    # derives REST URL, gRPC addr, OIDC redirect, TLS SAN
```

`PUBLIC_GRPC_ADDR` derives from this host plus the gRPC port, so you don't set it
separately. To move to another address (for example `https://cron.example.com`), change
the host/port and restart (`./croncompose-ctl.sh restart`); add the new host to
`TLS_HOSTS`, or delete `<runtime>/tls` to regenerate the server cert so its SAN covers it.

## Agent

The per-server agent runs scheduled jobs through a Unix shell and uses Unix process
APIs, so it builds and runs on **Linux and macOS only**. The Linux/macOS installer can
enroll and start a local agent for you (it logs in with the seeded admin, mints a
one-time token, enrolls, and starts the agent). On Windows the installer sets up the
control plane only; run agents on Linux/macOS hosts and point them at this control
plane (see `scripts/install-agent.sh`).

On Linux, installers also write `/etc/sudoers.d/croncompose-agent` so the agent can
inspect listen sockets (Ports page) and drive connectors (`systemctl`, `nginx`, …).
The helper is `install/lib/agent_sudoers.sh`; run it manually to refresh:

```sh
sudo ./install/lib/agent_sudoers.sh "$(whoami)"    # source install (pm2 user)
sudo ./install/lib/agent_sudoers.sh croncompose    # standalone agent package
```

`./update.sh` re-applies this when `CC_ENABLE_AGENT=1`.

## Non-interactive install

Pass `--non-interactive` (PowerShell: `-NonInteractive`) to take defaults without
prompting. Values come from `CC_*` environment variables:

| Variable             | Meaning                                            | Default                          |
|----------------------|----------------------------------------------------|----------------------------------|
| `CC_RUNTIME_DIR`     | runtime state directory                            | `./.run`                         |
| `CC_ADVERTISE_HOST`  | host used to build the default `PUBLIC_BASE_URL`   | `localhost`                      |
| `CC_PUBLIC_BASE_URL` | external base URL written to `.env` (overrides the host-derived default) | `http://<advertise>:<http port>` |
| `CC_WEB_PORT`        | web UI port (internal; reached via the control plane) | first free at/after `3000`    |
| `CC_API_PORT`        | REST API port                                      | first free at/after `8080`       |
| `CC_GRPC_PORT`       | agent gRPC port                                    | first free at/after `9090`       |
| `CC_ADMIN_EMAIL`     | seed admin email                                   | `admin@example.com`              |
| `CC_ADMIN_PASSWORD`  | seed admin password                                | generated if empty               |
| `CC_DB_METHOD`       | `psql` \| `native` \| `existing` \| `docker`        | default path: `psql` if local Postgres is accepted, else `existing`; `native`/`docker` only via `--advanced` or this env |
| `CC_DATABASE_URL`    | DSN (for `existing`)                               | local dev DSN                    |
| `CC_DB_NAME` / `CC_DB_USER` / `CC_DB_PASS` | database, role, password (for `native`/`psql`) | `croncompose` / `croncompose` / generated |
| `CC_LOG_LEVEL`       | `debug` \| `info` \| `warn` \| `error`             | `info`                           |

Other flags: `--no-web` / `-NoWeb` (API-only), `--no-agent` (Linux/macOS, control plane
only), `--runtime-dir DIR` / `-RuntimeDir DIR`.

Example: a headless, scripted install against an existing database:

```sh
CC_DB_METHOD=existing \
CC_DATABASE_URL='postgres://cc:cc@db:5432/cc?sslmode=disable' \
CC_ADMIN_EMAIL=you@example.com CC_ADMIN_PASSWORD='strong-pass' \
CC_WEB_PORT=3000 CC_API_PORT=8080 CC_GRPC_PORT=9090 \
./install/install.sh --non-interactive
```

## Generated files

- `.env`: config and secrets (mode `600`, git-ignored). Source of truth for the stack.
  Edit it directly to change config; see `.env.example` at the repo root for every
  supported key. Restart afterwards (`./croncompose-ctl.sh restart` or `make pm2-restart`).
- `croncompose-ctl.sh` / `croncompose-ctl.ps1`: process manager (git-ignored).
- `.run/`: logs, pids, TLS material, and agent data (git-ignored).
- `control-plane/bin/`, `cli/bin/`, `agent/bin/`: compiled binaries (git-ignored).

## Production notes

This is a from-source install meant to get you running quickly. For real deployments:
set **Advertise host** to a real DNS name, terminate TLS in front of the API and UI,
replace the self-signed CA under `<runtime>/tls` with your own PKI, and run the
services under a process supervisor. The repository's `docker-compose.prod.yml` is a
container-based alternative.

## Troubleshooting

- **"Go/Node is required"**: install the toolchain and re-run; the installer prints the
  download links.
- **Web didn't bind its port**: check `./croncompose-ctl.sh logs web`. The UI is built
  with `output: "standalone"` and runs as `node .next/standalone/server.js`.
- **Migrations failed**: verify the `DATABASE_URL` is reachable and the role can create
  tables. The migration tool retries the connection briefly on startup.
- **Blank page or 404 at the root**: open `/app`, not `/`. The control plane redirects the
  bare URL to `/app`.
- **`/app` returns 502 or a blank page**: the control plane couldn't reach the web UI.
  Check `./croncompose-ctl.sh logs web` and that `WEB_UPSTREAM` in `.env` points at the
  web UI port.

# Installation

Two supported ways to run the **control plane**, plus several ways to put an **agent** on each target.

| Path | When to use |
|------|-------------|
| **From source** (`./install/install.sh`) | Raspberry Pi, a VM, or any single box. Builds Go + Next.js, can provision Postgres, supervises with pm2. |
| **Docker Compose** (`docker-compose.prod.yml`) | One host or orchestrator; recommended production shape. |
| **Dev make targets** | Day-to-day development. See [Development](Development.md). |

## Prerequisites

### Control plane (from source)

- **Go 1.25+**
- **Node 20+** and npm
- **PostgreSQL 16** — optional to install yourself. The installer can use an existing local server, install Postgres via the OS package manager, or take a remote `DATABASE_URL`.

### Control plane (Compose)

- Docker + Compose v2
- A `.env` with `SESSION_SECRET`, `SECRETS_MASTER_KEY`, and seed admin credentials

### Agents

- Linux (amd64 or arm64) or macOS for execution
- Outbound TCP to the control plane’s **gRPC port** (default 9090) and, once, its HTTP API for enrollment
- No inbound ports

## Path A — From source (interactive)

```sh
git clone https://github.com/workvar/cron-compose.git
cd cron-compose
./install/install.sh
```

Windows (control plane only):

```powershell
git clone https://github.com/workvar/cron-compose.git
cd cron-compose
./install/install.ps1
```

Four questions: public URL, HTTP port, database, admin login. Secrets are generated (or reused from an existing `.env` so re-running never orphans stored secrets).

### `--advanced`

The default path is those four questions. For runtime directory, every port, the database method menu (OS package manager or Docker Postgres), log level, **OIDC SSO**, and extra `KEY=VALUE` lines:

```sh
./install/install.sh --advanced
```

When it asks **Configure OIDC single sign-on now?**, put the identity provider’s **client id** and **secret** there (GitLab Applications work; GitHub OAuth Apps are not OIDC). Register this callback with the provider:

`https://<your-host>/api/v1/auth/oidc/callback`

You can also paste the same keys into `.env` later and `./croncompose-ctl.sh restart`. Details: [OIDC SSO](Operations.md#oidc-sso).

Then:

```sh
./croncompose-ctl.sh status
./croncompose-ctl.sh logs web
./croncompose-ctl.sh restart
./croncompose-ctl.sh boot          # survive reboots (pm2 startup + save)
```

Open `http://<host>:<http port>` — it redirects to `/app`.

Full flag and `CC_*` env reference: [`install/README.md`](https://github.com/workvar/cron-compose/blob/main/install/README.md).

### Non-interactive example

```sh
CC_DB_METHOD=existing \
CC_DATABASE_URL='postgres://cc:cc@db:5432/cc?sslmode=disable' \
CC_ADMIN_EMAIL=you@example.com CC_ADMIN_PASSWORD='strong-pass' \
CC_WEB_PORT=3000 CC_API_PORT=8080 CC_GRPC_PORT=9090 \
./install/install.sh --non-interactive
```

## Path B — Docker Compose

```sh
# next to docker-compose.prod.yml
SESSION_SECRET=$(openssl rand -hex 32)
SECRETS_MASTER_KEY=$(openssl rand -hex 32)
# write those plus SEED_ADMIN_* and PUBLIC_* into .env

docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d postgres
docker compose -f docker-compose.prod.yml run --rm --no-deps \
  --entrypoint /usr/local/bin/migrate control-plane -dir /migrations
docker compose -f docker-compose.prod.yml up -d
```

Published ports: HTTP (`PUBLIC_PORT`, default 8080) and gRPC (`PUBLIC_GRPC_PORT`, default 9090). The web container has no published ports.

`./update.sh` rebuilds, migrates, and restarts (auto-detects Compose vs source).

## Installing an agent on a remote server

Create a server in the UI to mint a token, then on the target. Full guide, including the gRPC address requirements, verification and uninstall: [Agent-Installation](Agent installation.md).

### One-liner (from a GitHub release)

```sh
curl -sSL https://raw.githubusercontent.com/workvar/cron-compose/main/scripts/install-agent.sh | \
  sudo TOKEN=<token> \
       CONTROL_PLANE_HTTP=https://<host>/api/v1 \
       CONTROL_PLANE_ADDR=<host>:9090 \
       AGENT_VERSION=v0.0.1 bash
```

### Debian / Ubuntu package

```sh
sudo dpkg -i croncompose-agent_v0.0.1_amd64.deb   # or arm64
# edit /etc/croncompose/agent.env
sudo systemctl enable --now croncompose-agent
```

### Alpine package

```sh
sudo apk add --allow-untrusted croncompose-agent_v0.0.1_amd64.apk
sudo vi /etc/croncompose/agent.env
sudo rc-service croncompose-agent start
```

Packages install `/usr/local/bin/croncompose-agent`, a hardened systemd unit (`NoNewPrivileges`, `ProtectSystem=full`, `ProtectHome`, `PrivateTmp`), and create the `croncompose` system user plus `/var/lib/croncompose` (0700).

### Agent env

```sh
CONTROL_PLANE_ADDR=cc.example.com:9090
CONTROL_PLANE_HTTP=https://cc.example.com/api/v1
CONTROL_PLANE_SNI=cc.example.com
DATA_DIR=/var/lib/croncompose
```

Then `agent enroll --token=...` if the installer did not already do it.

The Linux/macOS control-plane installer can also enroll a **local** agent so the same box is a job runner. The Windows installer does not; point Linux/macOS agents at that control plane.

## Uninstalling

```sh
./uninstall.sh              # interactive; type the database name to confirm the drop
./uninstall.sh --dry-run
./uninstall.sh --keep-db
```

Removes pm2 processes, systemd units, runtime dir, built artifacts, `.env`, and optionally the database. Leaves the checkout, PostgreSQL, Node, and pm2 installed.

To remove an agent from a remote server, see [Agent-Installation#uninstall-an-agent-linux-script-install](Agent installation.md).

## Troubleshooting

| Symptom | What to check |
|---------|----------------|
| “Go/Node is required” | Install the toolchain and re-run; the installer prints download links. |
| Blank page at `/` | Open `/app`. The control plane redirects `/` → `/app`; `next dev` does not. |
| `/app` is 502 | Control plane cannot reach the UI. `WEB_UPSTREAM` and `./croncompose-ctl.sh logs web`. |
| Migrations failed | `DATABASE_URL` reachable; role can create tables. |
| Agent will not enroll | HTTP URL, token unexpired/unused, clock sane. |
| Agent will not stay connected | gRPC port reachable **directly** (TCP passthrough, not HTTP proxy). `TLS_HOSTS` / SNI must match the name the agent dials. Regenerating `TLS_DIR` forces re-enroll. |

Next: [Deployment](Deployment.md) for TLS, secrets, and production hardening.

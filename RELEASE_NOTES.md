# CronCompose v0.0.1

First tagged release of CronCompose — a web tool for writing, scheduling, and managing cron-style jobs across remote Linux machines.

## Highlights

- **Control plane** — Next.js web UI, Go (Fiber) REST API, Postgres, and mTLS gRPC for agents. Single HTTP entry point serves `/api` and proxies `/app` to the UI.
- **Agent** — Small Go binary that dials out to the control plane, holds a local job schedule, runs shell jobs, and streams logs. Jobs keep firing when the control plane is unreachable.
- **Interactive installer** — `install/install.sh` (Linux/macOS) and `install/install.ps1` (Windows) build from source, provision Postgres, apply migrations, and start the stack under PM2.
- **Job management** — Script editor, cron scheduling, timezones, manual run-now, run history, and live log streaming.
- **Multi-user RBAC** — Owner, admin, operator, and viewer roles with audit logging.
- **Secrets** — Encrypted storage with injection into runs and log scrubbing.
- **Connectors** — Manage nginx, systemd, Docker, and PM2 from the portal (discover, status, config, lifecycle).
- **Notifications** — Webhook, Slack, and email targets on run failure.
- **Agent self-update** — Control plane can offer pinned, checksum-verified agent updates.
- **Web terminal** — Browser-based PTY session to enrolled servers.
- **CLI** — `cc` command for jobs, secrets, audit, and auth from the shell.

## Agent binaries (this release)

Pre-built static Linux binaries are attached to this GitHub release:

| Asset | Platform |
|-------|----------|
| `croncompose-agent-linux-amd64` | Linux x86_64 |
| `croncompose-agent-linux-arm64` | Linux ARM64 |
| `croncompose-agent_v0.0.1_amd64.deb` | Debian/Ubuntu amd64 |
| `croncompose-agent_v0.0.1_arm64.deb` | Debian/Ubuntu arm64 |
| `croncompose-agent_v0.0.1_amd64.apk` | Alpine amd64 |
| `croncompose-agent_v0.0.1_arm64.apk` | Alpine arm64 |

Install on a remote server with the enrollment token from the UI:

```sh
curl -sSL https://raw.githubusercontent.com/workvar/cron-compose/main/scripts/install-agent.sh | \
  sudo TOKEN=<token> \
       CONTROL_PLANE_HTTP=https://<host>/api/v1 \
       CONTROL_PLANE_ADDR=<host>:9090 \
       AGENT_VERSION=v0.0.1 bash
```

Or download the binary directly from this release's assets.

## Control plane install

Clone the repo and run the interactive installer on the machine that will host the control plane:

```sh
git clone https://github.com/workvar/cron-compose.git
cd cron-compose
./install/install.sh
```

Requires Go 1.25+, Node 20+, and PostgreSQL (the installer can install Postgres for you on supported Linux distros).

## Documentation

- [Architecture](docs/architecture.md)
- [API reference](docs/api.md)
- [Installer guide](install/README.md)
- [Operations](docs/operations.md)
- [Roadmap](docs/roadmap.md)

## Known limitations

- Connectors for Apache, Caddy, Traefik, HAProxy, system cron, and ufw are designed but not yet implemented.
- Windows installer covers the control plane only; agents target Linux and macOS.
- Horizontally scaled control plane is not supported (single replica).

# CronCompose v0.0.2

Installer, ports, and UI polish on top of v0.0.1.

## Highlights

- **Safer `.env` writing** — the installer quotes values only when needed (spaces, `#`, `$`, quotes). Ports, URLs, and hex secrets stay bare (`PORT=3107`) so PM2 and Next.js do not treat wrapper quotes as part of the value. Reinstalls unquote previously quoted secrets.
- **Quoted env at runtime** — the control plane and `scripts/pm2-env.js` strip wrapping quotes from process env, so an already-quoted `.env` no longer crashes `PUBLIC_BASE_URL` parsing or binds the web UI to port 3000.
- **Ports page** — new sidebar item listing listening sockets owned by systemd and pm2. Inline labels (saved on the control plane), full-width search, and Close still means stop (not kill). Labels also appear on the connector Ports tab.
- **Searchable dropdowns** — native `<select>` controls (Secrets scope, job concurrency, catch-up, timezone, connector config path) are replaced with a combobox that matches input height and can be typed to filter.
- **Sidebar** — the “Offline-first agents” promo hides once at least one server exists.
- **Terminal chrome** — run logs and live sessions use a dark terminal frame; “Run a command” is a `$` prompt.

## Agent binaries (this release)

Pre-built static Linux binaries are attached to this GitHub release:

| Asset | Platform |
|-------|----------|
| `croncompose-agent-linux-amd64` | Linux x86_64 |
| `croncompose-agent-linux-arm64` | Linux ARM64 |
| `croncompose-agent_v0.0.2_amd64.deb` | Debian/Ubuntu amd64 |
| `croncompose-agent_v0.0.2_arm64.deb` | Debian/Ubuntu arm64 |
| `croncompose-agent_v0.0.2_amd64.apk` | Alpine amd64 |
| `croncompose-agent_v0.0.2_arm64.apk` | Alpine arm64 |

Install on a remote server with the enrollment token from the UI:

```sh
curl -sSL https://raw.githubusercontent.com/workvar/cron-compose/main/scripts/install-agent.sh | \
  sudo TOKEN=<token> \
       CONTROL_PLANE_HTTP=https://<host>/api/v1 \
       CONTROL_PLANE_ADDR=<host>:9090 \
       AGENT_VERSION=v0.0.2 bash
```

Or download the binary directly from this release's assets.

## Control plane install

Clone the repo and run the interactive installer on the machine that will host the control plane:

```sh
git clone https://github.com/workvar/cron-compose.git
cd cron-compose
./install/install.sh
```

Requires Go 1.25+, Node 20+, and PostgreSQL (the installer can install Postgres for you on supported Linux distros). Existing installs: pull, rebuild the control plane so migration `0011_port_labels.sql` applies, then restart PM2.

## Documentation

- [Architecture](docs/architecture.md)
- [API reference](docs/api.md)
- [Installer guide](install/README.md)
- [Operations](docs/operations.md)
- [Roadmap](docs/roadmap.md)

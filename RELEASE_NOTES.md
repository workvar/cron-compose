# CronCompose v0.0.3

Reliable port discovery for systemd and pm2, plus in-app update visibility.

## Highlights

- **Port discovery that works on real hosts** — the agent now falls back from `ss` to `lsof`, then retries with passwordless `sudo` when unprivileged `ss -p` omits process columns (common on Raspberry Pi and other boxes where the agent runs as an unprivileged user). systemd ownership is built from `systemctl show` (MainPID, cgroup PIDs) instead of relying only on per-socket cgroup reads. pm2 ownership includes child/worker PIDs, not just the top-level `jlist` pid.
- **Agent installer sudoers** — `scripts/install-agent.sh` writes `/etc/sudoers.d/croncompose-agent` with grants for `ss` and `lsof` so the Ports page works out of the box on fresh agent installs.
- **Clearer Ports empty state** — when systemd/pm2 connectors exist but no sockets are reported, the UI explains that services must be listening and socket inspection may need a sudo grant (instead of implying no connector was discovered).
- **Updates panel (Settings)** — check for control-plane and agent updates from the UI, see what is current vs available, and trigger agent self-updates per server when policy allows.

## Upgrade notes

### Control plane (source install)

```sh
cd cron-compose
git pull
./croncompose-ctl.sh restart   # or rebuild if you changed Go/TS locally
```

No new database migrations in this release.

### Agent (binary / package install)

Reinstall or replace the agent binary, then restart the service:

```sh
# package install
sudo dpkg -i croncompose-agent_v0.0.3_<arch>.deb
sudo systemctl restart croncompose-agent

# or curl installer
curl -sSL ... AGENT_VERSION=v0.0.3 bash
```

### Agent (source / pm2 on the same host as the control plane)

```sh
cd cron-compose/agent
go build -o bin/agent ./cmd/agent
cd ..
pm2 restart croncompose-agent
# or: ./croncompose-ctl.sh restart
```

### Socket inspection sudo (existing installs)

Re-run any installer path to refresh `/etc/sudoers.d/croncompose-agent`, or:

```sh
sudo ./install/lib/agent_sudoers.sh <agent-user>   # e.g. pi or croncompose
```

Test: `sudo -n ss -H -lntp | head` should show `users:(("name",pid=...))` on listen lines.

## Agent binaries (this release)

Pre-built static Linux binaries are attached to this GitHub release:

| Asset | Platform |
|-------|----------|
| `croncompose-agent-linux-amd64` | Linux x86_64 |
| `croncompose-agent-linux-arm64` | Linux ARM64 |
| `croncompose-agent_v0.0.3_amd64.deb` | Debian/Ubuntu amd64 |
| `croncompose-agent_v0.0.3_arm64.deb` | Debian/Ubuntu arm64 |
| `croncompose-agent_v0.0.3_amd64.apk` | Alpine amd64 |
| `croncompose-agent_v0.0.3_arm64.apk` | Alpine arm64 |

Install on a remote server with the enrollment token from the UI:

```sh
curl -sSL https://raw.githubusercontent.com/workvar/cron-compose/main/scripts/install-agent.sh | \
  sudo TOKEN=<token> \
       CONTROL_PLANE_HTTP=https://<host>/api/v1 \
       CONTROL_PLANE_ADDR=<host>:9090 \
       AGENT_VERSION=v0.0.3 bash
```

Or download the binary directly from this release's assets.

## Documentation

- [Connectors — Ports](docs/connectors.md#ports-page)
- [Operations — agent socket inspection](docs/operations.md#agent-socket-inspection-ports-page)
- [Wiki — Connectors](https://github.com/workvar/cron-compose/wiki/Connectors)
- [Wiki — Releases](https://github.com/workvar/cron-compose/wiki/Releases)

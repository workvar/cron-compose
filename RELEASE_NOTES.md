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

If the Ports page stays empty after upgrading the agent, grant the **user that runs the agent** passwordless sudo for socket tools. The standalone installer creates a `croncompose` user; a source install usually runs the agent as your login user (e.g. `pi`):

```sh
# standalone agent user (typical package install)
sudo tee /etc/sudoers.d/croncompose-agent <<'EOF'
croncompose ALL=(root) NOPASSWD: /usr/bin/ss, /usr/sbin/ss, /usr/bin/lsof
EOF

# source install as pi (Raspberry Pi dev box)
sudo tee /etc/sudoers.d/croncompose-agent <<'EOF'
pi ALL=(root) NOPASSWD: /usr/bin/ss, /usr/bin/lsof
EOF

sudo chmod 0440 /etc/sudoers.d/croncompose-agent
sudo visudo -c
```

Test:

```sh
sudo -n ss -H -lntp | head
```

You should see `users:(("name",pid=...))` on listen lines.

## Agent binaries (this release)

This release is **notes-only**: no prebuilt agent binaries. Install with:

```sh
curl -sSL https://github.com/workvar/cron-compose/releases/latest/download/install-agent.sh | \
  sudo TOKEN=<token> \
       CONTROL_PLANE_HTTP=https://<host>/api/v1 \
       CONTROL_PLANE_ADDR=<host>:9090 \
       bash
```

Or clone the tag and `go build` the agent yourself.

## Documentation

- [Connectors — Ports](docs/connectors.md#ports-page)
- [Operations — agent socket inspection](docs/operations.md#agent-socket-inspection-ports-page)
- [Wiki — Connectors](https://github.com/workvar/cron-compose/wiki/Connectors)
- [Wiki — Releases](https://github.com/workvar/cron-compose/wiki/Releases)

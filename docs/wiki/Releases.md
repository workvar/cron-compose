# Releases

Tagged builds are published on [GitHub Releases](https://github.com/workvar/cron-compose/releases). The control plane is installed from source or Docker; agents ship as static Linux binaries plus `.deb` / `.apk` packages.

## Current: v0.0.3

**[Download v0.0.3](https://github.com/workvar/cron-compose/releases/tag/v0.0.3)**

- Reliable **Ports page** discovery for systemd and pm2 (`ss`/`lsof` fallback, sudo retry, cgroup and child-PID ownership).
- Agent installer writes sudoers for socket inspection on fresh Linux installs.
- **Updates panel** in Settings (check versions, offer agent self-update per server).

Upgrade: pull the repo and `./croncompose-ctl.sh restart` for the control plane; rebuild or reinstall the agent binary on each target. See the [release notes](https://github.com/workvar/cron-compose/blob/main/RELEASE_NOTES.md) for sudoers steps on existing Pis.

## v0.0.2

- Safer `.env` quoting and runtime unquoting.
- Ports page (labels, search, Close = stop unit/process).
- Searchable combobox dropdowns, terminal chrome, sidebar polish.

## v0.0.1

First tagged release: core jobs, agents, connectors (nginx/systemd/Docker/pm2), terminal, CLI.

## Versioning

- Tags look like `v0.0.x`. Pushing a tag runs [`.github/workflows/release.yml`](https://github.com/workvar/cron-compose/blob/main/.github/workflows/release.yml) to attach agent binaries.
- The control plane can poll GitHub for agent updates when configured, or you can set `AGENT_UPDATE_*` env vars explicitly.
- There is no automatic control-plane updater yet; upgrade by pulling the repo or rebuilding your image.

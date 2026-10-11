# Connectors

Connectors let CronCompose see and manage service managers that already run on a target: web servers, process managers, containers. The agent discovers what is installed, reads live status and config, and (with enough privilege) edits config and drives lifecycle — all from the portal, over the **existing outbound gRPC stream**. No new inbound access.

Full design: [`docs/connectors.md`](https://github.com/workvar/cron-compose/blob/main/docs/connectors.md).

## Goals

- Discover installed service managers automatically.
- Show live status (running/stopped, version, object counts) and current configuration.
- Edit configuration safely: validate before activating, back up before writing, roll back on failure.
- Drive start / stop / restart / reload / enable / disable.
- Keep the agent unprivileged by default; privileged actions go through a narrow, explicit, per-connector allowlist.
- Reuse transport, audit log, and RBAC. Add no new daemons.

## Non-goals

- Installing or removing the underlying software.
- Cross-server templating or fleet rollout (one connector instance at a time).
- A vendor-aware config IDE. Validation is delegated to each tool (`nginx -t`, and so on).

## Concepts

| Term | Meaning |
|------|---------|
| **Connector** | One integration on one server, e.g. nginx on `web-01`. Identified by `(server_id, kind, instance)`. |
| **Resource** | `config_file` (editable text + validator + reload) or `object` (lifecycle unit: systemd unit, Docker container, pm2 process). |
| **Snapshot** | Immutable backup of a config file, taken before every write. Mirrors job versions: never mutate in place. |
| **Operation** | One read / validate / apply / lifecycle / rollback. Persisted like a run, with step-by-step output. |
| **Capabilities** | Booleans reported at discovery (`manages_config`, `can_validate`, `can_reload`, …). The UI only offers what the agent can actually do. |

## Implemented today

| Kind | Detect | Config | Validate | Activate | Objects |
|------|--------|--------|----------|----------|---------|
| nginx | `nginx -v` | `/etc/nginx/**` | `nginx -t` | reload | none |
| systemd | `systemctl --version` | unit files / drop-in overrides | `systemd-analyze verify` | daemon-reload then reload/restart | units |
| Docker | `docker version` | compose files | `docker compose config -q` | `up -d` / container restart | containers |
| pm2 | `pm2 --version` (per user) | ecosystem / `pm2 save` dump | JSON on start | reload/restart | processes |

## Ports page

The **Ports** sidebar lists TCP listen sockets owned by **systemd** or **pm2**. Each connector also has a **Ports** tab. You can label sockets (stored on the control plane) and **Close** a port — that stops the owning unit or pm2 process, not a raw `kill`.

How the agent finds sockets (since **v0.0.3**):

1. `ss -lntp`, then `lsof` if `ss` omits PIDs (common when the agent is unprivileged).
2. Retry with passwordless `sudo` for `ss` / `lsof` when sudoers allows it.
3. Map PIDs to owners: systemd via `systemctl show` + cgroup PIDs; pm2 via `jlist` + child processes.

**If the page is empty but connectors show services running**, re-run the installer or `sudo ./install/lib/agent_sudoers.sh <agent-user>`. See [Operations — socket inspection](Operations.md#agent-socket-inspection-ports-page).

| Install | Typical agent user | Sudoers |
|---------|-------------------|---------|
| `scripts/install-agent.sh` | `croncompose` | automatic |
| Source install (`pm2` on a Pi) | your login user (e.g. `pi`) | `./install/install.sh` or `agent_sudoers.sh` |

## Designed, not implemented

Apache, Caddy, Traefik, HAProxy, system cron, ufw. The tables and security notes are in the repo spec. Extra guardrails are already specified for **ufw** (must not drop the control path) and **cron** (overlaps CronCompose’s own jobs).

## Access control

- Viewer: list connectors, status, operations history, ports.
- Operator: lifecycle actions (start/stop/…), port labels.
- Admin: read and write config bytes, snapshots, restore.

Every mutating call is audited. Apply uses `base_checksum` so a concurrent edit is refused rather than overwritten.

## How it moves on the wire

Connector requests are commands on the same `AgentStream` as jobs. If the agent is offline, the control plane records `offline` and returns `503`. Results are stored as operations so the browser can leave and still see how it ended.

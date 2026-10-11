# Roadmap

Phased so each phase ships something usable. The MVP is the smallest slice that proves the core promise: write a job in the UI, have it run on a remote server on a schedule, and watch the logs.

**Status:** Phases 0–4 are implemented. Connectors (Phase 5) are complete through Phase C (nginx, systemd, Docker, pm2). What remains is under “Not built yet”.

Repo spec: [`docs/roadmap.md`](https://github.com/workvar/cron-compose/blob/main/docs/roadmap.md). First tagged release: [v0.0.1](https://github.com/workvar/cron-compose/releases/tag/v0.0.1).

## Shipped

| Phase | What landed |
|-------|-------------|
| **0 Design** | Architecture, data model, agent protocol, API, security model |
| **1 MVP** | REST + gRPC enroll/stream, Postgres, email/password, local agent scheduler, UI for servers/jobs/run-now/live logs |
| **2 Multi-user and safety** | RBAC, encrypted secrets + log scrubbing, API tokens, audit log in the UI, retention/pruning |
| **3 Scale and reach** | Label targeting, job templates, notifications (webhook/Slack/email), checksum-verified agent auto-update |
| **4 Depth** | systemd-run resource limits, Prometheus metrics, per-run log cap + retention pruner |
| **5 Connectors A–C** | Discover / status / config / lifecycle for nginx, systemd, Docker, pm2 |
| **Also** | Web terminal (PTY), `cc` CLI, from-source installer, Compose prod file, `.deb`/`.apk` agent packages |

Definition of done for the MVP, still the product’s north star: a Raspberry Pi behind a home router runs a job every 6 hours, survives a control-plane restart, and shows correct logs and history.

## Not built yet

- **Connectors Phase D (breadth).** Apache, Caddy, Traefik, HAProxy, system cron, and ufw are designed and unimplemented.
- **Job dependencies / DAGs.** Still single jobs only.
- **Object storage for logs.** Cap + retention cover the cases that forced this onto the list; revisit if someone needs full logs kept forever.
- **Horizontally scaled control plane.** Log broker and connector request registry are in-process. A fan-out bus is the prerequisite.
- **Windows installer parity.** `install.ps1` has not been kept in lockstep with the Linux/macOS installer, and there is no Windows agent.

## Product / business decisions (open)

1. **Deployment model.** Self-hosted single-tenant first (current). Multi-tenant SaaS would require `org_id` in the schema.
2. **Catch-up default.** Implemented as `once`. Whether that is right needs real-user confirmation.
3. **Job dependencies.** Is “run B after A” a real need, or out of scope?
4. **gRPC vs WebSocket.** gRPC/mTLS shipped and carries jobs, logs, connectors, and the terminal. WebSocket fallback is off the table unless a self-hoster reports otherwise.
5. **Cron syntax.** Settled: 5-field only. Sub-minute cadence is a daemon, not cron.

## What CronCompose is not

- Not a CI/CD or container build system.
- Not Kubernetes-native job orchestration (agents are OS processes).
- Not a Windows job runner.
- Not a hosted SaaS (yet). You run the control plane.

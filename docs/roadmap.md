# Roadmap

Phased so each phase ships something usable. The MVP is the smallest slice that proves
the core promise: write a job in the UI, have it run on a remote server on a schedule,
and watch the logs.

## Phase 0 — Design (current)

The spec in this repo. Output: agreed architecture, data model, agent protocol, API,
security model.

## Phase 1 — MVP

The end-to-end vertical slice, single user, no RBAC yet.

Control plane:

- Fiber v3 REST API: servers CRUD, jobs CRUD with versions, runs, SSE log stream.
- gRPC agent endpoint: `Enroll` + `AgentStream`.
- Postgres schema from [data-model.md](data-model.md).
- Basic email/password auth.

Agent:

- Enrollment via token, mTLS thereafter.
- Local store + local cron scheduler.
- Execute jobs (interpreter, env, working dir, timeout, concurrency=skip).
- Stream logs and run status; buffer + replay on reconnect.

UI (Next.js 16):

- Add a server and copy the install command.
- Write a job: script editor, cron schedule, timezone, target server.
- Manual "run now".
- Run history and live log view.

Definition of done: a Raspberry Pi behind a home router runs a job every 6 hours,
survives a control-plane restart, and shows correct logs and history.

## Phase 2 — Multi-user and safety

- RBAC roles (owner/admin/operator/viewer).
- Secrets: encrypted storage, injection into runs, log scrubbing.
- API tokens for programmatic access.
- Audit log surfaced in the UI.
- Run-history retention/pruning.

## Phase 3 — Scale and reach

- Target multiple servers from one job via labels (run the same job on all "edge" boxes).
- Job templates / a small library of common scripts.
- Notifications on failure: email, Slack, generic webhook.
- Agent auto-update channel.

## Phase 4 — Depth

- Resource limits via `systemd-run` / cgroups (CPU, memory).
- Metrics and dashboards (success rate, durations, trends).
- Larger log handling: object storage or file backend beyond the Postgres cap.
- Possibly job dependencies / simple DAGs (run B after A succeeds).

## Phase 5 — Connectors

Manage the service managers already running on a target server (nginx, Apache, Caddy,
Traefik, HAProxy, pm2, systemd, Docker, system cron, ufw) from the portal: discover what
is installed, view live status and current config, and safely edit config and drive
lifecycle actions. Reuses the agent channel, audit log, and RBAC; the agent stays
unprivileged by default. Full design and phasing in [connectors.md](connectors.md).

## Open questions

These need a decision before or during the relevant phase:

1. **Deployment model.** Self-hosted single-tenant first, or multi-tenant SaaS from the
   start? This decides whether `org_id` lands in the schema in Phase 1. Current
   assumption: self-hosted single-tenant first.
2. **Log storage backend.** Postgres cap is fine for the MVP. When do jobs with large or
   long-running output force object storage or files? Decide in Phase 4, or sooner if a
   user hits the cap.
3. **Catch-up policy default.** After a long offline period, default to running a single
   catch-up (`once`). Confirm this matches user expectations vs `skip`.
4. **Job dependencies / DAGs.** Single jobs only for now. Is "run B after A" a real need,
   or out of scope? Affects the data model if yes.
5. **Agent distribution.** Single binary + install script, a system package (apt/rpm), a
   container, or all three? Affects the install UX shown in the UI.
6. **gRPC vs WebSocket transport.** gRPC is the recommendation. If self-hosters find the
   gRPC/mTLS setup heavy, the WebSocket fallback is on the table. Validate during Phase 1.
7. **Cron syntax.** Standard 5-field, or 6-field with seconds, plus human helpers like
   "every 6 hours"? Recommend supporting both cron and a friendly preset picker in the UI.

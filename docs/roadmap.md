# Roadmap

Phased so each phase ships something usable. The MVP is the smallest slice that proves
the core promise: write a job in the UI, have it run on a remote server on a schedule,
and watch the logs.

## Status

Phases 0 through 4 are implemented, and connectors (Phase 5) are complete through
Phase C. What remains is listed under "Not built yet" at the bottom.

## Phase 0 — Design (done)

The spec in this repo. Output: agreed architecture, data model, agent protocol, API,
security model.

## Phase 1 — MVP (done)

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

## Phase 2 — Multi-user and safety (done)

- RBAC roles (owner/admin/operator/viewer).
- Secrets: encrypted storage, injection into runs, log scrubbing.
- API tokens for programmatic access.
- Audit log surfaced in the UI.
- Run-history retention/pruning.

## Phase 3 — Scale and reach (done)

- Target multiple servers from one job via labels (run the same job on all "edge" boxes).
  `jobs.target_kind = 'labels'` plus a jsonb selector; resolved at sync and run-now time.
- Job templates: six built-ins seeded by migration 0008, plus "save this job as a
  template". Offered in the job wizard's script step.
- Notifications on failure: webhook, Slack (Block Kit), and email (SMTP with
  STARTTLS or implicit TLS). Targets can be scoped by server label and by run
  outcome, carry a test-delivery button, and record their last delivery error.
- Agent auto-update channel: the control plane offers an update on Hello when the
  agent's version differs from the configured target. The agent verifies a pinned
  sha256 before swapping its own binary and exits for its supervisor to restart it.
  Inert until `AGENT_UPDATE_VERSION`, `AGENT_UPDATE_URL` and `AGENT_UPDATE_SHA256`
  are all set: no checksum, no update.

## Phase 4 — Depth (done)

- Resource limits via `systemd-run` (CPU quota, memory, max processes, IO weight).
  The agent checks that systemd is actually running, not merely installed, and
  degrades to an unlimited run with a note in the log rather than failing.
- Metrics: run durations, log bytes, connector operations, notification deliveries,
  and retention deletions, on top of the existing HTTP and agent gauges. `/metrics`
  can be gated behind `METRICS_TOKEN`.
- Log handling: a per-run storage cap (`RUN_LOG_MAX_BYTES`, 5 MiB default) that
  truncates storage without truncating the run, and a retention pruner for
  `run_logs`, `runs`, `audit_log` and `connector_operations`. Pruning is off until a
  window is configured. An object-storage backend is still not built; the cap plus
  retention is what replaced the need for one so far.
- Job dependencies / DAGs: NOT built. Still an open question below.

## Phase 5 — Connectors (A, B and C done; D pending)

Manage the service managers already running on a target server (nginx, Apache, Caddy,
Traefik, HAProxy, pm2, systemd, Docker, system cron, ufw) from the portal: discover what
is installed, view live status and current config, and safely edit config and drive
lifecycle actions. Reuses the agent channel, audit log, and RBAC; the agent stays
unprivileged by default. Full design and phasing in [connectors.md](connectors.md).

## Not built yet

- **Connectors Phase D (breadth).** Only nginx, systemd, docker and pm2 have
  providers. apache, caddy, traefik, haproxy, system cron and ufw are designed in
  [connectors.md](connectors.md) and unimplemented.
- **Job dependencies / DAGs.** See the open question below.
- **Object storage for logs.** The per-run cap and retention pruner cover the cases
  that forced this onto the list; revisit if someone genuinely needs full logs kept.
- **Horizontally scaled control plane.** The log broker and the connector request
  registry are both in-process, so a second replica would not see the first one's
  agent streams. A fan-out bus is the prerequisite.
- **Windows installer parity.** `install.ps1` has not been updated alongside the
  Linux/macOS installer.

## Open questions

These need a decision before or during the relevant phase:

1. **Deployment model.** Self-hosted single-tenant first, or multi-tenant SaaS from the
   start? This decides whether `org_id` lands in the schema in Phase 1. Current
   assumption: self-hosted single-tenant first.
2. **Log storage backend.** Postgres cap is fine for the MVP. When do jobs with large or
   long-running output force object storage or files? Decide in Phase 4, or sooner if a
   user hits the cap.
3. **Catch-up policy default.** Implemented: the agent persists each job's last fire
   time locally and, on startup, replays what the schedule missed. `once` runs one
   catch-up however many windows were missed, `all` runs each up to a cap of 20, and
   `skip` does nothing. `once` is the default. Whether that is the right default is
   still worth confirming with real users.
4. **Job dependencies / DAGs.** Still single jobs only, and still undecided. Is "run B
   after A" a real need, or out of scope? Affects the data model if yes.
5. **Agent distribution.** Single binary + install script, a system package (apt/rpm), a
   container, or all three? Affects the install UX shown in the UI.
6. **gRPC vs WebSocket transport.** gRPC/mTLS shipped and carries jobs, logs,
   connectors and the web terminal over one stream. The WebSocket fallback was never
   needed and is off the table unless a self-hoster reports otherwise.
7. **Cron syntax.** Settled: standard 5-field only, with a preset picker in the wizard.
   Seconds were not added; a job that needs sub-minute cadence wants a daemon, not cron.

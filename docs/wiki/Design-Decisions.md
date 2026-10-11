# Design decisions

Architecture Decision Records (ADRs) for CronCompose. Each records the options considered, the choice made, and the consequences. Source of truth for the narrative lives in the repo docs (`docs/architecture.md`, `docs/security.md`, `docs/agent-protocol.md`); this page is the indexed summary for wiki readers.

See also: [Diagrams](Diagrams.md) · [Architecture](Architecture.md) · [How it Works](How-it-Works.md)

---

## ADR-001 — Agent-local scheduler (not central)

**Status:** Accepted  
**Context:** Jobs must keep firing on intermittently connected hosts (home Pis, NAT, flaky links).

| Option | Summary |
|--------|---------|
| A. Central scheduler | Control plane evaluates cron and pushes “run now” at fire time |
| **B. Agent-local scheduler (chosen)** | Control plane owns definitions; each agent caches them and evaluates cron locally |

**Decision:** Option B.

**Consequences:**
- Agents mint run ULIDs; control plane upserts by id (idempotent replay).
- Schedules carry IANA timezones; DST is handled on the machine that runs the job.
- Catch-up policy (`once` / `all` / `skip`) decides what happens after a long offline period.
- “Run now” still works as a live-stream command.

**Diagram:** [Job definition and log data flow](Diagrams.md#job-definition-and-log-data-flow)

---

## ADR-002 — Outbound mTLS gRPC (not inbound SSH / WebSocket-first)

**Status:** Accepted  
**Context:** Targets often sit behind consumer routers with no inbound ports.

| Option | Summary |
|--------|---------|
| Inbound SSH / agent listener | Requires port forwarding or public IP |
| JSON WebSocket channel | Simple to proxy; hand-rolled envelope |
| **gRPC bidi stream over mTLS (chosen)** | Typed protobuf, one persistent connection, identity + encryption together |

**Decision:** Agents always dial out on a single `AgentStream`. Enrollment is the only agent REST call (`POST /api/v1/agents/enroll`) so cert bootstrap does not require gRPC first.

**Consequences:**
- No CronCompose inbound ports on targets.
- Protocol can map to WebSocket later if self-hosters need it; message set is transport-agnostic enough.
- Edge / reverse-proxy modes exist for environments that cannot end-to-end mTLS to `:9090`.

**Diagram:** [Agent enroll, sync, and run](Diagrams.md#agent-enroll-sync-and-run)

---

## ADR-003 — Live browser logs via SSE (not WebSocket)

**Status:** Accepted  
**Context:** Operators need live run output in the browser.

**Decision:** Agents push `LogChunk`s up gRPC; `LogBroker` fans them to `GET /api/v1/runs/:id/logs/stream` (SSE).

**Why SSE:** one-directional, auto-reconnects, trivial to serve from Fiber.  
**Exception:** the [Web Terminal](Web-Terminal.md) uses WebSocket because it needs bidirectional byte-accurate I/O.

---

## ADR-004 — Immutable job versions

**Status:** Accepted  
**Context:** Operators edit scripts; history must show what actually ran.

**Decision:** Editing a script inserts a new `job_version` and bumps `current_version_id`. Every `run` records the version id. Connectors mirror the idea with config **snapshots** before every write.

---

## ADR-005 — Secrets: envelope encryption, write-only API, inject at sync

**Status:** Accepted  
**Context:** Jobs need API keys without leaking them in UI, logs, or script files.

**Decision:**
- AES-256-GCM envelope encryption at rest.
- No API endpoint returns secret *values*.
- Control plane decrypts only when building `SyncJobs` and sends secrets over mTLS.
- Agent passes secrets as env vars at exec time; never interpolates into the on-disk script.

See [Security](Security.md).

---

## ADR-006 — Single-tenant self-host first

**Status:** Accepted (for now)  
**Context:** Multi-tenant SaaS would reshape auth, billing, and isolation.

**Decision:** One deployment = one tenant. Roles (owner / admin / operator / viewer) live inside that tenant. An `org_id` can be added later without reshaping core tables.

**Non-goals retained:** Windows agents, general CI/CD, multi-step DAGs (see [Roadmap](Roadmap.md)).

---

## ADR-007 — Connectors reuse the agent stream

**Status:** Accepted  
**Context:** Operators want nginx / systemd / Docker / pm2 without a second agent protocol.

**Decision:** Connectors are additive: discovery, config edit (validate → backup → apply), lifecycle actions, and object logs all ride the existing outbound stream, audit log, and RBAC. No new inbound path.

See [Connectors](Connectors.md).

---

## ADR-008 — Agent root is explicit and narrow

**Status:** Accepted  
**Context:** Some hosts need privileged connector or install actions; default must stay unprivileged.

**Decision:**
- Default agent user is unprivileged.
- Passkey step-up can elevate via a narrow `agent-privctl elevate|demote` sudoers path (never `NOPASSWD: ALL`).
- Root install (`AGENT_RUN_AS_ROOT=1`) is recorded on the server row so the UI matches reality.

See [Agent Root Access](Agent-Root-Access.md).

---

## ADR-009 — Process names are project-qualified

**Status:** Accepted  
**Context:** Two deploy projects can both ship an app named `web`.

**Decision:** Deploy process names are qualified as `<project>-<app>` (unless already matching) so pm2 / systemd / docker names do not collide on one host.

---

## Related reading

| Topic | Page |
|-------|------|
| Topology and components | [Architecture](Architecture.md) |
| End-to-end operator loop | [How it Works](How-it-Works.md) |
| Protocol messages | [Agent Protocol](Agent-Protocol.md) |
| Schema | [Data Model](Data-Model.md) |
| Threat model | [Security](Security.md) |
| Visual maps | [Diagrams](Diagrams.md) |

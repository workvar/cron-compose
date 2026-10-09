# Architecture

## Goals

- Manage scheduled shell jobs across many Linux servers from one web UI.
- Work for servers behind NAT or home firewalls (Raspberry Pi at home, for example),
  not just servers with public inbound access.
- Keep jobs running on a server even when the control plane or network is unavailable.
- Give clear, live visibility: run history, exit codes, durations, and streamed logs.
- Be self-hostable as a small set of services.

## Non-goals (for now)

- Windows targets. Linux only.
- General CI/CD or container build orchestration.
- Multi-step DAGs / job dependencies (tracked as a later phase, see roadmap).

## Topology

```
+---------------------------------------------------------------+
|                        Control Plane                          |
|                                                               |
|   Next.js UI  --REST/SSE-->  Fiber API  <-->  PostgreSQL      |
|   (/ landing, public docs)       ^                            |
|                                  | gRPC bidi stream (mTLS)    |
+----------------------------------|----------------------------+
                                   |  (agents always dial OUT)
              +--------------------+--------------------+
              |                    |                    |
       +------v------+      +------v------+      +------v------+
       |  Agent      |      |  Agent      |      |  Agent      |
       |  Raspberry  |      |  EC2        |      |  bare-metal |
       |  Pi (home)  |      |  instance   |      |  Linux box  |
       +-------------+      +-------------+      +-------------+
```

The agent is always the dialer. The control plane never needs inbound access to a
target server, which is what makes home Raspberry Pi devices behind a consumer router
work without port forwarding.

## Public vs signed-in web

The Next.js app uses `basePath: /app`. The control plane proxies `/` to the marketing
landing (`/app/landing`), redirects `/docs` to `/app/docs`, and forwards `/app/*` to
Next. Routes under landing, docs, and use-cases are public. Everything else requires a
`cc_session` cookie.

Anonymous visitors render inside **PublicChrome** (marketing header/footer). Signed-in
users get the sidebar **AppShell**. Login is full-bleed and never probes `/me` unless a
session cookie is present, so opening `/login` signed-out does not surface a 401
“missing session” banner.

Interactive runtime map: the in-app **Architecture** page embeds the Archify diagram
from `web/public/architecture/runtime.html`.

## Components

### Web UI (Next.js 16, App Router)

Server-rendered React app. Talks to the Fiber API over REST for reads and writes, and
subscribes to Server-Sent Events (SSE) for live run logs and status changes. Public
screens: product landing, use-case guides, `croncompose.yml` docs, login. Signed-in
screens: dashboard, servers, deploys, jobs, connectors (including process logs), tools,
secrets, settings, architecture.

### Control-plane API (Go 1.25, Fiber v3)

Two listeners in one service (or two services sharing the database):

- **REST API** (Fiber v3): everything the browser and external API callers use. See
  [api.md](api.md).
- **Agent gRPC endpoint** (gRPC-Go): the long-lived bidirectional stream every agent
  connects to, plus the unary enrollment RPC. See [agent-protocol.md](agent-protocol.md).

Responsibilities: authentication and RBAC, CRUD for servers/jobs/schedules, storing job
versions, accepting run records and log chunks from agents, fanning live logs out to the
UI over SSE, and being the source of truth that agents sync from.

### PostgreSQL

Stores users, servers, jobs and job versions, schedules, runs, log output (capped),
secrets (encrypted), API tokens, and the audit log. See [data-model.md](data-model.md).

### Agent (Go binary, one per server)

A single static binary. Responsibilities:

- Enroll once using a short-lived token, then authenticate with a client certificate.
- Maintain one outbound gRPC stream to the control plane.
- Keep a local cache of its assigned job definitions in an embedded store (SQLite or
  bbolt) so it survives restarts and control-plane outages.
- Run a local cron scheduler over those definitions.
- Execute each job (write script to a temp file, run with the chosen interpreter,
  capture stdout/stderr/exit code/duration), enforce timeout and concurrency policy.
- Stream logs and run status up the stream; buffer locally and replay on reconnect.

## The scheduling model (important)

There are two ways to build this, and the choice drives everything else.

**Option A, central scheduler:** the control plane evaluates every cron schedule and
pushes a "run now" command to the agent at fire time. Simpler conceptually, but a
control-plane outage or a network blip means missed runs. Bad fit for home Pis on flaky
links.

**Option B, agent-local scheduler (chosen):** the control plane is the source of truth
for job *definitions*. When a job is created or edited, the control plane syncs the
definition down to the relevant agent. The agent stores it locally and runs its own
cron evaluation. Jobs fire on time even if the control plane is down or the link drops;
run records and logs queue locally and upload on reconnect.

We choose **Option B** because resilience for intermittently connected servers is a core
goal. The control plane still supports on-demand "run now" by sending a command over the
stream, and still drives all configuration. But execution does not depend on a live
connection.

Consequences of Option B:

- Agents generate run IDs (ULIDs) locally; the control plane accepts agent-originated
  run records and de-duplicates by ID (idempotent upsert).
- Schedules carry an IANA timezone; the agent evaluates cron in that timezone so DST is
  handled on the machine that actually runs the job.
- A "catch-up policy" decides what happens after a long offline period: run missed
  occurrences once, run all of them, or skip. Default: run once (see open questions in
  [roadmap.md](roadmap.md)).

## Transport choice

The agent channel is a **single gRPC bidirectional stream over mTLS**.

- Outbound only from the agent, so it traverses NAT and firewalls.
- One persistent stream carries both directions: control plane pushes commands (sync
  jobs, run now, cancel), agent pushes heartbeats, run status, and log chunks.
- mTLS gives both transport encryption and agent identity in one mechanism.
- Typed contract via protobuf, which keeps agent and control plane in sync as the
  protocol evolves.

Alternative considered: a WebSocket channel with JSON messages. Simpler to start and
easy to proxy, but loses the typed contract and needs a hand-rolled message envelope.
gRPC is the recommendation; WebSocket is a reasonable fallback if gRPC infra proves
heavy for self-hosters. The protocol design in [agent-protocol.md](agent-protocol.md) is
transport-agnostic enough to swap if needed.

## Live logs to the browser

Agents push log chunks up their gRPC stream as a job runs. The control plane writes
them to Postgres (capped) and simultaneously fans them out to any subscribed browser
over SSE at `GET /api/v1/runs/:id/logs/stream`. SSE is chosen over WebSocket for the
browser side because it is one-directional (server to browser), auto-reconnects, and is
trivial to serve from Fiber.

Deployed process output (pm2 / journalctl / docker) is a separate path: operators call
`GET /api/v1/connectors/:id/objects/:ref/logs`, which dispatches a `ConnectorCommand`
with `op=logs` over the same mTLS stream. See [connectors.md](connectors.md).

Deploy process names are qualified as `<project>-<app>` (unless they already match) so
two projects that both ship a `web` app do not collide in pm2 or systemd.

## Deployment shape

- Control plane: one container/binary for the API (REST + gRPC), plus Postgres. Can run
  on a single small VM for self-hosting.
- Agent: distributed as a single binary plus an install script, or a system package.
  Runs under systemd as a dedicated unprivileged user.

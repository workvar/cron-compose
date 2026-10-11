# Architecture

> Visual maps: [Diagrams](Diagrams.md) · Design rationale: [Design Decisions](Design-Decisions.md)

![Runtime architecture](diagrams/runtime-architecture.png)

[Interactive runtime diagram](html/runtime-architecture.html) · Also available signed-in at `/app/architecture`.

## Goals

- Manage scheduled shell jobs across many Linux servers from one web UI.
- Work for servers behind NAT or home firewalls (a Raspberry Pi at home), not only boxes with public inbound access.
- Keep jobs running on a server even when the control plane or network is unavailable.
- Give clear, live visibility: run history, exit codes, durations, and streamed logs.
- Be self-hostable as a small set of services.

## Non-goals (for now)

- Windows **agents**. Linux and macOS only for job execution. The Windows installer can stand up a control plane.
- General CI/CD or container build orchestration.
- Multi-step DAGs / job dependencies (tracked on the [Roadmap](Roadmap.md)).
- Multi-tenant SaaS. Single-tenant self-host first.

## Topology

```
+---------------------------------------------------------------+
|                        Control Plane                          |
|                                                               |
|   Next.js UI  --REST/SSE-->  Fiber API  <-->  PostgreSQL      |
|                                  ^                            |
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

The agent is always the dialer. The control plane never needs inbound access to a target server. That is what makes a home Pi behind a consumer router work without port forwarding.

## Components

### Web UI (Next.js 16, App Router)

Server-rendered React 19 app with `basePath: /app`. Talks to the Fiber API over REST; subscribes to Server-Sent Events for live run logs. Screens: servers, job editor (script + schedule), run history, live run view, secrets, connectors, web terminal, settings.

In production the UI is **not** a public port. The control plane reverse-proxies `/app` to the internal Next.js process.

### Control-plane API (Go 1.25, Fiber v3)

One process, two listeners:

| Listener | Default | Who uses it |
|----------|---------|-------------|
| HTTP (`HTTP_ADDR`) | `:8080` | Browsers and REST clients. Serves `/api` and proxies `/app`. Bare `/` redirects to `/app`. |
| gRPC mTLS (`GRPC_ADDR`) | `:9090` | Agents only. End-to-end mTLS; do not HTTP-terminate this port. |

Responsibilities: authentication and RBAC, CRUD for servers/jobs/schedules, job versions, accepting run records and log chunks, fanning live logs to the UI over SSE, connector operations, terminal relay, and being the source of truth that agents sync from.

### PostgreSQL 16

Stores users, servers, jobs and job versions, schedules, runs, capped log output, encrypted secrets, API tokens, audit log, connectors, notifications, and retention state. See [Data Model](Data-Model.md).

### Agent (Go binary, one per server)

A single static binary. It:

- Enrolls once with a short-lived token, then authenticates with a client certificate.
- Maintains one outbound gRPC stream to the control plane.
- Keeps a local cache of assigned job definitions in an embedded store (bbolt) so it survives restarts and control-plane outages.
- Runs a local cron scheduler over those definitions (timezone-aware, `robfig/cron`).
- Executes each job (temp script file, chosen interpreter, stdout/stderr/exit/duration), enforces timeout, concurrency, and optional systemd resource limits.
- Streams logs and run status; buffers locally and replays on reconnect.

## The scheduling model

Two designs were considered:

**Option A — central scheduler.** The control plane evaluates cron and pushes “run now” at fire time. A control-plane outage or a network blip means missed runs. Bad fit for home Pis on flaky links.

**Option B — agent-local scheduler (chosen).** The control plane is the source of truth for job *definitions*. When a job is created or edited, the definition is synced to the agent. The agent stores it locally and evaluates cron itself. Jobs fire on time even if the control plane is down; run records and logs queue locally and upload on reconnect.

Consequences of Option B:

- Agents generate run IDs (ULIDs) locally; the control plane upserts by ID (idempotent).
- Schedules carry an IANA timezone; DST is handled on the machine that actually runs the job.
- A **catch-up policy** decides what happens after a long offline period: `once` (default), `all` (capped at 20), or `skip`.
- On-demand “run now” is still a command over the live stream.

## Transport

The agent channel is a **single gRPC bidirectional stream over mTLS**.

- Outbound only from the agent, so it traverses NAT and firewalls.
- One persistent stream carries both directions: control plane pushes commands (sync jobs, run now, cancel, connector ops, terminal, agent update); agent pushes heartbeats, run status, log chunks, and operation results.
- mTLS is both encryption and agent identity.
- Typed contract via protobuf (`proto/agent.proto`).

Live logs to the browser use **SSE** at `GET /api/v1/runs/:id/logs/stream` — one-directional, auto-reconnects, trivial to serve from Fiber. The web terminal uses **WebSocket** because it needs bidirectional byte-accurate I/O; see [Web Terminal](Web-Terminal.md).

## Deployment shape

- **Control plane:** one API process (REST + gRPC) plus Postgres, plus an internal Next.js process. Fits on a single small VM.
- **Agent:** single binary + install script, or `.deb` / `.apk`. Runs under systemd as a dedicated unprivileged user.

Only two ports are published in production (`docker-compose.prod.yml`): HTTP and gRPC. Postgres and the web container stay internal.

## Stack

| Layer | Technology |
|-------|------------|
| Frontend | Next.js 16 (App Router), React 19, TypeScript, xterm.js |
| Control-plane API | Go 1.25, Fiber v3, gRPC-Go |
| Database | PostgreSQL 16 |
| Agent | Go 1.25, bbolt, robfig/cron, creack/pty |
| CLI | Go 1.25, Cobra (`cc`) |
| Process manager (source install) | pm2 |
| Agent packages | nfpm → `.deb` and `.apk` for linux/amd64 and linux/arm64 |

## Repo layout

```
.
├── control-plane/     Go: Fiber REST + mTLS gRPC agent gateway
├── agent/             Go: per-server agent
├── web/               Next.js 16 App Router
├── cli/               cc CLI
├── proto/             agent.proto + generated Go
├── migrations/        SQL migrations
├── install/           from-source installer
├── packaging/         nfpm agent packages
├── docs/              design spec
└── docker-compose.prod.yml
```

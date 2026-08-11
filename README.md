# CronCompose

A web tool for writing, scheduling, and managing cron-style jobs across remote Linux
machines (Raspberry Pi, EC2, bare-metal, or any Linux box). You write a shell script
in the UI, set a schedule (for example "every 6 hours"), pick the server it runs on,
and CronCompose deploys it, runs it on time, and shows you live logs and run history.

## The shape of it

CronCompose has two halves:

1. **Control plane** (central): a Next.js web UI plus a Go (Fiber) API and a Postgres
   database. This is where you define servers, write jobs, set schedules, and read logs.
2. **Agent** (per server): a small Go binary installed on each target machine. It dials
   *out* to the control plane (so it works behind home routers, NAT, and firewalls),
   runs the jobs assigned to it on a local schedule, and streams results back.

The key design choice: the agent holds its own copy of every job and runs its own
scheduler. If the control plane goes down or the network drops, jobs keep firing on the
server and results sync back when the connection returns. The control plane is the
source of truth for *definitions*, not a runtime dependency for *execution*.

## Single entry point

The control plane is the single entry point for browsers: its HTTP port serves the
REST API under `/api` and reverse-proxies `/app` to the web UI (Next.js, `basePath:
/app`), redirecting the bare URL to `/app`. Agents connect to the control plane's
mTLS gRPC port directly. In production only those two ports are published in
[docker-compose.prod.yml](docker-compose.prod.yml); the web container stays internal.

## Documentation

This repo currently holds the design spec. Read in this order:

- [docs/architecture.md](docs/architecture.md) — system overview, components, how the
  pieces talk, and the scheduling model.
- [docs/data-model.md](docs/data-model.md) — entities and the Postgres schema.
- [docs/agent-protocol.md](docs/agent-protocol.md) — enrollment, the agent transport,
  job sync, execution, and log streaming.
- [docs/api.md](docs/api.md) — the REST API surface for the UI and external callers.
- [docs/security.md](docs/security.md) — auth, RBAC, secrets, agent trust, threat model.
- [docs/roadmap.md](docs/roadmap.md) — MVP scope, later phases, and open questions.

## Stack

- Frontend: Next.js 16 (App Router), React.
- Control-plane API: Go 1.25, Fiber v3 (REST for the UI), gRPC for the agent channel.
- Database: PostgreSQL.
- Agent: Go binary, embedded local store (SQLite or bbolt), local cron evaluation.

## Status

Design phase. No application code yet. The roadmap defines the MVP slice to build first.

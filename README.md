# CronCompose

Deploy apps and schedule shell jobs across remote Linux machines (Raspberry Pi, EC2,
bare-metal, or any Linux box). Define a project with `croncompose.yml` or the UI, pick a
server, and CronCompose clones, builds, starts the process manager, and streams status
back. Jobs keep their own agent-local schedules so work continues offline.

## The shape of it

CronCompose has two halves:

1. **Control plane** (central): a Next.js web UI plus a Go (Fiber) API and a Postgres
   database. This is where you manage servers, deploys, jobs, connectors, and secrets.
2. **Agent** (per server): a small Go binary on each target. It dials *out* to the control
   plane (so it works behind home routers, NAT, and firewalls), runs assigned jobs on a
   local schedule, executes deploys, and streams results back.

The agent holds its own copy of every job and runs its own scheduler. If the control plane
goes down or the network drops, jobs keep firing and results sync back when the connection
returns.

## Public site vs signed-in app

The UI lives under `/app` (`basePath: /app`). Marketing pages are public without a
session; the rest of the app requires sign-in.

| URL | Who | What |
|-----|-----|------|
| `/` | Anyone | Product landing (proxied to `/app/landing`) |
| `/app/use-cases` | Anyone | Use-case guides (fleet, deploys, jobs, connectors) |
| `/app/docs` | Anyone | `croncompose.yml` reference (short link: `/docs`) |
| `/app/login` | Anyone | Sign in / create account |
| `/app/*` (elsewhere) | Signed in | Dashboard, deploys, jobs, connectors, tools, … |

Anonymous visitors get a marketing header and footer. Signed-in users get the sidebar
shell. Opening login without a session cookie does not call `/me`, so you do not see a
spurious “missing session” error.

## Single entry point

The control plane is the public HTTP entry: REST under `/api`, UI under `/app`, and the
marketing landing at `/`. Agents connect to the control plane’s mTLS gRPC port. In
production only those two ports are published in
[docker-compose.prod.yml](docker-compose.prod.yml); the web container stays internal.

## Documentation

- In-product: [`/app/docs`](web/app/docs/page.tsx) — `croncompose.yml` reference.
- Design / ops specs in this repo:

  - [docs/architecture.md](docs/architecture.md) — topology, components, scheduling model
  - [docs/data-model.md](docs/data-model.md) — entities and Postgres schema
  - [docs/agent-protocol.md](docs/agent-protocol.md) — enrollment, stream, log streaming
  - [docs/api.md](docs/api.md) — REST surface
  - [docs/connectors.md](docs/connectors.md) — host service managers (pm2, systemd, …)
  - [docs/security.md](docs/security.md) — auth, RBAC, secrets, threat model
  - [docs/roadmap.md](docs/roadmap.md) — phases and open questions

Runtime diagram (signed-in): **Architecture** in the sidebar, or
[`/app/architecture`](web/app/architecture/page.tsx).

## Stack

- Frontend: Next.js 16 (App Router), React, light/dark theme
- Control-plane API: Go 1.25, Fiber v3 (REST), gRPC for the agent channel
- Database: PostgreSQL
- Agent: Go binary, local job cache, local cron evaluation

## Status

Actively developed. See [RELEASE_NOTES.md](RELEASE_NOTES.md) for the latest tagged
release.

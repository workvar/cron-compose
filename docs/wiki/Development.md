# Development

Phase 1–5 of the product are implemented: mTLS agent channel, SSE live logs, session auth + RBAC, secrets, connectors (nginx/systemd/Docker/pm2), notifications, web terminal.

## Prerequisites

- Go 1.25+
- Node 20+ (Next.js 16)
- PostgreSQL 16 — local server, or `make db-up` for Docker
- Only when regenerating proto: `protoc` plus `protoc-gen-go` and `protoc-gen-go-grpc`

## First run

```sh
make db-up        # start Postgres in Docker (skip if you run a local Postgres)
make migrate      # apply migrations
make tidy         # pull Go deps for all modules

export SESSION_SECRET="$(openssl rand -hex 32)"
export SEED_ADMIN_EMAIL="you@example.com"
export SEED_ADMIN_PASSWORD="change-me"

# control plane: REST on :8080, mTLS gRPC on :9090
make control-plane && ./control-plane/bin/control-plane

# web UI on :3000 under /app (another shell)
make web

# agent (another shell, on the target or the same box)
make agent
./agent/bin/agent enroll --token=<paste from UI>
./agent/bin/agent run
```

There is also `./setup.sh` / `make setup` for an interactive local setup.

## End-to-end flow

1. Open <http://localhost:3000/app>. In `next dev` go to `/app` directly — only the production control plane redirects `/` → `/app`. Sign in with the seeded admin.
2. **Add server**, copy the install command (one-time token).
3. On the target: `agent enroll --token=...` then `agent run`. The server row flips to `online`.
4. **New job**, write a script and a cron, save. Click **Run now**.
5. The run page opens an SSE stream and shows log chunks live.

## Make targets

| Target | Purpose |
|--------|---------|
| `db-up` / `db-down` | Postgres via `docker-compose.yml` |
| `migrate` | Apply `migrations/*.sql` with local `psql` |
| `migrate-tool` | Build the bundled migrate binary (no `psql` required) |
| `proto` / `proto-check` | Regenerate / CI-guard protobuf Go |
| `control-plane` / `agent` / `cli` | Build binaries |
| `web` | `npm install` + `next dev` |
| `tidy` | `go mod tidy` in each module |
| `pm2-start` / `pm2-restart` / `pm2-status` / `pm2-logs` | Supervise a built tree |
| `install` / `uninstall` | From-source installer wrappers |

## Auth notes for developers

- Seeded admin is upserted on **every** boot from `SEED_ADMIN_*` as `owner`.
- Cookie: `cc_session`.
- Open: `/healthz`, `/api/v1/auth/login`, `/api/v1/agents/enroll`.

## mTLS in dev

First start writes `ca.crt`, `ca.key`, `server.crt`, `server.key` under `TLS_DIR` (default `./tls`). Agents store `agent.key`, `agent.crt`, `ca.crt` under `DATA_DIR/tls/`. Subsequent starts load whatever is on disk.

## SSE live logs

`agentgw.LogBroker` fans `LogChunk`s to subscribers keyed by `run_id`. `GET /api/v1/runs/:id/logs/stream` writes a snapshot, then live `log` events, then `done`. The run page uses `EventSource` and closes on `done`.

## Config (dev defaults)

Control plane:

| Var | Default |
|-----|---------|
| `DATABASE_URL` | local dev DSN |
| `HTTP_ADDR` | `:8080` |
| `GRPC_ADDR` | `:9090` |
| `TLS_DIR` | `./tls` |
| `TLS_HOSTS` | `localhost,127.0.0.1` |
| `SESSION_SECRET` | **must be set** in anything non-toy |
| `PUBLIC_HTTP_URL` | `http://localhost:8080/api/v1` |
| `PUBLIC_GRPC_ADDR` | `localhost:9090` |

Agent:

| Var | Default |
|-----|---------|
| `CONTROL_PLANE_ADDR` | `localhost:9090` |
| `CONTROL_PLANE_HTTP` | `http://localhost:8080/api/v1` |
| `CONTROL_PLANE_SNI` | `localhost` |
| `DATA_DIR` | `/var/lib/croncompose` |

`API_BASE` is **inlined at Next.js build time** by `next.config.ts`. Set it for `npm run build`, not only at runtime.

## CI

[`.github/workflows/ci.yml`](https://github.com/workvar/cron-compose/blob/main/.github/workflows/ci.yml) on `main` and PRs:

- Proto freshness + complete `go.sum` for `proto`, `control-plane`, `agent`, `cli`
- `go vet`, `go test -race`, `go build` per module
- Control-plane REST integration test against Postgres 16
- Web TypeScript check (`tsc --noEmit`) on Node 20

Tag `v*` runs [release.yml](https://github.com/workvar/cron-compose/blob/main/.github/workflows/release.yml): static `linux/amd64` and `linux/arm64` agent binaries plus `.deb` / `.apk`.

## Repo layout

```
.
├── DEVELOPMENT.md
├── docs/
├── proto/
├── migrations/
├── control-plane/           Fiber REST + mTLS gRPC
│   └── internal/
│       ├── api/             router, auth gating, web proxy
│       ├── auth/            password, session, RBAC, OIDC
│       ├── agentenroll/     REST enroll (signs CSRs)
│       ├── agentgw/         gRPC, registry, log broker, terminal, connectors
│       ├── pki/             CA + server cert + CSR signing
│       ├── jobs/, runs/, servers/, secrets/, notify/, connectors/
├── agent/
│   └── internal/
│       ├── enroll/, mtls/, store/, transport/
│       ├── scheduler/       timezone-aware local cron
│       ├── executor/        timeout + log capture
│       ├── connectors/      nginx, systemd, docker, pm2
│       ├── terminal/        PTY sessions
│       └── runtime/         top-level loop
├── web/                     Next.js 16, basePath /app
└── cli/                     cc
```

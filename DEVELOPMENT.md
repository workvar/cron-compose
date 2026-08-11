# Development

Phase 1 MVP with the security and live-data passes applied. Full end-to-end flow works:
mTLS for the agent channel, SSE for live logs, session auth + RBAC on the REST API.

## Prereqs

- Go 1.25+
- Node 20+ (Next.js 16)
- Docker (for the Postgres dev instance)
- Only when regenerating proto: `protoc` plus the Go plugins
  - `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest`
  - `go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest`

## First run

```sh
make db-up        # start Postgres
make migrate      # apply migrations/0001_init.sql, 0002_agent_token.sql
make tidy         # pull Go deps for all three modules

# Set required + recommended env vars for the control plane
export SESSION_SECRET="$(openssl rand -hex 32)"
export SEED_ADMIN_EMAIL="you@example.com"
export SEED_ADMIN_PASSWORD="change-me"
# Optional: where to keep the PKI material. Defaults to ./tls.
# export TLS_DIR="./tls"

# control plane: REST on :8080, mTLS gRPC on :9090
make control-plane && ./control-plane/bin/control-plane

# web UI on :3000 under /app (in another shell)
make web

# agent (in another shell, on the target server)
make agent
./agent/bin/agent enroll --token=<paste from UI>
./agent/bin/agent run
```

End-to-end flow:

1. Open <http://localhost:3000/app>. The UI lives under `/app` (Next.js `basePath`); bare
   `/` is only redirected there by the control plane in prod, so in `next dev` go to `/app`
   directly. You'll be bounced to `/app/login` (the middleware sees no session). Sign in
   with the seeded admin.
2. **Add server**, copy the install command (with a one-time token).
3. On the target: `agent enroll --token=...` generates an ed25519 keypair + CSR, POSTs
   it to the control plane's REST enroll endpoint, receives a signed client cert + the
   CA, and persists everything under `DATA_DIR/tls/`. The server row flips to `online`.
4. `agent run` dials the gRPC endpoint over mTLS (client cert presented and verified),
   opens `AgentStream`, and immediately receives the initial `SyncJobs`.
5. From the server detail page, click **New job**, write a script and a cron, save.
6. Click **Run now**. The run page opens an SSE stream and shows log chunks live. The
   "done" event closes the stream when the run finishes.

## Auth + RBAC

- **Seeded admin**: the control plane upserts the user identified by
  `SEED_ADMIN_EMAIL` / `SEED_ADMIN_PASSWORD` on every boot. Role is `owner`.
- **Session cookie**: `cc_session`, signed HMAC-SHA256 with `SESSION_SECRET`, 7-day TTL,
  `HttpOnly`. (In prod set `Secure: true` and serve via TLS.)
- **Roles**, lowest to highest: `viewer`, `operator`, `admin`, `owner`. Comparison is
  numeric rank, so a route gated at `operator` admits operator/admin/owner.
- **Route gating** today:
  - All GETs under `/api/v1` require at least `viewer`.
  - Job mutations (create, edit, enable/disable, run now) require `operator`.
  - Server mutations (create, edit, delete, issue token) require `admin`.
- **Open endpoints**: `/healthz`, `/api/v1/auth/login`, `/api/v1/agents/enroll`.
  The last carries its own one-time token; agents never have a user session.

## mTLS

- The control plane bootstraps a self-signed CA + server cert under `TLS_DIR` (default
  `./tls`) on first start: `ca.crt`, `ca.key`, `server.crt`, `server.key`. Subsequent
  starts load whatever is on disk.
- Agents generate their own keypair on enroll, send a CSR to
  `POST /api/v1/agents/enroll`, and receive a signed client cert + the CA. Files land in
  `DATA_DIR/tls/`: `agent.key`, `agent.crt`, `ca.crt`.
- The gRPC listener requires and verifies the client cert
  (`tls.RequireAndVerifyClientCert`).
- `AgentStream` authenticates by SHA-256 fingerprint of the peer cert, looked up against
  `servers.cert_fingerprint` (set on enroll).
- Production: swap the self-signed CA for one from your real PKI by overwriting the
  files in `TLS_DIR`. The gateway just loads whatever is there.

## SSE live logs

- `agentgw.LogBroker` fans incoming `LogChunk`s out to any subscribers keyed by
  `run_id`.
- `GET /api/v1/runs/:id/logs/stream` writes Server-Sent Events: first the already-
  persisted log lines as a snapshot, then live `log` events, then a `done` event with
  the final status.
- The web app's run page uses `EventSource` and closes on `done`. No polling.

## Config (env vars)

| Var                       | Default                                     | Notes                                |
|---------------------------|---------------------------------------------|--------------------------------------|
| `DATABASE_URL`            | local dev DSN                                | required for non-dev                 |
| `HTTP_ADDR`               | `:8080`                                      | REST listener                        |
| `GRPC_ADDR`               | `:9090`                                      | mTLS agent listener                  |
| `TLS_DIR`                 | `./tls`                                      | CA + server cert path                |
| `TLS_HOSTS`               | `localhost,127.0.0.1`                        | SANs on the server cert              |
| `SESSION_SECRET`          | dev placeholder                              | **must be set** in prod, min 16 chars |
| `SEED_ADMIN_EMAIL`        | unset                                        | upserts on every boot                |
| `SEED_ADMIN_PASSWORD`     | unset                                        | upserts on every boot                |
| `PUBLIC_BASE_URL`         | unset                                        | single source of truth for the external address. When set (e.g. `https://cron.example.com`), derives `PUBLIC_HTTP_URL`, `PUBLIC_GRPC_ADDR` (host + gRPC port), the OIDC redirect, and adds its host to `TLS_HOSTS`. |
| `PUBLIC_HTTP_URL`         | `http://localhost:8080/api/v1`               | derived from `PUBLIC_BASE_URL`; set explicitly to override |
| `PUBLIC_GRPC_ADDR`        | `localhost:9090`                             | derived from `PUBLIC_BASE_URL`; set explicitly to override |
| `LOG_LEVEL`               | `info`                                       | `debug`/`info`/`warn`/`error`        |

Agent:

| Var                       | Default                                | Notes                                  |
|---------------------------|----------------------------------------|----------------------------------------|
| `CONTROL_PLANE_ADDR`      | `localhost:9090`                       | mTLS gRPC                              |
| `CONTROL_PLANE_HTTP`      | `http://localhost:8080/api/v1`         | for the one-time enroll REST call      |
| `CONTROL_PLANE_SNI`       | `localhost`                            | server name to verify against in TLS   |
| `DATA_DIR`                | `/var/lib/croncompose`                 | identity + tls + jobs cache live here  |

## What's deferred to later phases

- Persistent agent-side buffer/replay for very long offline periods (the broker drops
  events for slow SSE subscribers, but agent->control plane is in-process channel only).
- Secrets table + AES-GCM envelope encryption.
- Audit log surfaced in the UI.
- Per-run cancel (current cancel cancels per-job).
- API tokens for programmatic access.

## Repo layout

```
.
├── README.md
├── DEVELOPMENT.md           <- you are here
├── docs/                    <- design spec
├── proto/                   <- agent.proto + generated agent/v1/*.pb.go
├── migrations/              <- SQL migrations
├── docker-compose.yml
├── Makefile
├── control-plane/           <- Go: Fiber REST + mTLS gRPC agent gateway
│   ├── cmd/server/
│   └── internal/
│       ├── api/             <- router with auth gating
│       ├── auth/            <- password, session, RBAC middleware, /auth handlers
│       ├── agentenroll/     <- REST endpoint that signs CSRs
│       ├── agentgw/         <- gRPC service, registry, log broker, sync helpers
│       ├── pki/             <- CA + server cert + CSR signing
│       ├── config/, db/, ids/, logger/
│       ├── servers/, jobs/, runs/
├── agent/                   <- Go: per-server agent binary
│   ├── cmd/agent/
│   └── internal/
│       ├── config/, identity/, store/, transport/
│       ├── enroll/          <- REST client for the enroll call
│       ├── mtls/            <- keypair + CSR + tls.Config loader
│       ├── scheduler/       <- timezone-aware local cron
│       ├── executor/        <- script runner with timeout + log capture
│       └── runtime/         <- top-level loop: sync, fire, push events
└── web/                     <- Next.js 16 App Router
    ├── app/
    │   ├── login/           <- /login form
    │   ├── servers/[id]/    <- server detail + new job
    │   ├── jobs/[id]/       <- job detail + Run now
    │   └── runs/[id]/       <- run detail with SSE
    ├── components/          <- Nav, ServerCard, JobRow, RunRow, LogoutButton
    ├── lib/                 <- api client (forwards session cookie), types
    ├── next.config.ts       <- standalone build + basePath /app + /api -> control plane rewrite
    └── middleware.ts        <- redirect unauthenticated users to /login (paths are basePath-relative)
```

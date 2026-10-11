# Deployment

How to run CronCompose in production. Install steps are on [Installation](Installation.md); this page is topology, TLS, configuration, and day-two operations.

## Topology

The **control plane is the single entry point**. It exposes two ports:

```
                       :8080  (HTTP, public)        :9090  (gRPC mTLS, public)
  browsers / REST ───────────►┐                          ▲
                              │  control plane            │
                  /app  ──────┼──► web UI (Next.js, internal :3000)
                  /api  ──────┘    REST API (/api -> /api/v1)
                                                          │
  agents ─────────────────────────────────────────────────┘  (dial out only)

                          control plane ──► Postgres (internal)
```

- **HTTP** (`HTTP_ADDR`, default `:8080`) serves REST under `/api` and reverse-proxies `/app` to Next.js via `WEB_UPSTREAM`. `/` redirects to `/app`.
- **gRPC** (`GRPC_ADDR`, default `:9090`) is the agent mTLS endpoint. Agents never accept inbound connections.
- **Web UI** binds loopback (source install) or stays on the Compose network.
- **Postgres** is the only stateful dependency.

You need a public DNS name that both browsers and agents can resolve. Browsers and agents may use **different** names if HTTP is tunneled and gRPC is not (for example Cloudflare Tunnel for `/app` + a raw TCP hostname for agents).

## Configuration

The control plane reads the environment. Source installs use a `0600` `.env` at the repo root (written by the installer, read by `ecosystem.config.js`). Compose uses `docker-compose.prod.yml`’s `x-env` block or a `.env` beside it. Start from [`.env.example`](https://github.com/workvar/cron-compose/blob/main/.env.example).

`PUBLIC_BASE_URL` is the easiest knob: set it once (`https://cc.example.com`) and the control plane derives the public REST URL, advertised gRPC address, OIDC redirect, and a TLS SAN. Compose instead sets `PUBLIC_HTTP_URL` and `PUBLIC_GRPC_ADDR` explicitly.

### Critical variables

| Variable | Purpose |
|----------|---------|
| `DATABASE_URL` | Postgres DSN. Required. |
| `HTTP_ADDR` / `GRPC_ADDR` | Listeners. Defaults `:8080` / `:9090`. |
| `WEB_UPSTREAM` | Internal Next.js address. Empty = API-only. |
| `PUBLIC_BASE_URL` | External address; derives the rest. |
| `SESSION_SECRET` | HMAC for cookies. Required, 16+ chars. `openssl rand -hex 32`. |
| `SECRETS_MASTER_KEY` | 32-byte hex wrapping stored secrets. **Set in prod.** Back it up. |
| `SEED_ADMIN_EMAIL` / `SEED_ADMIN_PASSWORD` | Bootstrap admin; upserted **every boot**. |
| `TLS_DIR` / `TLS_HOSTS` | Agent-facing CA + server cert. Persist `TLS_DIR`. SANs must include the name agents dial. |
| `OIDC_*` | Optional SSO. Prompted by `./install/install.sh --advanced`, or set in `.env` and restart. See [OIDC SSO](Operations.md#oidc-sso). |
| `METRICS_TOKEN` | If set, `/metrics` requires `Authorization: Bearer`. |
| `RUN_LOG_MAX_BYTES` | Per-run log cap. Default 5 MiB. |
| `RETENTION_*_DAYS` | Prune windows. `0` (default) never prunes. |
| `AGENT_UPDATE_*` | Pinned agent self-update. All three of version, URL, SHA-256 required, or GitHub polling via `GITHUB_RELEASE_REPO`. |

Generate secrets before first boot:

```sh
openssl rand -hex 32   # SESSION_SECRET
openssl rand -hex 32   # SECRETS_MASTER_KEY
```

Restart after any `.env` change — pm2 and Compose cache the environment.

## TLS and PKI

**Browser / REST.** Control plane speaks cleartext HTTP. Put nginx, Caddy, or a cloud LB in front:

```nginx
server {
  listen 443 ssl;
  server_name cc.example.com;
  ssl_certificate     /etc/ssl/cc.example.com.crt;
  ssl_certificate_key /etc/ssl/cc.example.com.key;

  location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $remote_addr;
    proxy_set_header X-Forwarded-Proto $scheme;
  }
}
```

**Agents.** End-to-end mTLS. **Do not terminate TLS on the gRPC port.** Load balancer = L4/TCP passthrough.

Changing the public name: update `PUBLIC_BASE_URL` (or the explicit public URLs) and `TLS_HOSTS`, delete `TLS_DIR` so the server cert regenerates with the new SAN, restart, and re-enroll agents if the CA changed.

## Database and migrations

Migrations live in [`migrations/`](https://github.com/workvar/cron-compose/tree/main/migrations) and are applied by the bundled `migrate` tool (`schema_migrations` table, safe to re-run).

**Always migrate before starting or upgrading the control plane.** Take a `pg_dump -Fc` backup first.

- Compose: `docker compose -f docker-compose.prod.yml run --rm --no-deps --entrypoint /usr/local/bin/migrate control-plane -dir /migrations`
- Source: `control-plane/bin/migrate -dir migrations`

There is no automatic down-migration. Rollback = restore the dump and roll the binary back together. See [`docs/upgrades.md`](https://github.com/workvar/cron-compose/blob/main/docs/upgrades.md).

`./update.sh` performs backup-friendly rebuild → migrate → restart for both Compose and source.

## Retention

Two independent controls:

- **Per-run cap** (`RUN_LOG_MAX_BYTES`) stops one runaway job filling the database. The script keeps running; only stored log tail is dropped. Live SSE still sees everything.
- **Retention pruner** (off by default) sweeps hourly in 5000-row batches. A sensible start:

```
RETENTION_RUN_LOG_DAYS=14
RETENTION_RUN_DAYS=90
RETENTION_AUDIT_DAYS=365
RETENTION_OPERATION_DAYS=365
```

Keep run rows longer than their logs. Watch `cc_retention_deleted_total` and `cc_run_log_bytes_total`.

## Health

- `GET /healthz` — liveness plus a Postgres ping. Use for container and LB checks.
- `GET /metrics` — Prometheus. See [Operations](Operations.md).

## Agent self-update

Off unless `AGENT_UPDATE_VERSION`, `AGENT_UPDATE_URL`, and `AGENT_UPDATE_SHA256` are all set (or GitHub release polling is configured). The agent verifies the digest as it streams, keeps `<name>.old`, swaps atomically, exits 0 for the supervisor. On a box with no supervisor, set `AGENT_SELF_UPDATE=0` on the agent and update by hand.

## Hardening

Use the checklist on [Security](Security.md). Persist Postgres and `TLS_DIR`. Do not expose 5432. Do not expose the Next.js port; only the control plane’s HTTP and gRPC ports should be reachable.

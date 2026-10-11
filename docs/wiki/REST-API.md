# REST API

The Fiber v3 control plane exposes JSON under `/api/v1` for the Next.js UI and external callers. **Agents do not use this API** — they use the gRPC channel in [Agent Protocol](Agent-Protocol.md).

- Browser: session cookie. External callers: bearer `api_token`.
- Live run logs: SSE. Web terminal: WebSocket (not listed below).
- Mutating endpoints write an `audit_log` entry.
- Pagination: `?limit=&cursor=` → `{ items, next_cursor }`.
- Errors: `{ error: { code, message } }`.
- Timestamps: RFC 3339 in UTC.

Paths below are relative to `/api/v1`.

## Auth

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/auth/login` | Email + password, sets session |
| POST | `/auth/logout` | Clears session |
| GET | `/auth/config` | Whether OIDC is enabled (login page) |
| GET | `/auth/oidc/start` | Redirect to IdP |
| GET | `/auth/oidc/callback` | OIDC callback |
| GET | `/me` | Current user and role |

## Servers

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/servers` | List with status and last seen |
| POST | `/servers` | Create; returns enrollment token + install command |
| GET | `/servers/:id` | Detail |
| PATCH | `/servers/:id` | Rename, description, labels |
| DELETE | `/servers/:id` | Remove |
| POST | `/servers/:id/enrollment-token` | Re-issue one-time token |
| POST | `/servers/:id/update` | Offer connected agent a self-update (admin) |
| POST | `/servers/:id/revoke` | Revoke agent cert; force re-enroll |
| GET | `/updates` | Latest agent release and per-server status |

## Jobs

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/jobs?server=:id` | List, optional server filter |
| POST | `/jobs` | Create (version 1) |
| GET | `/jobs/:id` | Detail including current version |
| PATCH | `/jobs/:id` | Metadata/schedule/options; editing `script_body` creates a new version |
| DELETE | `/jobs/:id` | Delete |
| POST | `/jobs/:id/enable` | Enable scheduling |
| POST | `/jobs/:id/disable` | Disable scheduling |
| POST | `/jobs/:id/run` | Manual run now (`RunNow` to agent) |
| GET | `/jobs/:id/versions` | Version history |
| GET | `/jobs/:id/versions/:n` | Specific version’s script |
| POST | `/jobs/:id/versions/:n/restore` | Make an old version current (inserts a new version from it) |

## Runs and logs

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/jobs/:id/runs` | History (paginated) |
| GET | `/runs/:id` | Status, timing, exit code |
| GET | `/runs/:id/logs` | Full captured log for finished runs |
| GET | `/runs/:id/logs/stream` | **SSE** live log stream |
| POST | `/runs/:id/cancel` | Cancel (`CancelRun`) |

## Secrets, tokens, audit

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/secrets` | Names and scopes (never values) |
| POST | `/secrets` | Create; value encrypted at rest |
| DELETE | `/secrets/:id` | Delete |
| GET | `/api-tokens` | List caller’s tokens (no secrets) |
| POST | `/api-tokens` | Create; plaintext shown once |
| DELETE | `/api-tokens/:id` | Revoke |
| GET | `/audit` | Paginated, filterable |

## Job templates

| Method | Path | Role | Notes |
|--------|------|------|-------|
| GET | `/job-templates?category=` | viewer | Built-ins sort first |
| GET | `/job-templates/:id` | viewer | |
| POST | `/job-templates` | operator | Full body, or `{from_job_id, name}` |
| DELETE | `/job-templates/:id` | operator | `403 builtin` for shipped templates |

## Notification targets

Admin throughout (rows hold SMTP credentials and webhook URLs).

| Method | Path | Notes |
|--------|------|-------|
| GET | `/notification-targets` | Secret config values come back as `********` |
| POST | `/notification-targets` | `kind`: webhook, slack, or email |
| PATCH | `/notification-targets/:id` | `********` keeps the stored value |
| POST | `/notification-targets/:id/test` | Real delivery; HTTP 200 either way, read `delivered` |
| DELETE | `/notification-targets/:id` | |

`server_labels` scopes to servers whose labels contain all given pairs; empty = whole fleet. `on_statuses` limits which run outcomes fire; empty = every non-success.

## Connectors

Read: any authenticated role. Lifecycle: operator+. Config bytes: admin (read and write).

| Method | Path | Role |
|--------|------|------|
| GET | `/connectors` | viewer |
| GET | `/connectors/:id` | viewer |
| GET | `/connectors/:id/resources` | viewer |
| GET | `/connectors/:id/operations?limit=` | viewer |
| GET | `/servers/:id/connectors` | viewer |
| POST | `/connectors/:id/actions` | operator — `{action, ref}`: start, stop, restart, reload, enable, disable |
| GET | `/connectors/:id/config?path=` | admin |
| POST | `/connectors/:id/config` | admin — `{path, content, base_checksum, dry_run}` |
| GET | `/connectors/:id/snapshots?path=&limit=` | admin |
| POST | `/connectors/:id/snapshots/:snapshotID/restore` | admin |

Mutating calls return `{operation_id, status, message, steps}`. Read `status` (`succeeded`, `failed`, `invalid`, `unauthorized`, `unsupported`, or transport `offline` / `timeout`) — a 200 with `status: "invalid"` means the change was refused. Transport failures: `503 agent_offline`, `504 agent_timeout`.

`base_checksum` on apply is the checksum from the read it was based on. Mismatch is `invalid` rather than clobbering. Pass empty to force.

## Outside `/api/v1`

| Path | Purpose |
|------|---------|
| `GET /healthz` | Liveness + Postgres ping |
| `GET /metrics` | Prometheus (optionally bearer-gated) |
| `GET /app/*` | Proxied web UI |
| `WS /api/servers/:id/terminal/ws` | Web terminal (session cookie, admin/owner) |

Full tables and request bodies: [`docs/api.md`](https://github.com/workvar/cron-compose/blob/main/docs/api.md).

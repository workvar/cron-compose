# REST API

The Fiber v3 control plane exposes a REST API under `/api/v1` for the Next.js UI and for
external callers. Agents do **not** use this API; they use the gRPC channel in
[agent-protocol.md](agent-protocol.md).

- Auth: browser uses a session cookie; external callers use a bearer `api_token`.
- Content type: JSON.
- Live data (run logs, status) uses SSE endpoints noted below.
- All mutating endpoints write an `audit_log` entry.

## Auth

| Method | Path                | Purpose                              |
|--------|---------------------|--------------------------------------|
| POST   | `/auth/login`       | Email + password, sets session.      |
| POST   | `/auth/logout`      | Clears session.                      |
| GET    | `/me`               | Current user and role.               |

## Servers

| Method | Path                              | Purpose                                  |
|--------|-----------------------------------|------------------------------------------|
| GET    | `/servers`                        | List servers with status and last seen.  |
| POST   | `/servers`                        | Create a server, returns enrollment token + install command. |
| GET    | `/servers/:id`                    | Server detail.                           |
| PATCH  | `/servers/:id`                    | Rename, edit description/labels.         |
| DELETE | `/servers/:id`                    | Remove server (and its jobs).            |
| POST   | `/servers/:id/enrollment-token`   | Re-issue a one-time enrollment token.    |
| POST   | `/servers/:id/update`             | Offer a connected agent a self-update to the latest release (admin). |
| GET    | `/updates`                        | Latest agent release and per-server update status. |
| POST   | `/updates/check`                 | Re-poll GitHub immediately and return update status (admin). |
| POST   | `/servers/:id/revoke`             | Revoke the agent cert; forces re-enroll. |

## Jobs

| Method | Path                  | Purpose                                          |
|--------|-----------------------|--------------------------------------------------|
| GET    | `/jobs?server=:id`    | List jobs, optionally filtered by server.        |
| POST   | `/jobs`               | Create a job (creates version 1). Body: server_id, name, interpreter, script_body, schedule_cron, timezone, options. |
| GET    | `/jobs/:id`           | Job detail incl. current version.                |
| PATCH  | `/jobs/:id`           | Edit metadata/schedule/options. Editing `script_body` creates a new `job_version`. |
| DELETE | `/jobs/:id`           | Delete a job.                                    |
| POST   | `/jobs/:id/enable`    | Enable scheduling.                              |
| POST   | `/jobs/:id/disable`   | Disable scheduling.                            |
| POST   | `/jobs/:id/run`       | Trigger a manual run now (sends `RunNow` to agent). |

## Job versions

| Method | Path                          | Purpose                            |
|--------|-------------------------------|------------------------------------|
| GET    | `/jobs/:id/versions`          | List version history.              |
| GET    | `/jobs/:id/versions/:n`       | Get a specific version's script.   |
| POST   | `/jobs/:id/versions/:n/restore` | Make an old version current (creates a new version from it). |

## Runs and logs

| Method | Path                          | Purpose                                  |
|--------|-------------------------------|------------------------------------------|
| GET    | `/jobs/:id/runs`              | Run history for a job (paginated).       |
| GET    | `/runs/:id`                   | Run detail: status, timing, exit code.   |
| GET    | `/runs/:id/logs`             | Full captured log (text), for finished runs. |
| GET    | `/runs/:id/logs/stream`      | **SSE** live log stream for an in-progress run. |
| POST   | `/runs/:id/cancel`           | Cancel a running job (sends `CancelRun`). |

## Secrets

| Method | Path             | Purpose                                          |
|--------|------------------|--------------------------------------------------|
| GET    | `/secrets`       | List secret names and scopes (never values).     |
| POST   | `/secrets`       | Create a secret (name, scope, value). Value is encrypted at rest. |
| DELETE | `/secrets/:id`   | Delete a secret.                                 |

## API tokens

| Method | Path                 | Purpose                                  |
|--------|----------------------|------------------------------------------|
| GET    | `/api-tokens`        | List the caller's tokens (no secrets).   |
| POST   | `/api-tokens`        | Create a token; plaintext shown once.    |
| DELETE | `/api-tokens/:id`    | Revoke a token.                          |

## Audit

| Method | Path        | Purpose                              |
|--------|-------------|--------------------------------------|
| GET    | `/audit`    | Paginated, filterable audit entries. |

## Conventions

- Pagination: `?limit=&cursor=` with an opaque cursor; list responses return
  `{ items: [...], next_cursor }`.
- Errors: JSON `{ error: { code, message } }` with appropriate HTTP status.
- Timestamps: RFC 3339 / ISO 8601 in UTC.
- Idempotency: run upserts from agents are keyed by run `id`; the manual-run endpoint
  may accept an `Idempotency-Key` header to avoid duplicate triggers.

## Connectors

Read endpoints are open to any authenticated role. Lifecycle is operator and above.
Anything touching config bytes is admin, for reading as well as writing.

| Method | Path | Role | Notes |
|---|---|---|---|
| GET | `/connectors` | viewer | Every connector across the fleet. |
| GET | `/connectors/:id` | viewer | |
| GET | `/connectors/:id/resources` | viewer | Objects and config files. |
| GET | `/connectors/:id/operations?limit=` | viewer | Append-only command history. |
| GET | `/servers/:id/connectors` | viewer | |
| POST | `/connectors/:id/actions` | operator | `{action, ref}`. Action must be one of start, stop, restart, reload, enable, disable. |
| GET | `/connectors/:id/config?path=` | admin | `{path, content, checksum}`. |
| POST | `/connectors/:id/config` | admin | `{path, content, base_checksum, dry_run}`. |
| GET | `/connectors/:id/snapshots?path=&limit=` | admin | Backup history, without the bytes. |
| POST | `/connectors/:id/snapshots/:snapshotID/restore` | admin | |

Mutating calls return `{operation_id, status, message, steps}`. `status` is the agent's
own verdict (`succeeded`, `failed`, `invalid`, `unauthorized`, `unsupported`) or a
transport outcome the control plane recorded (`offline`, `timeout`). A 200 with
`status: "invalid"` means the request was handled and the change was refused, so callers
have to read `status` rather than only the HTTP code.

Transport failures do use HTTP codes: `503 agent_offline`, `504 agent_timeout`. Both
still carry `operation_id`, so the UI can link to the recorded attempt.

`base_checksum` on an apply is the checksum from the read it was based on. A mismatch is
refused with `invalid` rather than overwriting a change made in the meantime. Pass it
empty to force.

## Job templates

| Method | Path | Role | Notes |
|---|---|---|---|
| GET | `/job-templates?category=` | viewer | Built-ins sort first within a category. |
| GET | `/job-templates/:id` | viewer | |
| POST | `/job-templates` | operator | Either the full body, or `{from_job_id, name}` to save an existing job. |
| DELETE | `/job-templates/:id` | operator | `403 builtin` for shipped templates. |

## Notification targets

Admin throughout: these rows hold SMTP credentials and webhook URLs.

| Method | Path | Notes |
|---|---|---|
| GET | `/notification-targets` | Secret config values come back as `********`. |
| POST | `/notification-targets` | `{name, kind, url, config, server_labels, on_statuses}`. `kind` is webhook, slack or email. |
| PATCH | `/notification-targets/:id` | Any subset. A config value sent as `********` keeps its stored value. |
| POST | `/notification-targets/:id/test` | Sends a real message. Returns `{delivered: true}` or `{delivered: false, error}` with a 200 either way: the request succeeded, the delivery is what failed. |
| DELETE | `/notification-targets/:id` | |

`server_labels` scopes a target to servers whose labels contain all of the given pairs;
empty means the whole fleet. `on_statuses` limits which run outcomes fire it; empty
means every non-success outcome.

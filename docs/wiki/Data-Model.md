# Data model

> Run status map: [Job run status lifecycle](Diagrams.md#job-run-status-lifecycle) · Definition sync: [Job definition and log data flow](Diagrams.md#job-definition-and-log-data-flow) · [ADR-004 — Immutable job versions](Design-Decisions.md#adr-004-immutable-job-versions)

PostgreSQL. IDs are ULIDs stored as `text` (sortable, agent-generatable). Timestamps are `timestamptz`. JSON is `jsonb`. Spec: [`docs/data-model.md`](https://github.com/workvar/cron-compose/blob/main/docs/data-model.md). Schema evolves via numbered files in [`migrations/`](https://github.com/workvar/cron-compose/tree/main/migrations).

Multi-tenancy is deferred. One deployment is one tenant; `org_id` can be added later.

## Entities

| Entity | Role |
|--------|------|
| **user** | Person who logs in (email, password hash or SSO-only, role) |
| **server** | Target machine, 1:1 with an agent (`pending` / `online` / `offline`, cert fingerprint, labels, last seen) |
| **enrollment_token** | One-time secret (hash only) to bind an agent to a server |
| **job** | Schedule + options + target (`server` or `labels`) + pointer to current version |
| **job_version** | Immutable snapshot of script and non-secret env. Edits insert a new row |
| **secret** | Encrypted value (`value_enc`), scoped global / server / job |
| **run** | One execution: trigger, status, timing, exit code. ID may be agent-generated |
| **run_log** | Chunked stdout/stderr, capped per run |
| **api_token** | Programmatic access (hash only) |
| **audit_log** | Who did what |
| **notification_target** | Webhook / Slack / email, optional label and status filters |
| **job_template** | Built-in or user-saved starting points for the wizard |
| **connector** / **connector_operation** / snapshots | Portal management of nginx, systemd, Docker, pm2 |

## Key choices

- **Job versions are immutable.** Editing a script never mutates a row. A run always points at the exact `job_version_id` it executed.
- **Run IDs can be agent-generated.** Offline agents upload later; the control plane upserts by `id`.
- **Logs live in Postgres** up to `RUN_LOG_MAX_BYTES`, then a truncation marker. Object storage is not built.
- **Secrets store ciphertext only.** Decrypt at sync time over mTLS. See [Security](Security.md).
- **Label targeting.** `jobs.server_id` may be null; `target_kind = 'labels'` plus jsonb selector, resolved at sync and run-now time. Deleting a server SET NULL rather than cascading those jobs.

## Run statuses

`pending` | `running` | `succeeded` | `failed` | `timed_out` | `canceled` | `skipped`

Triggers: `schedule` | `manual` | `api`.

## Migrations (index)

| File | What it added |
|------|----------------|
| `0001_init.sql` | Phase 1 schema |
| `0002_agent_token.sql` | Legacy bearer column (mTLS uses fingerprint) |
| `0003_notifications.sql` | Webhook targets |
| `0004_labels_and_limits.sql` | Label targeting, CPU/memory columns |
| `0005_connectors.sql` | Connector registry |
| `0006_connector_ops.sql` | Connector operations |
| `0007_notify_channels.sql` | Slack / email channels |
| `0008_job_templates.sql` | Templates + six built-ins |
| `0009_job_limits.sql` | Extra job resource limits |
| `0010_retention_indexes.sql` | Indexes for the retention pruner |
| … | … |
| `0023_deploy_run_metrics.sql` | `deploy_runs.metrics` jsonb (phase timings, release size) |

Apply with the bundled migrate tool **before** rolling the control plane. No automatic down-migration — restore from `pg_dump`.

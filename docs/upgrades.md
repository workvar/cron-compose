# Upgrades

Migrations live in `migrations/` and are applied automatically:

1. **`./update.sh`** (and the in-app Updates flow that runs it) applies pending
   SQL after rebuilding, before restart.
2. **Control plane boot** applies any still-pending files whenever Postgres is
   reachable (idempotent via `schema_migrations`).

You should not need to run `make migrate` by hand for a normal upgrade: update
the control plane, then update agents from the UI. This doc still describes each
migration and how to roll forward safely if you are debugging.

## Before any migration

1. **Take a database backup.** Self-hosters: `pg_dump -Fc croncompose > before.dump`.
2. **Skim the migration's SQL** so you know what's changing.
3. **Apply on a staging copy first** if you can. Migrations here use `IF NOT EXISTS`
   and `IF EXISTS` where it's safe, so re-running on a freshly-restored DB is fine.
4. **Preferred path:** update control plane (migrations run on update + boot) →
   update agents from the UI → they reconnect with the new binary. Manual
   `make migrate` is only a fallback.

## Migration index

### `0022_deploy_run_as_user.sql` — deploy as OS account

Adds `deploy_projects.run_as_user` so a project can clone/install/start as `pi`,
`root`, etc. Empty keeps the previous “agent’s own user” behavior. Applied
automatically on control-plane update/boot.

### `0001_init.sql` — initial schema

The full Phase 1 schema: `users`, `servers`, `enrollment_tokens`, `jobs`,
`job_versions`, `secrets`, `runs`, `run_logs`, `api_tokens`, `audit_log`. Indexes
on `runs (job_id, created_at desc)` and `runs (server_id, status)`. Safe to apply
on an empty database.

### `0002_agent_token.sql` — bearer agent token

Adds `servers.agent_token_hash`. The bearer-token enrollment path was an MVP
shortcut; mTLS uses `servers.cert_fingerprint` instead. The column is still
present for backward compatibility with deployments that haven't moved to mTLS
yet. Idempotent.

### `0003_notifications.sql` — webhook targets

Adds `notification_targets` (id, name, kind='webhook', url, enabled). Empty by
default; create one with `POST /api/v1/notification-targets` (admin) or
`cc` (when that subcommand exists). Idempotent.

### `0004_labels_and_limits.sql` — label targeting + resource limits

Larger change. Does three things:

1. **`jobs.server_id` becomes nullable** and the foreign key changes from
   `ON DELETE CASCADE` to `ON DELETE SET NULL`, so deleting a server no longer
   deletes label-targeted jobs that happened to mention it.
2. Adds `jobs.target_kind` (default `'server'`) and `jobs.target_labels` (jsonb,
   default `{}`). Existing jobs keep working unchanged because every row defaults
   to `target_kind = 'server'` and they all have a `server_id`.
3. Adds `jobs.cpu_quota_percent` and `jobs.memory_max_mb` (both default 0 = no
   limit). Adds a GIN index on `target_labels` for fast selector matches.

**Compatibility note.** An older control-plane binary querying `BuildFullSync` will
fail after this migration because the new SQL selects the new columns. Apply this
migration in the SAME deploy window as the control-plane upgrade.

## Required env after each upgrade

Whenever you upgrade the control plane, verify these are set in the environment:

| Variable                  | When introduced | Required?                           |
|---------------------------|-----------------|-------------------------------------|
| `DATABASE_URL`            | from day 1      | yes                                 |
| `SESSION_SECRET`          | auth + RBAC     | yes, min 16 chars, hex-rand in prod |
| `SEED_ADMIN_EMAIL`/`...PASSWORD` | auth + RBAC | recommended (otherwise no one can log in on first run) |
| `TLS_DIR` / `TLS_HOSTS`   | mTLS            | optional; defaults to `./tls`       |
| `SECRETS_MASTER_KEY`      | secrets         | yes, 32-byte hex, REAL value in prod (default is a clearly-marked dev key) |
| `PUBLIC_HTTP_URL` / `PUBLIC_GRPC_ADDR` / `INSTALL_SCRIPT_URL` | deployment | recommended so install commands point at the right host |

## Rolling back

There is no automatic down-migration. Recovery path is restore-from-backup:

```sh
psql croncompose -c "drop schema public cascade; create schema public;"
pg_restore -d croncompose before.dump
```

Roll the control-plane binary back to the matching version at the same time.

## When you write a new migration

- Number it `NNNN_short_name.sql` in lexicographic order so update/boot pick it
  up after the previous ones.
- Use `IF NOT EXISTS` / `IF EXISTS` for `create` / `drop` so reruns are safe.
- Wrap structural changes in `begin; ... commit;`.
- If the migration changes columns the control plane reads, ship the migration
  and the control-plane upgrade in the same deploy window.
- Update this doc's "Migration index" section with one sentence per change.

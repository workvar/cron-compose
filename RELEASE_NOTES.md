# CronCompose v0.0.47

Shareable deploy configs, per-process pre-start scripts, deploy analytics, and
cleaner routing so logged-in users land on the app instead of the marketing page.

## Highlights

- **Share / Import YAML** — Download or copy a project’s `croncompose.yml`, then
  import it on New Deploy (edit, then deploy). Project menus and Share match the
  familiar deploy-platform pattern.
- **Pre-start scripts** — Each process can run a `pre_start` command after
  release and before start (for example `npx prisma migrate deploy`). Shown in
  the deploy board and progress stream.
- **Deploy analytics** — Agents report phase timings and release size on finish.
  Project and dashboard views show duration/size trends, success rate, and
  phase averages (with duration fallback when metrics are missing).
- **Root routing** — Bare `/` sends signed-in users to `/app/` and signed-out
  users to the public landing, so the marketing site no longer renders inside
  the AppShell.

## Upgrade

1. Apply migration **0023** (`deploy_runs.metrics` JSONB).
2. Update **control-plane**, **agent**, and **web** together (proto + metrics
   stream + UI). Older agents still finish deploys; analytics stay empty until
   agents are upgraded.

# CronCompose v0.0.50

Failed deploys no longer look like they are still running, and process
controls stay usable even when the agent never registered the process.

## Highlights

- **Failed progress stays failed** — When a run ends as failed, canceled, or
  agent offline (including the deploy-budget timeout), no step stays active.
  The summary reads **Failed · …** (or **Canceled · …**) and the progress bar
  turns red instead of green mid-percent.
- **Start when the process never came up** — Deploy cards and the project
  Processes table show **Start** when the process is missing or stopped.
  Pause / Resume appear only when it is actually running or frozen.
- **Controls without a live inventory hit** — Stop, restart, and start work
  against the process name when the deploy dies before the start step, as long
  as the server has a pm2, systemd, or docker connector.

## Upgrade

1. Update **web** (progress freeze and process controls). Agent and
   control-plane unchanged for this release.
2. No database migration.

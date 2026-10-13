# CronCompose v0.0.57

pm2 ecosystem names drive the Process name field and the deploy wait path, with a
clearer full-width run board.

## Highlights

- **Ecosystem process name** — On deploy, `ecosystem.config.js` `apps[].name`
  wins over a computed `project-app` token. The control plane no longer sends
  that default for pm2, so waits match names like `peepal-proxy`.
- **Auto-populate + edit sync** — Inspect fills the Process name field from the
  ecosystem file and shows an “auto-populated” hint. Editing the field updates
  `apps[].name` in the ecosystem file on the next deploy.
- **pm2 wait reliability** — Each `pm2 jlist` call has a short timeout (no more
  forever hangs), with `pm2 pid` as a fallback. Start skips restart when the
  process is already online after Run.
- **Run board layout** — Waves are full width (no longer capped by the wizard
  `deploy-flow` max-width). The Server terminal is its own full-width row above
  the per-process terminals.

## Upgrade

1. Update **agent**, **control plane**, and **web**.
2. Restart or reinstall the agent on each target (ecosystem name sync + jlist
   timeout). Restart the control plane so deploys omit computed pm2 names.
3. Existing projects: open Process manager, confirm Process name (re-detect if
   empty), save, then redeploy.
4. No new database migration.

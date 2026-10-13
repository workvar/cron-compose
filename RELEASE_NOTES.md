# CronCompose v0.0.56

pm2 ecosystem files drive process names end-to-end, and the deploy run page shows
one terminal per process with a clearer wave flow.

## Highlights

- **Ecosystem autopopulate** — Inspect reads `ecosystem.config.*` / `pm2.json` and
  fills process manager, process name, run script, and port. Multi-cwd ecosystems
  seed one block per app. `process_name` is stored and sent to the agent.
- **pm2 wait uses the real name** — The agent prefers `apps[].name` (e.g.
  `peepal-proxy`), starts with `--only`, and matches aliases / cwd so waits no
  longer time out looking for `peepal-rp-proxy` while pm2 registered something
  else. Parent-folder ecosystems (monorepo `current/`) are found too.
- **One terminal per process** — The run board shows Server plus each app’s own
  terminal. Phase tabs (install, build, run, start, …) filter that process’s log.
- **Wave flow with arrows** — Waves are a horizontal task graph: arrows between
  waves, parallel tasks side by side. Click a task to jump to that process and
  phase.

## Upgrade

1. Update **agent**, **control plane**, and **web**.
2. Restart or reinstall the agent on each target (ecosystem name wait / `--only`).
   Restart the control plane so inspect returns `pm2_apps` and deploys send
   `process_name`.
3. Existing projects: re-detect the app folder or set **Process name** on the
   Process manager tab, then save — or rely on the agent’s ecosystem parse at
   start time.
4. No new database migration.

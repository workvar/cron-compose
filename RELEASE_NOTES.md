# CronCompose v0.0.51

Drag-and-drop deploy waves so Proxy can come up before Backend migrations,
plus a choice to stop or continue when a wave fails.

## Highlights

- **Deploy order** — On Processes (2+ apps), drag processes into waves. Same
  wave runs pre-start / start / health together; later waves wait. Presets:
  **All together**, **One by one**, and **+ Wave**. Install/build still run in
  parallel for every app.
- **If a wave fails** — **Stop the deploy** (default) ends the run. **Continue
  to the next wave** skips the rest of the failed wave and still starts later
  services; the run is marked failed if any wave failed. Within a wave,
  sibling processes still finish together.
- **Process controls after budget timeouts** — Start / Stop / Restart resolve
  the server’s pm2, systemd, or docker connector even when the process never
  registered, and show why controls are blocked when no connector is present.

## Upgrade

1. Apply migration `0024_on_wave_failure.sql` (adds `on_wave_failure` on
   `deploy_projects`, default `stop`).
2. Update **control-plane**, **agent**, and **web** together (`start_group` and
   `on_wave_failure` travel on the deploy command).
3. Restart or reinstall the agent on each target.

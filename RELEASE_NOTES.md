# CronCompose v0.0.54

Run a process and wait until it is actually up before the next wave starts.
Deploy logs keep streaming, and process controls work without a discovered
pm2 connector.

## Highlights

- **Run step** — Edit → Flow has a **Run** chip between Pre-start and Start.
  Run starts the app (the run script, or pm2, systemd, or Docker when one is
  set), streams its output, and holds the next wave until the process is
  online and, when a port is set, accepting connections. Put Proxy’s Run in
  an earlier wave than Backend’s Install when the API must be up first. A
  later Start confirms the supervisor.
- **Logs stay live** — After pm2, systemd, or Docker daemonizes, the agent
  tails that process for the rest of the deploy, including the name an
  `ecosystem.config.js` registered. The deploy log budget is 32MB, so a long
  pre-start script is not cut off mid-output.
- **Process controls** — Start, Pause, Stop, and Restart run as the project’s
  deploy user, so an nvm or fnm pm2 works even when discovery never registered
  a connector. A live run has **Cancel deploy**.
- **Waiting vs in progress** — A process card stays Waiting until one of its
  own steps is actually running.

## Upgrade

1. Update **agent**, **control plane**, and **web**.
2. Restart or reinstall the agent on each target, and restart the control
   plane (cancel and process-action routes).
3. No new database migration.
4. Open the project → Flow, put Proxy’s Run ahead of the other apps, set
   Proxy’s port, save, and redeploy. A deploy already in flight keeps the
   previous behavior until that agent is updated; cancel it after the upgrade.

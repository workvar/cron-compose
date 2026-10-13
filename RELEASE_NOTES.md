# CronCompose v0.0.55

Live wave board on each deploy, Restart redeploys a full install→run chain for
one app, and the agent stays online during long deploys.

## Highlights

- **Wave visualizer** — The run page shows each flow wave with per-task progress
  and completion. Click a task to open its log. Output is in tabs (Server,
  Proxy, Backend, Frontend) instead of stacked terminals only.
- **Restart = full chain** — Restart on a process card starts a new deploy for
  that app only: install → build → cleanup → pre-start → run → start. Start,
  Pause, and Stop still drive the process manager.
- **pm2 status after Run** — Ecosystem names such as `peepal-proxy` match the
  Proxy app (by name suffix or cwd). After Run or Start finishes, the agent
  rediscovers connectors so the card can show online.
- **Agent stays connected** — The control plane accepts the agent’s 20s
  keepalive pings. The previous default treated them as abuse and closed the
  stream about once a minute (`too_many_pings`), which showed as agent offline
  mid-deploy. A healthy link also resets reconnect backoff.
- **Dashboard root** — Signed-in redirects use `/app` (no trailing slash) so
  soft navigation to the dashboard root no longer 404s.

## Upgrade

1. Update **agent**, **control plane**, and **web**.
2. Restart the control plane (keepalive enforcement + scoped redeploy). Restart
   or reinstall the agent on each target (connector rediscover after Run).
3. No new database migration.

# CronCompose v0.0.10

Agent and stack updates now say what they are actually doing — fetching the
release, downloading images and packages, building, stopping the server,
restarting — with a determinate progress bar. The server Delete action is an
icon-only trash control so it matches the rest of the header.

## Highlights

- **Live update stages** — The full-screen stack overlay and the per-agent
  update panel no longer stall on "Building…" / "Restarting…". The agent
  streams named stages (`UpdateProgress` on the gRPC stream) as `update.sh`
  runs: git fetch/checkout, Docker image / npm downloads, compiles, migrations,
  stopping services, restart. `GET /updates` exposes the current stage so a
  page refresh does not lose the story. When the control plane itself goes
  down mid-stack-update, the overlay switches to "Stopping the server" then
  "Restarting services" until the new version answers.
- **Delete server, restyled** — The server detail header keeps a destructive
  delete, but it is now a white icon button with a red trash glyph (no "Delete"
  label). Confirmation is unchanged.

## Upgrade notes

### Control plane (source install)

From Settings → Updates, click **Update** on this host. Or by hand:

```sh
cd cron-compose
git fetch --tags
git checkout --force v0.0.10
./update.sh --no-pull
```

No new database migrations in this release.

### Agent (Linux / macOS)

This release changes the agent protocol (a new `UpdateProgress` message). Older
agents still apply updates; they just cannot narrate stages, so the UI falls
back to the coarser online/offline signals. A stack update rebuilds the local
agent automatically. Standalone agents pick it up the next time they self-update,
or reinstall:

```sh
curl -sSL https://github.com/workvar/cron-compose/releases/latest/download/install-agent.sh | \
  sudo TOKEN=<token> \
       CONTROL_PLANE_HTTP=https://<host>/api \
       CONTROL_PLANE_ADDR=<host>:9090 \
       bash
```

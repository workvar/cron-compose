# CronCompose v0.0.49

Pause, stop, and restart each deploy process from the live run board —
without leaving the page or kicking off a full redeploy.

## Highlights

- **Process controls on deploy cards** — Each Proxy / Backend / Frontend-style
  card on a deploy run shows **Pause**, **Stop**, and **Restart**. Pause
  toggles to **Resume** when the process is already frozen. Controls light up
  once the process is registered on the host (or the app step is done/failed).
- **Same actions on the project Processes table** — Day-2 ops use the same
  compact control row, with start / reload / delete kept beside it for pm2
  and friends.
- **Pause across managers** — Docker uses `pause` / `unpause`, systemd uses
  cgroup `freeze` / `thaw`, and pm2 freezes with `SIGSTOP` / `SIGCONT` so the
  process stays in the list (unlike stop).

## Upgrade

1. Update **agent** and **web** together (pause verbs live in the agent;
   card UI in web). Control-plane unchanged for this release.
2. Restart or reinstall the agent on each target so `pause` / `unpause` are
   accepted.
3. No database migration.

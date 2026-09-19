# CronCompose v0.0.16

Import folder picker and owner filter polish, clearer git API errors, and agent
root access that leaves “waiting” once the agent settles.

## Highlights

- **Agent root polling** — After enabling root access, the UI polls until euid
  matches (or times out with a clear error) instead of staying on
  **On (waiting)** after a reload.
- **Simpler folder picker** — Drill-down browse, workspace chips, optional full-tree
  search, and a typed path. Failed `/git/dirs` calls show a short message instead of
  dumping Cloudflare HTML.
- **Owner filter always visible** — Account / org chips (with an **All** option)
  appear whenever owners are known, including a single personal account.
- **Root path normalize** — Leading `/` is stripped (`/backend` → `backend`) so
  project block roots stay consistent.

## Upgrade notes

### Control plane (source install)

From Settings → Updates, click **Update** on this host. Or by hand:

```sh
cd cron-compose
git fetch --tags
git checkout --force v0.0.16
./update.sh --no-pull
```

No new migrations in this release.

### Agent (Linux / macOS)

No agent changes required for this release. Update from the UI as usual when a
newer agent tag is offered.

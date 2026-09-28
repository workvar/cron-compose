# CronCompose v0.0.18

Monorepo import polish: each app can own its health check and language detect,
plus a searchable branch picker and language icons on the repo list.

## Highlights

- **Per-app health checks** — A project block can set its own probe path/port/
  timeout, overriding the shared Advanced health check. Unset apps still use the
  project-wide check. `croncompose.yml` accepts `health` under each app; export
  writes it back. The agent probes each app's own check when set.
- **Folder-scoped detect** — Changing a block's root re-runs language/install
  detection for that subfolder only (`DetectAt`), so a Go API under `backend/`
  is not mislabeled because of a sibling `package.json`. Manual framework picks
  and blocks loaded from an explicit `croncompose.yml` are left alone.
- **Searchable branch picker** — Configure uses a searchable select fed by
  `GET /git/branches` instead of a free-text field. The default branch is
  labeled; custom names are still allowed.
- **Language icons** — The import repo list shows GitHub's reported language
  logo when available. Framework and process-manager fields on each block use
  searchable selects with icons.
- **Folder picker polish** — Clearer browse UX for picking an app root.

## API

- `GET /git/branches?provider=&repo=` — lists branches (`name`, `default`).
- `GET /git/inspect` — optional `path=` scopes detection to a monorepo
  subfolder; `croncompose.yml` is only applied on an unscoped (root) inspect.
- `GET /git/repos` — GitHub listings include `language` when the API reports it.
- Deploy apps accept per-app `health` (path / port / timeout); unset falls back
  to the project-level health check.

## Upgrade notes

### Control plane (source install)

From Settings → Updates, click **Update** on this host. Or by hand:

```sh
cd cron-compose
git fetch --tags
git checkout --force v0.0.18
./update.sh --no-pull
```

No new migrations in this release.

### Agent (Linux / macOS)

Update agents so per-app health probes take effect. Older agents ignore
`DeployApp.health` and keep using the project-level check only. Update from the
UI as usual when the newer agent tag is offered.

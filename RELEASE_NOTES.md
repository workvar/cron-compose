# CronCompose v0.0.15

Import git can deploy multiple apps from one monorepo: browse folders (lazy or full
tree), configure each as its own project block, and filter repos by owner/search.

## Highlights

- **Project blocks** — On Import → Build, add one or more apps. Each block has its
  own root folder, language, install script, port, and process manager. Runtime
  keeps shared server / branch / clone path and per-app env.
- **Browseable roots (hybrid)** — New `GET /git/dirs` lists directories (shallow by
  default; optional recursive list capped at 2000). Folder picker supports drill-down,
  detected workspace chips, **Load full tree** search, and a manual path escape hatch.
- **Repo picker UX** — Provider uses the searchable select; filter by personal
  account / org chips and search repositories by name or description.

## Upgrade notes

### Control plane (source install)

From Settings → Updates, click **Update** on this host. Or by hand:

```sh
cd cron-compose
git fetch --tags
git checkout --force v0.0.15
./update.sh --no-pull
```

No new migrations in this release.

### Agent (Linux / macOS)

No agent changes required for this release. Update from the UI as usual when a
newer agent tag is offered.

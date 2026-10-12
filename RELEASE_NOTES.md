# CronCompose v0.0.48

A clearer deploy folder picker, calmer agent online status between heartbeats,
and visible loading on fields that are still fetching repo data.

## Highlights

- **Finder-style folder picker** — Browse the repo as an expandable tree with
  macOS-style folder icons. Root folders show first; children load on expand.
  Shortcuts, filter, and an optional typed path stay out of the way. Selected
  path and **Use this folder** sit on one aligned row.
- **Branch switches refresh the tree** — Changing the default branch re-inspects
  the repo, clears folder caches, and reloads workspaces / `croncompose.yml`
  for that branch instead of leaving stale paths on screen.
- **Awaiting fields** — Selects, buttons, and the folder picker show a forest-green
  border sweep and a wait cursor while branches, specs, detection, or directories
  load in the background.
- **Offline grace** — Agents no longer flip to **offline** the instant a stream
  drops. Status stays online for 90s so brief reconnects and proxy idle cuts do
  not look like an outage. Heartbeats run every 15s with gRPC keepalive.

## Upgrade

1. Update **control-plane**, **agent**, and **web** together (status grace lives
   in the gateway; heartbeats/keepalive in the agent; picker UI in web).
2. No database migration.

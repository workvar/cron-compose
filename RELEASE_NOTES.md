# CronCompose v0.0.63

Deploy run progress no longer marks a real `go build` (or other build half) as
**Skipped** when install and build run in separate flow waves.

## Highlights

- **Flow install/build tracking** — When the agent logs only the install half
  of a script (e.g. `go mod download`), the live rail keeps the planned build
  step and activates it when the build wave runs, instead of replacing the
  step list and leaving **Build: Skipped**.
- **Wave completion** — `install: done` closes only the active half; pending
  build waits for its own wave. Cleanup/release still finish any remaining
  command steps.

## Upgrade

1. Rebuild/restart the **web UI**.
2. Redeploy a project whose flow has separate Install and Build waves — Build
   should show **Done** when the compile command runs.
3. Agent and control plane updates are **not** required for this release.

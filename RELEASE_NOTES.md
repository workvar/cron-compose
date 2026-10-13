# CronCompose v0.0.52

Phase-level deploy flow (install / build / cleanup across apps), a dedicated
Flow tab, stuck progress and process-control fixes, and non-TTY installs so
pnpm `approve-builds` no longer hangs the run.

## Highlights

- **Flow tab** — Edit / New Deploy → **Flow** (next to Redeploy). Column
  drag-and-drop for tasks like Proxy · Install, Backend · Build, and Cleanup.
  Same column runs together; later columns wait. Empty waves must be filled or
  deleted before save. Old projects get a default flow from their scripts.
- **Cross-app phase order** — Build of one app can wait for install of another.
  The agent splits install scripts into install vs build and runs the saved
  `flow` waves; release still activates between cleanup and pre-start.
- **Progress rail** — When cleanup starts (or `install: done` arrives), prior
  Installing / Building steps mark complete even if the start line was missed.
  Cleanup / pre-start percent keeps moving while logs stream.
- **Process controls** — Start, Pause, Stop, and Restart stay visible on run
  cards. Controls unlock once the process exists on the host (or start has
  begun), not only when the whole card is done.
- **pnpm approve-builds** — Deploy installs run over pipes (not a PTY), so
  stdin is non-TTY. Unapproved builds fail or warn instead of hanging on the
  interactive picker. Still approve builds in the repo and commit
  `allowBuilds` for a clean install.

## Upgrade

1. Apply migration `0025_deploy_flow.sql` (adds `flow` JSONB on
   `deploy_projects`).
2. Update **control-plane**, **agent**, and **web** together (`flow` travels on
   the deploy command; progress and controls are web-side).
3. Restart or reinstall the agent on each target.

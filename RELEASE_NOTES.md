# CronCompose v0.0.32

Deploy creation starts with a searchable server grid, redeploy can target a
branch, tag, or release, and the review plan breaks each app directory into
install → build → run steps. The installer also wires pm2 boot resurrection,
and the web terminal can open fullscreen in a new tab.

## Highlights

- **Server-first deploy flow** — pick a target from a searchable grid (emoji +
  name + status), then import and configure. Later steps show a clear
  “Deploying to” chip.
- **Green server emojis** — assign a monochrome emoji on the server page
  (`labels.emoji`) so hosts are easy to spot in lists and the deploy wizard.
- **Redeploy from branch, tag, or release** — searchable ref picker on the
  project page (`GET /git/refs`). Reminders after env or settings edits that a
  redeploy is required for the agent to apply them.
- **Hierarchical deploy review** — with a `croncompose.yml`, Confirm shows
  prepare → per-app Build (install/build) → activate release → per-app Run
  (env/start/health), matching how the agent works.
- **pm2 survives reboot** — `install.sh` runs `pm2 startup` + `pm2 save`
  (sudo once); `./croncompose-ctl.sh boot` remains for repair.
- **Fullscreen terminal** — Open in new tab / New tab opens a chrome-less
  session at `/servers/:id/terminal/full`.

## Upgrade

Pull or rebuild the control plane, web UI, and (for boot persistence) re-run
the installer or `./croncompose-ctl.sh boot` on existing hosts. No database
migration is required. Agent binaries do not need an update for these UI and
install changes.

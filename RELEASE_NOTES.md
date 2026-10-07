# CronCompose v0.0.36

Deploy and install toolchains as a chosen OS account (for example `pi` instead of
the agent service user), so login PATH tools like nvm’s `npm` work without
hand-editing systemd. A new Tools sidebar page detects and installs Node, Go,
Python, pm2, and friends per account. Updates apply migrations on control-plane
boot and refresh pm2 startup so the stack comes back after a reboot.

## Highlights

- **Deploy as** — Projects can set `run_as_user` (Advanced tab / Edit). The agent
  clones, installs, and starts as that account, checking binaries via its login
  shell PATH. Non-root accounts use `~/opt/…` and `~/tmp` instead of system
  `/opt` and `/tmp`.
- **Tools** — Sidebar → Tools: pick a server and OS account, scan what is
  installed, and install node (nvm), go, python, pm2, git, yarn, pnpm, or bun
  for that user.
- **Auto-migrate on boot** — When Postgres is reachable, the control plane
  applies pending SQL on startup (in addition to `update.sh`). No manual
  `make migrate` for a normal upgrade.
- **pm2 boot on update** — `update.sh` re-runs `pm2 save` and `pm2 startup`
  (passwordless sudo / root) after restart so agents and the stack resurrect
  after a reboot. `croncompose-ctl.sh` restart/start/reload also save the dump.
- **Migration `0022_deploy_run_as_user.sql`** — Adds `deploy_projects.run_as_user`
  (empty = previous agent-user behavior).

## Upgrade

Update the **control plane** (Updates UI or `./update.sh`), then update
**agents** from the UI. Migration `0022` applies automatically. Afterward, use
Tools to install Node for `pi` if needed, and set Deploy as → `pi` on projects
that should use that account’s toolchain.

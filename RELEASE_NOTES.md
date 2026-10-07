# CronCompose v0.0.37

Fixes deploy-as-user preflight failing to see `npm` (and similar) when the account
installs Node via nvm. Non-interactive `bash -lc` skips most of `~/.bashrc`, so
nvm never loaded even though an interactive SSH session as `pi` found `npm`.

## Highlights

- **nvm / fnm / asdf on deploy PATH** — Agent preflight, install scripts, and the
  Tools detector now source a toolchain prelude (nvm.sh, latest nvm node bin,
  fnm, asdf, `~/.local/bin`, bun, local go) before `command -v` and installs.
- Clearer preflight error when a binary is still missing for the chosen user.

## Upgrade

Update the **agent** (and control plane if you want matching notes). No new
migration. Redeploy with Deploy as → `pi` after the agent is on v0.0.37.

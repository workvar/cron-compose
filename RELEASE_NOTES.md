# CronCompose v0.0.43

Process managers installed under nvm (or similar) now start correctly during
deploy, and the confirm / post-deploy screens use the full content width.

## Highlights

- **PM2 (and friends) on start PATH** — Tools and preflight already found
  `pm2` via the deploy user's login shell, but `runCmdAs` still exec'd against
  the agent process PATH. Starting an app then failed with
  `executable file not found in $PATH` even when Tools showed PM2 installed.
  Deploy commands now resolve binaries the same way as Tools, and prepend the
  binary's directory so siblings like `npm` stay visible to `pm2 start`.
- **Full-width confirm deploy** — Confirm deploy and the post-deploy success
  panels span the main content area instead of the previous 860px column.

## Upgrade

Update the **agent** (required for the PM2 start fix) and **web** (layout).
No control-plane change and no new migration. Restart the agent, then redeploy
the project that failed at Starting.

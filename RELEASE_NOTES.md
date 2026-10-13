# CronCompose v0.0.58

Tools cards and a logo loader polish the UI; pm2 under nvm lists processes again;
the bare domain always shows the marketing landing.

## Highlights

- **Toolchain cards** — Tools page toolchains are equal-height 3-column cards with
  stacked Uninstall / Install (or Reinstall) rows and a full-width search bar.
- **Brand loader** — Logo-inspired loading mark (orbit, C draw, cascading slabs)
  used for route loading and common page placeholders across the app.
- **Uniform deploy terminals** — Per-process and Server terminal logs share a fixed
  280px height so Proxy / Backend / Frontend cards align regardless of log length.
- **pm2 + nvm PATH** — Agent pm2 commands (`jlist`, `save`, lifecycle) run through
  the toolchain prelude so `#!/usr/bin/env node` finds nvm Node. Fixes empty PM2
  lists and `env: 'node': No such file or directory` on save/startup.
- **Site root landing** — `cron.workvar.com/` (and bare `/`) always redirect to
  `/app/landing`, signed in or out. Visiting `/login` with a valid session hard-
  navigates to `/app` instead of soft-routing to a 404.

## Upgrade

1. Update **agent**, **control plane**, and **web**.
2. Restart or reinstall the agent on each target (pm2 toolchain PATH). Restart the
   control plane so `/` redirects to `/app/landing`.
3. Hard-refresh the UI (or clear cache) for Tools cards and the brand loader.
4. No new database migration.

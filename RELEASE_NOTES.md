# CronCompose v0.0.59

Deploy start/install now honors workdir `.env` ports and keeps listen aliases in
sync with the UI PORT field.

## Highlights

- **Workdir `.env` on start** — Install and process start merge `.env` from the
  app directory (and parents). Deploy/UI values still win over the file.
- **Listen port aliases** — Setting PORT in the project UI also updates existing
  `PROXY_PORT`, `HTTP_PORT`, `APP_PORT`, `LISTEN_PORT`, and `SERVER_PORT` in that
  env. Fixes Peepal-style proxies that read `PROXY_PORT` and were still binding
  `:80` (`bind: permission denied`) after the UI port was set to e.g. 3004.

## Upgrade

1. Update the **agent** on each target (restart or reinstall).
2. Confirm the app’s UI PORT (and host `.env` if you keep one), then redeploy or
   Stop → Start the process so pm2 picks up the new env.
3. No control-plane or database migration required for this release.

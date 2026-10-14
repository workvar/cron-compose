# CronCompose v0.0.64

Redeploy restarts PM2/systemd into the new release, and deploy env (e.g.
`FRONTEND_PORT`) survives the releases layout instead of going missing or
getting overwritten.

## Highlights

- **PM2 restart on deploy** — If a process is already online from a previous
  release, the agent deletes it and starts fresh under `current/`. Skipping
  start left `pm_cwd` on a pruned release and caused `MODULE_NOT_FOUND` /
  crash loops with no app changes.
- **systemd restart** — After `enable --now`, the unit is restarted so an
  already-active service picks up the new release.
- **Stable `.env` on disk** — Project and per-app env are written into the
  release and project base `.env`. Gitignored env that only lived in an old
  checkout is no longer lost after migrate/prune.
- **Monorepo listen ports** — Starting the frontend no longer rewrites shared
  `PROXY_PORT` when `FRONTEND_PORT` / `BACKEND_PORT` are present.
- **Deploy env wins** — Child process env overlays replace agent keys instead
  of appending duplicates that `getenv` could ignore.

## Upgrade

1. Update/rebuild the **agent** on each deploy host (required).
2. Rebuild/restart the control plane only if you pull this tag there too.
3. Redeploy affected projects (or `pm2 delete` + start once) so processes run
   from `current/` with the written `.env`.
4. Confirm in CronCompose project Env (or `/opt/.../<project>/.env`) that
   `FRONTEND_PORT` / `BACKEND_PORT` / `PROXY_PORT` are set as you expect.

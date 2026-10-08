# CronCompose v0.0.39

Leaner deploys: clone into tmp, cherry-pick only each process root into
`/opt/apps/…/releases`, build there, clean up source, start the process manager,
then delete the tmp clone. Also fixes noisy npm spinner logs, silent failures on
the live run page, and the cramped project Edit UI.

## Highlights

- **Cherry-pick deploy pipeline** — Full repo clones land in a disposable tmp
  directory. Only each app’s root folder (e.g. `web/`) is copied into the
  release under the project path. The tmp clone is removed when the run ends.
- **Cleanup after build** — New `cleanup` field on apps / `croncompose.yml`.
  Curated framework presets fill it automatically (drop `.git`, caches, source
  trees that are not needed at runtime).
- **pm2 / systemd boot persistence** — After start, deploys run `pm2 save` and
  best-effort `pm2 startup`; systemd user units keep `enable --now` and try
  `loginctl enable-linger`.
- **Readable install logs** — ANSI spinner frames from npm PTYs are stripped on
  the agent and in the UI (`CI` / `NO_COLOR` / `npm_config_progress=false` cut
  most of it at the source).
- **Failure reason on live runs** — SSE `done` includes `error`; the agent logs
  `FAILED — …` and the run page refetches so “failed” is never blank.
- **Deploy edit modal** — Project Edit opens a centered modal (Target / Build /
  Health) instead of nesting a form in the page header next to Redeploy.

## Upgrade

Update the **control plane**, **agent**, and **web**. No new migration. Existing
projects pick up framework cleanup defaults on the next deploy; set `cleanup`
explicitly in the Build & run tab or `croncompose.yml` to override.

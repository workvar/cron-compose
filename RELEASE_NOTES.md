# CronCompose v0.0.44

Public marketing pages (landing, use cases, docs chrome), process logs from
connectors, and stable project-qualified process names — plus an updated
runtime architecture diagram.

## Highlights

- **Public site chrome** — Signed-out visitors get a marketing header/footer on
  landing, `/app/use-cases`, and `/app/docs`, with short links `/docs` and
  `/use-cases`. The product landing lives at `/` (proxied to `/app/landing`).
  Signed-in users still see the normal app shell on docs.
- **Quiet login** — Opening `/login` without a `cc_session` cookie no longer
  probes `/me`, so the spurious “missing session” error is gone.
- **Process logs** — Operators can pull recent pm2, journalctl, or docker logs
  for a connector object (`GET /connectors/:id/objects/:ref/logs`) from the
  project and Connectors UI without SSHing to the host.
- **Qualified process names** — Deployed pm2/systemd names are
  `<project>-<app>` when the names differ, so two projects that both ship
  `web` do not collide on one host.
- **Architecture refresh** — The in-app Architecture tab embeds an updated
  Archify diagram covering the public surface, deploy path, and connector logs.
- **Docs** — README, `docs/architecture.md`, `docs/connectors.md`, and the
  in-product `croncompose.yml` reference describe the public routes, process
  naming, and object logs.

## Upgrade

Update **control-plane**, **agent**, and **web**. No new migration. Restart the
control plane and agents after deploy so connector log handlers and process-name
qualification are live.

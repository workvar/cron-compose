# CronCompose v0.0.53

Start one process before installing or building another. Flow ordering is
per-app, so Proxy can come up while Backend is still compiling.

## Highlights

- **Early start across apps** — In Edit → Flow, you can place **Start** for one
  process before **Install** / **Build** for another. The release activates on
  the first start (or pre-start / health) wave; later installs and builds
  continue in the live release.
- **Clearer rules** — Within one process, install → build → cleanup → start
  still applies. A single wave still cannot mix install/build/cleanup with
  start — put start in the next column.
- **Docs & UI copy** — Flow tab and croncompose.yml docs describe early start
  and the same-wave restriction.

## Upgrade

1. Update **agent** and **web** (flow validation + early-release continue-build).
   Control-plane unchanged if you already run v0.0.52 with migration `0025`.
2. Restart or reinstall the agent on each target.
3. No new database migration.

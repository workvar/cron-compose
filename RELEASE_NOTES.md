# CronCompose v0.0.34

New-project configure is tabbed and modular: pick processes, set build and run
scripts, env, process manager, and Git redeploy triggers in separate steps. The
server search bar is full width. Build artifacts land under the deploy path
(usually `/opt/…`); the run script starts with that folder as cwd. Agents honor
an explicit run command, and webhooks can redeploy on branch push, tag push, or
a published release.

## Highlights

- **Full-width server search** — while choosing a deploy target, the search
  field spans the picker.
- **Tabbed configure** — Processes, Build & run, Environment, Process manager,
  Advanced, and Redeploy replace the long single-page form.
- **Build script + run script** — `install` builds after clone; `run` is the
  start command in the activated app folder (e.g. `./app` for Go). Documented
  on `/docs` and in the UI callout, including port detection for the web UI.
- **Redeploy triggers** — configure `redeploy_on`: `branch` (searchable branch
  dropdown), `tag`, and/or `release`. GitHub/GitLab webhooks subscribe to push
  and release events accordingly.
- **Agent + proto** — `DeployApp.run_script` is passed through; pm2/systemd use
  it when set instead of guessing from language alone.
- **Migration** — `0021_redeploy_on.sql` adds `deploy_projects.redeploy_on`
  (default `["branch"]`).

## Upgrade

Apply migrations, then rebuild and restart the **control plane**, **web UI**,
and **agent**. Existing projects keep branch-push redeploy until you change
`redeploy_on`.

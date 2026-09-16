# CronCompose v0.0.6

Deploys get real health signal instead of "the install script exited 0": atomic
releases, opt-in health checks that gate success, auto-rollback that reports what it
actually did, and commit statuses posted by CronCompose itself via an optional GitHub
App.

## Highlights

- **Atomic releases, preflight, and bounded runs** — Each deploy now lands in its own
  release directory and is swapped in with a symlink, so a failed or half-finished
  install never leaves a project half-upgraded. A preflight pass and a run timeout with
  a 2 MiB log cap keep one bad script from hanging or flooding storage.
- **Opt-in health checks** — Set a health path/port on a project (`HealthCheckFields`
  in the UI) and a deploy only counts as successful once the app actually answers, not
  just once the install script exits 0. Without one set, behavior is unchanged. This is
  what makes auto-rollback catch a clean install that crashes on boot.
- **Project health state** — Projects now track `healthy` / `degraded` / `rolled_back`
  / `unknown` (migration `0014`), shown as a badge on the deploy list and detail pages.
  Notifications say which phase failed and what the project's state is now, not just
  pass/fail.
- **Auto-rollback audit trail** — Every automatic rollback now leaves an audit entry
  recording what triggered it and which commit it rolled back to.
- **Webhook idempotency** — GitHub and GitLab both retry deliveries, and a repo with
  both the push webhook and the generated CI job can report the same commit twice.
  Duplicate deliveries (by project + delivery id) no longer start a second deploy.
- **GitHub App commit statuses (optional)** — Configure a GitHub App
  (`GITHUB_APP_ID` + `GITHUB_APP_PRIVATE_KEY_PATH`) and deploy commit statuses post as
  CronCompose itself, scoped only to the repos it's installed on, and keep working
  after the importing user leaves or revokes their OAuth grant. Without it, statuses
  still post, just under that user's account, exactly as before. See
  [DEPLOYMENT.md — Creating a GitHub App](DEPLOYMENT.md#creating-a-github-app-optional).

## Upgrade notes

### Control plane (source install)

From Settings → Updates, click **Update** on this host. Or by hand:

```sh
cd cron-compose
git fetch --tags
git checkout --force v0.0.6
./update.sh --no-pull
```

This release applies migration `0014_deploy_health_and_idempotency.sql` (health-check
columns and state on `deploy_projects`, plus the webhook de-duplication table).
`update.sh` runs migrations; back up Postgres first if you want a rollback path.

### Optional: GitHub App for deploy statuses

Nothing changes unless you opt in. To have deploy commit statuses post as CronCompose:

1. Create a GitHub App and note its App ID and private key — full steps in
   [DEPLOYMENT.md](DEPLOYMENT.md#creating-a-github-app-optional) or `.env.example`.
2. Set `GITHUB_APP_ID` and `GITHUB_APP_PRIVATE_KEY_PATH` (or `GITHUB_APP_PRIVATE_KEY`)
   in `.env`.
3. Install the App on the repos you deploy, then restart the control plane.

### Optional: health checks per project

Existing projects are unaffected (health path defaults to empty, meaning "install
script exit code decides success," same as before). To turn one on, set a health
path/port/timeout on the project's deploy settings in the UI.

### Agent (Linux / macOS)

```sh
curl -sSL https://github.com/workvar/cron-compose/releases/latest/download/install-agent.sh | \
  sudo TOKEN=<token> \
       CONTROL_PLANE_HTTP=https://<host>/api \
       CONTROL_PLANE_ADDR=<host>:9090 \
       bash
```

No agent protocol changes beyond what auto-rollback already added in v0.0.5; existing
agents keep working.

## Documentation

- [DEPLOYMENT.md — Configuration](DEPLOYMENT.md#configuration)
- [DEPLOYMENT.md — Creating a GitHub App](DEPLOYMENT.md#creating-a-github-app-optional)
- [REST API — Deploys](docs/api.md#deploys)

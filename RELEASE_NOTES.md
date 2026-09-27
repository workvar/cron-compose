# CronCompose v0.0.17

A Vercel-style import flow, deploys driven by a `croncompose.yml` file, and a
public docs page that explains how to write one.

## Highlights

- **New project in two screens** — The four-step import wizard is now **New
  project** → **Configure project** → **Deploy**. Pick a connected repo (search,
  owner filter, **Import** on each row), paste a public repo URL, or paste /
  upload a `croncompose.yml`. Configure is one page: name, server, apps, with
  **Environment variables** and **Advanced** (branch, folder, health check,
  auto-rollback) folded away. The server is preselected when there is only one,
  or only one online.
- **Deploy from `croncompose.yml`** — Importing a repo reads `croncompose.yml`
  (also `.yaml` and dot-prefixed) from its root and fills every field. Like
  `vercel.json`, the file only overrides: language and install fall back to
  detection. New keys: `version`, `server`, `health`, `deploy_timeout`,
  `auto_rollback`; app `env` accepts a map or a `{key, value}` list. Files
  CronCompose wrote in earlier releases still read cleanly.
- **Validation up front** — Every problem is listed at once before anything
  deploys: bad YAML, unknown keys (with line numbers), invalid process manager or
  port, roots outside the repo, duplicate apps, and secret-looking values.
- **Your file is never overwritten** — When the repo already has a
  `croncompose.yml`, import only adds the CI trigger. Previously the file was
  replaced with a generated copy.
- **Export as croncompose.yml** — Configure turns a hand-built setup into a file
  to copy or download. Sensitive variables are left out.
- **Public repos without a connection** — Paste a GitHub or GitLab URL to import
  it anonymously; the folder picker works too. GitHub file reads use
  raw.githubusercontent.com, so they do not burn the API rate limit.
- **Public docs** — `/docs` (served at `/app/docs`) is readable signed out: quick
  start, full key reference, env and secrets, health checks, process managers,
  six examples, validation errors, and CI triggers. Linked from the sidebar and
  the Deploy page.

## API

- `POST /deploys/spec/validate` — body `{"yaml": "…"}`, returns
  `{spec, issues, valid}`.
- `GET /git/inspect` — adds `spec` when the repo has a file; `public=1` reads a
  public repo without a grant. `GET /git/dirs` falls back to anonymous reads.
- `POST /deploys` — accepts `spec_from_repo`, `auto_rollback`, `health_path`,
  `health_port`, `health_timeout_seconds`, `deploy_timeout_seconds`.

## Upgrade notes

### Control plane (source install)

From Settings → Updates, click **Update** on this host. Or by hand:

```sh
cd cron-compose
git fetch --tags
git checkout --force v0.0.17
./update.sh --no-pull
```

No new migrations in this release.

### Agent (Linux / macOS)

No agent changes required for this release. Update from the UI as usual when a
newer agent tag is offered.

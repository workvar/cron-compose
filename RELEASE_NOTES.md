# CronCompose v0.0.5

Import a GitHub or GitLab repo onto an agent, sign in with those providers, and point installers at `/api` instead of `/api/v1`.

## Highlights

- **Deploys** — Connect a GitHub or GitLab grant under Settings → Git, then **Deploy → Import git** to clone a repo onto a chosen agent and run its installer with a live PTY log. Attach PM2, systemd, or Docker Compose afterwards. Import can commit `croncompose.yml` plus GitHub Actions / GitLab CI and register a push webhook so later pushes start a run. Redeploy from the project page.
- **GitHub / GitLab login** — Optional OAuth apps power “Sign in with GitHub/GitLab”. Login and git-connect are separate (`purpose=login` vs `purpose=git`). Register `<PUBLIC_BASE_URL>/api/auth/github/callback` and `…/gitlab/callback` on the OAuth app. Git clone needs repo scope.
- **Public REST is `/api`** — `PUBLIC_HTTP_URL` and agent `CONTROL_PLANE_HTTP` should be `https://<host>/api`, not `/api/v1`. Using `/api/v1` behind the Next.js front doubles the prefix and 401s enrollment.
- **Installer and updates** — The source installer, systemd setup, standalone agent install, and `update.sh` write agent sudoers for the Ports page. `install-agent.sh` finds Go when sudo strips `PATH`. `./install/install.sh --advanced` prompts for OIDC client credentials. Source-tree restore after an update is more reliable.
- **UI** — Collapsible sidebar (width persisted), copy button on the new-server install command, account details in the avatar menu, and `scope_id` on the secrets form.

## Upgrade notes

### Control plane (source install)

From Settings → Updates, click **Update** on this host. Or by hand:

```sh
cd cron-compose
git fetch --tags
git checkout --force v0.0.5
./update.sh --no-pull
```

This release applies migration `0012_deploys.sql` (git connections, OAuth identities, deploy projects). `update.sh` runs migrations; back up Postgres first if you want a rollback path.

### Agent (Linux / macOS)

```sh
curl -sSL https://github.com/workvar/cron-compose/releases/latest/download/install-agent.sh | \
  sudo TOKEN=<token> \
       CONTROL_PLANE_HTTP=https://<host>/api \
       CONTROL_PLANE_ADDR=<host>:9090 \
       bash
```

Use `/api`, not `/api/v1`. Windows is not supported for the agent (Unix process APIs). `install-agent.ps1` only says so.

### Existing installs

Optional, for GitHub/GitLab login and Deploy. Set these in `.env` and restart:

```
GITHUB_OAUTH_CLIENT_ID=
GITHUB_OAUTH_CLIENT_SECRET=
GITLAB_OAUTH_CLIENT_ID=
GITLAB_OAUTH_CLIENT_SECRET=
```

If `PUBLIC_HTTP_URL` is still `…/api/v1`, change it to `…/api` (or unset it and let `PUBLIC_BASE_URL` derive it) and restart so new enroll commands use `/api`. Existing agents keep working over gRPC; re-run the installer only when adding a server.

## Documentation

- [REST API — Deploys](docs/api.md#deploys)
- [Deployment — updates](DEPLOYMENT.md#updates-source-builds)
- [Operations — OIDC](docs/operations.md#oidc-sso)

# CronCompose v0.0.19

Git connections renew their access tokens automatically, so repo fetching no
longer dies when a GitHub or GitLab token expires.

## Highlights

- **Auto-refresh Git OAuth tokens** — Connecting GitHub or GitLab now stores the
  refresh token and expiry alongside the access token. When the access token is
  near expiry, CronCompose renews it before listing repos, inspecting, or
  deploying. Concurrent callers share one refresh so a rotating refresh token is
  not burned twice.
- **One reconnect after upgrade** — Existing connections only have an access
  token. Reconnect Git once after upgrading so a refresh token is stored; after
  that, expiry is handled for you. Non-expiring GitHub OAuth apps (no refresh
  token returned) keep working as before.

## Upgrade notes

### Control plane (source install)

From Settings → Updates, click **Update** on this host. Or by hand:

```sh
cd cron-compose
git fetch --tags
git checkout --force v0.0.19
./update.sh --no-pull
```

This release adds columns on `git_connections` (`refresh_token_enc`,
`token_expires_at`). Migrations run automatically on update.

After upgrading, open Settings (or New project) and **reconnect** each Git
provider once if repo lists fail with an expired token.

### Agent (Linux / macOS)

No agent changes required for this release.

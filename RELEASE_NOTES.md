# CronCompose v0.0.8

Configure Git OAuth from Settings instead of `.env`, delete a server (and its run
history) from the UI, and a batch of smaller fixes: a slimmer update banner, editable
clone-path languages, a Ports server filter, and a password-manager annoyance on
Secrets.

## Highlights

- **Git OAuth from Settings** — GitHub/GitLab OAuth app credentials (client ID,
  client secret, callback URL, GitLab base URL) can now be plugged in from
  Settings → Git OAuth, admin-only. Takes effect immediately, no restart. Leave it
  unset and the control plane falls back to `GITHUB_OAUTH_*`/`GITLAB_OAUTH_*` in
  `.env` exactly as before, so nothing changes for an existing install that doesn't
  touch this.
- **Delete a server** — The server detail page now has a Delete action
  (admin/owner), which removes the server, its jobs, and its run history. Fixes a
  related bug: deleting a server with any run history used to fail outright with a
  foreign-key error, because `runs.server_id` was the one server-scoped table that
  never cascaded.
- **Clone paths: add/remove languages** — Settings → Clone paths now lets you add a
  language that isn't in the default set or drop one you don't use (e.g. Elixir),
  laid out as a responsive grid with a logo per language instead of a single
  stacked column.
- **Ports: filter by server** — A server dropdown next to the search bar scopes the
  Ports table to one server, on top of the existing text search.
- **Dashboard update banner, slimmed** — Collapses to one row instead of a
  headline + paragraph + button column.
- **Secrets form fix** — The scope field no longer gets auto-highlighted by
  password managers guessing it's a username field next to the value input.

## Upgrade notes

### Control plane (source install)

From Settings → Updates, click **Update** on this host. Or by hand:

```sh
cd cron-compose
git fetch --tags
git checkout --force v0.0.8
./update.sh --no-pull
```

This release applies two migrations: `0015_server_delete_cascade.sql` (cascades
`runs.server_id`, needed for the new Delete action) and
`0016_oauth_settings.sql` (adds the `oauth_settings` table for the new Git OAuth
UI — empty until you fill it in, so nothing changes unless you use it). `update.sh`
runs both automatically; back up Postgres first if you want a rollback path.

### Agent (Linux / macOS)

No agent changes in this release. Existing agents keep working as-is.

```sh
curl -sSL https://github.com/workvar/cron-compose/releases/latest/download/install-agent.sh | \
  sudo TOKEN=<token> \
       CONTROL_PLANE_HTTP=https://<host>/api \
       CONTROL_PLANE_ADDR=<host>:9090 \
       bash
```

### Optional: Git OAuth via Settings

Nothing changes unless you opt in.

1. Create a GitHub or GitLab OAuth app, using this control plane's public URL plus
   `/api/auth/<github|gitlab>/callback` as the callback URL (Settings → Git OAuth
   shows the exact URL it expects).
2. Paste the client ID and client secret into Settings → Git OAuth and save. GitLab
   also takes a base URL if you're self-hosted.
3. That's it — no restart. Use **Revert to .env** to go back to whatever (if
   anything) is set there.

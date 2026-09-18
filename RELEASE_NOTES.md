# CronCompose v0.0.12

Agent self-update works on systemd installs where `/usr/local/bin` is root-owned,
and passkeys can be renamed inline under Settings → Security.

## Highlights

- **Agent install when the directory is not writable** — Source and binary
  self-updates no longer fail with
  `open /usr/local/bin/croncompose-agent.new: permission denied`. When the
  install directory cannot create a sibling `.new` file, the agent backs up
  beside `DATA_DIR` and overwrites the existing binary in place.
- **Rename passkeys** — Click a passkey name in Settings → Security to edit it
  inline (Enter saves, Escape cancels). `PATCH /auth/passkeys/:id` with
  `{ "name" }`. Empty names become `Passkey`.
- **Passkey delete control** — Delete uses the same trash icon button as the
  servers page (`button icon-danger`).

## Upgrade notes

### Control plane (source install)

From Settings → Updates, click **Update** on this host. Or by hand:

```sh
cd cron-compose
git fetch --tags
git checkout --force v0.0.12
./update.sh --no-pull
```

No new migrations in this release.

### Agent (Linux / macOS)

If a prior source update failed with permission denied on
`croncompose-agent.new`, this release contains the fix — but that broken
binary cannot install it itself. Reinstall once from this tag (or latest), then
future updates should succeed:

```sh
curl -sSL https://github.com/workvar/cron-compose/releases/latest/download/install-agent.sh | \
  sudo TOKEN=<token> \
       CONTROL_PLANE_HTTP=https://<host>/api \
       CONTROL_PLANE_ADDR=<host>:9090 \
       bash
```

Agents already on a build that includes the in-place install path can update
from the UI as usual.

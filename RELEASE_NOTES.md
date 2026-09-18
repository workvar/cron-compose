# CronCompose v0.0.13

OAuth setup moves into the Connect flow, deploy apps get Vercel-style env vars
(with sensitive secrets), the dashboard shows control-plane host metrics and more
charts, and agent updates that fail for lack of root spell out how to fix it.

## Highlights

- **Connect GitHub/GitLab OAuth modal** — If OAuth app credentials are not
  configured, **Connect** opens an admin-only modal (client ID/secret, callback,
  GitLab base URL). Save, then click Connect again. The separate Settings → Git
  OAuth section is removed. Non-admins see an ask-an-admin message.
- **Per-app environment variables** — Each deploy app has a Vercel-style env
  editor: paste a full `.env` (comments stripped), add/edit rows, mark
  **Sensitive** (values are encrypted at rest and never shown again — Replace
  only). Edits on the deploy page **autosave**; a sticky bar asks you to
  **Redeploy** so the agent applies them (redeploy stays manual).
- **Dashboard host metrics & charts** — Control plane card shows CPU, memory,
  disk, load, and uptime via `GET /system/host`. New chart types: area, heatmap,
  scatter, bubble, radar, and treemap alongside the existing bar and gauge.
- **Clone paths full width** — The Settings → Clone paths panel spans the same
  width as other settings cards.
- **Agent update permission denied** — Hardened install falls back to in-place
  overwrite when creating `croncompose-agent.new` fails. The server update UI
  states that the agent **lacks root permission**, with a link to **Agent root
  access** (`#agent-root-access`) before Retry.

## Upgrade notes

### Control plane (source install)

From Settings → Updates, click **Update** on this host. Or by hand:

```sh
cd cron-compose
git fetch --tags
git checkout --force v0.0.13
./update.sh --no-pull
```

No new migrations in this release. Sensitive deploy env vars use the existing
`SECRETS_MASTER_KEY` / cryptobox.

### Agent (Linux / macOS)

If updates still fail with permission denied on `croncompose-agent.new` (common
on **v0.0.11** and earlier):

1. On the server page, turn on **Agent root access**, then **Retry**, or
2. Reinstall once from this tag:

```sh
curl -sSL https://github.com/workvar/cron-compose/releases/latest/download/install-agent.sh | \
  sudo TOKEN=<token> \
       CONTROL_PLANE_HTTP=https://<host>/api \
       CONTROL_PLANE_ADDR=<host>:9090 \
       bash
```

Agents already past the in-place install fix can update from the UI as usual.

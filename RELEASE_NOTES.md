# CronCompose v0.0.14

Connect GitHub/GitLab after saving OAuth app credentials actually starts the
provider OAuth flow (instead of reopening the setup modal), and agent self-update
stops getting stuck reporting the install-time version.

## Highlights

- **Connect after OAuth app save** — Saving client ID/secret now **Save and
  connect**, which stores credentials and sends you to GitHub/GitLab. A second
  **Connect** no longer reopens the setup modal (the old `fetch` + opaque
  redirect path treated success as “not configured”). Status text distinguishes
  **OAuth app not configured** from **Not connected**; admins can still edit
  credentials via **OAuth app**.
- **Agent version after self-update** — Installers no longer pin
  `Environment=AGENT_VERSION` in the unit/plist. The agent prefers the version
  linked into the binary, and elevate/self-update clear a leftover pin so Hello
  (and the Updates UI) leave “restarting” once the new binary is running.

## Upgrade notes

### Control plane (source install)

From Settings → Updates, click **Update** on this host. Or by hand:

```sh
cd cron-compose
git fetch --tags
git checkout --force v0.0.14
./update.sh --no-pull
```

No new migrations in this release.

### Agent (Linux / macOS)

Agents updating from the UI pick up the version-pin fix. If an older install
still pins `AGENT_VERSION` in the systemd unit and Updates stays on
**restarting**, elevate root access once (or reinstall from this tag) so the
unset drop-in can clear it:

```sh
curl -sSL https://github.com/workvar/cron-compose/releases/latest/download/install-agent.sh | \
  sudo TOKEN=<token> \
       CONTROL_PLANE_HTTP=https://<host>/api \
       CONTROL_PLANE_ADDR=<host>:9090 \
       bash
```

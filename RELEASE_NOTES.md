# CronCompose v0.0.7

Agent updates now tell you what's actually happening, and keep telling you after a
refresh.

## Highlights

- **Live agent update progress** — Clicking **Update** on a server no longer just
  flips to a static "Started" label. The panel now polls the control plane and shows
  the real phase — *Building from source…*, *Restarting agent…*, then *Updated* — with
  a spinner and an elapsed-time clock, both on the server detail page and in
  Settings → Updates.
- **Survives a page refresh** — The in-progress state is now tracked per server
  (not just in memory), so reloading the page while an agent is mid-rebuild no longer
  shows a fresh, clickable "Update" button. It picks up right where the update left
  off and disables itself correctly until the agent reports the new version or the
  attempt times out (20 minutes), at which point you can retry.
- Whole-stack (control-plane host) updates are unaffected — they still hand off to the
  existing full-screen update overlay, since that host goes down during its own
  rebuild.

## Upgrade notes

This is a web-only UI change: no database migration, no agent protocol change, no
control-plane API change.

### Control plane (source install)

From Settings → Updates, click **Update** on this host. Or by hand:

```sh
cd cron-compose
git fetch --tags
git checkout --force v0.0.7
./update.sh --no-pull
```

### Agent (Linux / macOS)

No agent changes in this release. Existing agents keep working; update them from
Settings → Updates or Servers → *host* to pick up the new progress UI once their
control plane is on v0.0.7.

```sh
curl -sSL https://github.com/workvar/cron-compose/releases/latest/download/install-agent.sh | \
  sudo TOKEN=<token> \
       CONTROL_PLANE_HTTP=https://<host>/api \
       CONTROL_PLANE_ADDR=<host>:9090 \
       bash
```

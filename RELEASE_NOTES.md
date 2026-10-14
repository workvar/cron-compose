# CronCompose v0.0.60

Browser Web Push for deploys and updates, plus richer network interface details
in the UI.

## Highlights

- **Browser Web Push** — In Settings, enable push on this device to get notified
  when a deploy starts, succeeds, or fails; when a job fails; or when a newer
  agent release appears — even if the CronCompose tab is closed. Requires HTTPS
  (or localhost). VAPID keys are auto-generated and stored in Postgres; optional
  `VAPID_*` env vars override them when you need stable keys across empty DBs.
- **Networks interface details** — An Interfaces overview (and wired / active
  Wi‑Fi drawers) shows IPv4/IPv6, gateway, DNS, MAC, method, and connection for
  each reported link. The active-link summary includes IP, gateway, and MAC.

## Upgrade

1. Restart the **control plane** so migration `0026_web_push.sql` applies and
   VAPID keys are ready. Rebuild/restart the web UI so `/sw.js` and Settings
   pick up the new push UI.
2. Open **Settings → Browser notifications**, allow permission, and enable push
   on each device you want woken.
3. Agent update is **not** required for this release (Networks uses fields
   agents already report).

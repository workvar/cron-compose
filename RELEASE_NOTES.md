# CronCompose v0.0.62

Safari-friendly browser push: a Home Screen web app manifest for iPhone/iPad,
plus clearer guidance when push is not available in a Safari tab.

## Highlights

- **PWA manifest** — `display: standalone` so Add to Home Screen installs a web
  app (required for Web Push on iOS/iPadOS 16.4+). macOS Safari 16+ still works
  in a normal tab.
- **Safari subscribe hardening** — VAPID `applicationServerKey` is passed in a
  form Mac Safari accepts more reliably.
- **Settings UI** — When push cannot run (iPhone Safari tab, insecure origin,
  old browser), Settings explains why and, on iOS, how to Add to Home Screen.

## Upgrade

1. Rebuild/restart the **web UI** (and control plane if it is not already on
   v0.0.60+ with web push).
2. **Mac Safari:** Settings → Browser notifications → Enable → Test.
3. **iPhone/iPad:** Share → Add to Home Screen → open that icon → Enable.
4. Agent update is **not** required for this release.

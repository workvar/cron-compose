# CronCompose v0.0.46

Networks UI redesign: guided setup flows, mode tabs, and expandable actions
instead of a static dump of tables and misaligned forms.

## Highlights

- **Flow-first Networks** — Wireless uses a Join wizard (pick SSID → password →
  confirm save / connect). Wired and Bluetooth open in-row drawers so actions
  sit with the selected interface or device.
- **Mode tabs** — Wireless, Wired, Bluetooth, and Cellular are separate panels;
  one connection type at a time.
- **Less clutter** — Virtual ethernet (`veth*`, bridges, etc.) is hidden by
  default with a “Show virtual” toggle. The giant decorative title icon is gone;
  a compact active-link strip shows the current path.
- **Safer Wi‑Fi copy** — Confirm step still defaults to save-without-drop;
  connect and alongside remain explicit choices.

## Upgrade

Update **web** (and redeploy the UI). Control-plane and agent APIs are unchanged
from v0.0.45. No migration.

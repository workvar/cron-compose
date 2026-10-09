# CronCompose v0.0.45

Agent host networking from the UI: wired IP, Wi‑Fi (including save-without-drop
and dual-STA when the radio allows), Bluetooth pairing with interactive PIN and
PAN, plus cellular via ModemManager — with a Networks sidebar tab and jobs
navigation polish.

## Highlights

- **Networks tab** — New sidebar page to pick a server and manage its links:
  wired DHCP/static, Wi‑Fi save/connect/password/forget, Bluetooth
  pair/connect/forget + PAN, and cellular APN/connect when a modem is present.
- **NetworkManager first** — Agents prefer `nmcli` (typical on Raspberry Pi OS),
  fall back to netplan when NM is absent, and advertise `network` /
  `network.bluetooth` / `network.cellular` / `network.dual_wifi` capabilities.
- **Safer Wi‑Fi changes** — Saving a profile or rotating a PSK does not activate
  the connection (no drop). Optional **Connect alongside** uses a virtual STA
  when the phy supports dual-station; otherwise the UI keeps the save-only path.
- **Bluetooth PIN** — Pairing streams SSE `pin_required` events; the UI modal
  posts the PIN/passkey back without closing the in-flight pair request.
- **Privilege / install** — Agent sudoers and priv allowlists cover `nmcli`,
  `bluetoothctl`, `mmcli`, `netplan`, `iw`, and `ip`. Re-run
  `install/lib/agent_sudoers.sh` on managed hosts after upgrading the agent.
- **Jobs UX** — Jobs list is server-card based (`/jobs/servers/:id`); shell
  chrome is split into `ShellFrame` for clearer auth vs app layouts.

## Upgrade

Update **control-plane**, **agent**, and **web**. No new migration.

1. Deploy the control plane and web UI.
2. Upgrade agents to this tag (or rebuild from source).
3. Refresh agent sudoers on each host so network binaries are granted:
   `sudo ./install/lib/agent_sudoers.sh <agent-user>`
4. Restart the control plane and agents so `NetworkRequest` handlers and the
   Networks API are live.

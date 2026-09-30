# CronCompose v0.0.24

Fixes the case where an agent installed as root still showed "Enable as root" and
"Reinstall as root" because it could not reach the control plane. The installer now
tells you when that happens, and the agent has its own variable names so its
settings can no longer clash with the control plane's.

## Highlights

- **Agent-specific variable names.** The agent now reads `AGENT_GRPC_ADDR`,
  `AGENT_GRPC_SNI` and `AGENT_ENROLL_HTTP`. They cannot be mistaken for the control
  plane's own `GRPC_ADDR` or `PUBLIC_GRPC_ADDR` when both share one `.env`. The old
  `CONTROL_PLANE_ADDR`, `CONTROL_PLANE_SNI` and `CONTROL_PLANE_HTTP` names still work
  as a fallback. If both spellings are set to different values, the new name wins and
  the agent logs a warning naming the one it ignored.
- **Install commands use the new names.** The commands shown on "Add server" and
  "Reinstall as root" now pass `AGENT_ENROLL_HTTP` and `AGENT_GRPC_ADDR`. The
  installer accepts both spellings, and the unit files, macOS plist, packaging and
  local-agent installer now write the new names.
- **Installer warns about an unreachable gRPC address.** After starting the service
  it tests the address the agent will really dial. An agent that cannot dial never
  reports its privileges, which left the toggle stuck. The warning explains that raw
  gRPC does not pass Cloudflare's proxy and suggests `127.0.0.1:<port>` with
  `AGENT_GRPC_SNI` for an agent on the control plane host.
- **Installer warns about overridden settings and stray agents.** It names a systemd
  drop-in that changes the address, calls out a stale `CONTROL_PLANE_ADDR` that is now
  ignored, and finds another agent process at any path, not only the installed binary.
- **Legacy root agents are never demoted automatically.** A turned-off flag is only
  re-sent as a demote when an operator switched it off, so agents installed as root
  before v0.0.23 keep running as root after an upgrade.
- **Tests and docs.** New installer, agent config, install command and database tests,
  and updated `docs/operations.md`, `DEVELOPMENT.md` and `DEPLOYMENT.md`.

## Upgrade

No database migration. Publish this release before upgrading the control plane: the
install command now uses the new variable names and fetches the installer from the
latest release, and v0.0.23's installer does not know them.

Existing agents keep working with their old variable names. New installs and reinstalls
write the new names. If you added a systemd drop-in with `CONTROL_PLANE_ADDR`, a
reinstalled unit's `AGENT_GRPC_ADDR` takes precedence, so switch the drop-in to
`AGENT_GRPC_ADDR` too. The installer and agent both warn when they see the conflict.

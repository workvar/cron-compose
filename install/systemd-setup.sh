#!/usr/bin/env bash
# Install systemd units for a CronCompose install so it starts on boot and restarts on
# crash. Run AFTER install/install.sh has built everything and written .env.
#
#   sudo ./install/systemd-setup.sh
#
# It reads .env for the runtime dir, ports, and which services are enabled, derives all
# absolute paths from this checkout, and installs one unit per service:
#   croncompose-control-plane.service, croncompose-web.service, croncompose-agent.service
#
# Test/dry-run without root:  SYSTEMD_DIR=/tmp/units NO_ENABLE=1 ./install/systemd-setup.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LIB_DIR="$SCRIPT_DIR/lib"
# shellcheck source=lib/agent_sudoers.sh
. "$LIB_DIR/agent_sudoers.sh"

ENV_FILE="$REPO_ROOT/.env"
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"

[ -f "$ENV_FILE" ] || { echo "no .env found at $ENV_FILE; run install/install.sh first" >&2; exit 1; }

# Load the values we need from .env (first '=' splits key/value).
CC_RUNTIME_DIR=""; CC_ENABLE_WEB="1"; CC_ENABLE_AGENT="0"; CC_WEB_PORT=""
while IFS= read -r line; do
  case "$line" in ''|\#*) continue ;; esac
  key="${line%%=*}"; val="${line#*=}"
  case "$key" in
    CC_RUNTIME_DIR) CC_RUNTIME_DIR="$val" ;;
    CC_ENABLE_WEB) CC_ENABLE_WEB="$val" ;;
    CC_ENABLE_AGENT) CC_ENABLE_AGENT="$val" ;;
    CC_WEB_PORT) CC_WEB_PORT="$val" ;;
  esac
done < "$ENV_FILE"

# Run services as the user who invoked sudo (so the agent runs jobs as them, not root).
SVC_USER="${SUDO_USER:-$(id -un)}"
NODE_BIN="$(command -v node || true)"

writing_to_etc=0
case "$SYSTEMD_DIR" in /etc/*|/lib/*|/usr/lib/*) writing_to_etc=1 ;; esac
if [ "$writing_to_etc" = "1" ] && [ "$(id -u)" -ne 0 ]; then
  echo "this must run as root to write $SYSTEMD_DIR; use sudo" >&2; exit 1
fi

mkdir -p "$SYSTEMD_DIR"

unit_header() { # <description> <extra After=...>
  printf '[Unit]\nDescription=%s\nAfter=network-online.target %s\nWants=network-online.target\n\n[Service]\nType=simple\nUser=%s\nEnvironmentFile=%s\nRestart=always\nRestartSec=5\n' \
    "$1" "$2" "$SVC_USER" "$ENV_FILE"
}
unit_footer() { printf '\n[Install]\nWantedBy=multi-user.target\n'; }

echo "==> writing units to $SYSTEMD_DIR (user=$SVC_USER)"

# Control plane.
{
  unit_header "CronCompose control plane" ""
  printf 'WorkingDirectory=%s\nExecStart=%s/control-plane/bin/control-plane\n' "$REPO_ROOT" "$REPO_ROOT"
  unit_footer
} > "$SYSTEMD_DIR/croncompose-control-plane.service"
echo "  + croncompose-control-plane.service"

UNITS="croncompose-control-plane.service"

# Web UI (Next.js standalone server).
if [ "$CC_ENABLE_WEB" != "0" ]; then
  [ -n "$NODE_BIN" ] || echo "  ! node not found on PATH; fix ExecStart in croncompose-web.service" >&2
  {
    unit_header "CronCompose web UI" "croncompose-control-plane.service"
    printf 'WorkingDirectory=%s/web/.next/standalone\nEnvironment=HOSTNAME=0.0.0.0\nExecStart=%s server.js\n' \
      "$REPO_ROOT" "${NODE_BIN:-/usr/bin/node}"
    unit_footer
  } > "$SYSTEMD_DIR/croncompose-web.service"
  echo "  + croncompose-web.service  (PORT=${CC_WEB_PORT:-from .env})"
  UNITS="$UNITS croncompose-web.service"
fi

# Local agent.
if [ "$CC_ENABLE_AGENT" = "1" ]; then
  {
    unit_header "CronCompose agent" "croncompose-control-plane.service"
    printf 'WorkingDirectory=%s\nExecStart=%s/agent/bin/agent run\n' "$REPO_ROOT" "$REPO_ROOT"
    unit_footer
  } > "$SYSTEMD_DIR/croncompose-agent.service"
  echo "  + croncompose-agent.service"
  UNITS="$UNITS croncompose-agent.service"
  if [ "$writing_to_etc" = "1" ]; then
    install_agent_sudoers "$SVC_USER" || true
  fi
fi

if [ "${NO_ENABLE:-0}" = "1" ] || ! command -v systemctl >/dev/null 2>&1; then
  echo "==> units written (skipping enable: NO_ENABLE set or systemctl missing)"
  exit 0
fi

# Hand off from any installer-started (nohup) processes to systemd.
if [ -x "$REPO_ROOT/croncompose-ctl.sh" ]; then
  echo "==> stopping installer-started processes"
  sudo -u "$SVC_USER" "$REPO_ROOT/croncompose-ctl.sh" stop || true
fi

echo "==> enabling and starting services"
systemctl daemon-reload
# shellcheck disable=SC2086
systemctl enable --now $UNITS
echo
echo "done. check status with:  systemctl status croncompose-control-plane croncompose-web"
echo "logs:                     journalctl -u croncompose-web -f"

#!/usr/bin/env bash
# CronCompose agent installer for Linux (systemd) and macOS (launchd).
#
# Downloads the agent binary for this OS and architecture, enrolls it against the
# control plane, and installs it as a service that starts at boot.
#
# Required env (or flags):
#   TOKEN                 one-time enrollment token from the UI (required)
#   CONTROL_PLANE_HTTP    base URL for the REST enroll call, e.g. https://cc.example.com/api/v1
#   CONTROL_PLANE_ADDR    host:port of the mTLS gRPC endpoint, e.g. cc.example.com:9090
#   CONTROL_PLANE_SNI     server name to verify against (defaults to host portion of ADDR)
#   AGENT_VERSION         release tag to download, defaults to "latest"
#   DOWNLOAD_BASE         base URL for the binary; default points at GitHub releases
#   DATA_DIR              agent state directory; defaults per platform (see below)
#
# Run example:
#   curl -sSL https://cc.example.com/agent.sh | \
#     sudo TOKEN=abc CONTROL_PLANE_HTTP=https://cc.example.com/api/v1 \
#          CONTROL_PLANE_ADDR=cc.example.com:9090 bash
#
# This file stays a single self-contained script on purpose: it is fetched and piped
# straight into bash, so it cannot source helper files the way install/install.sh does.

set -euo pipefail

: "${TOKEN:?TOKEN env var is required}"
: "${CONTROL_PLANE_HTTP:?CONTROL_PLANE_HTTP env var is required}"
: "${CONTROL_PLANE_ADDR:?CONTROL_PLANE_ADDR env var is required}"

AGENT_VERSION="${AGENT_VERSION:-latest}"
DOWNLOAD_BASE="${DOWNLOAD_BASE:-https://github.com/croncompose/croncompose/releases}"
SNI="${CONTROL_PLANE_SNI:-${CONTROL_PLANE_ADDR%%:*}}"
BIN_PATH=/usr/local/bin/croncompose-agent

UNIT_PATH=/etc/systemd/system/croncompose-agent.service
PLIST_LABEL=com.croncompose.agent
PLIST_PATH="/Library/LaunchDaemons/${PLIST_LABEL}.plist"
MAC_LOG=/usr/local/var/log/croncompose-agent.log

if [[ "$(id -u)" -ne 0 ]]; then
  echo "this installer must be run as root (use sudo)" >&2
  exit 1
fi

# --- platform detection ----------------------------------------------------

os="$(uname -s)"
arch="$(uname -m)"

case "$os" in
  Linux)
    case "$arch" in
      x86_64|amd64) target="linux-amd64" ;;
      aarch64|arm64) target="linux-arm64" ;;
      armv7l) target="linux-armv7" ;;
      *) echo "unsupported arch: $arch" >&2; exit 1 ;;
    esac
    DATA_DIR="${DATA_DIR:-/var/lib/croncompose}"
    ;;
  Darwin)
    case "$arch" in
      arm64) target="darwin-arm64" ;;
      x86_64) target="darwin-amd64" ;;
      *) echo "unsupported arch: $arch" >&2; exit 1 ;;
    esac
    # /var/lib is not a macOS location. This must match defaultDataDir in
    # agent/internal/config/datadir_darwin.go, or the service starts with an empty
    # store and re-enrolls.
    DATA_DIR="${DATA_DIR:-/usr/local/var/croncompose}"
    ;;
  *)
    echo "unsupported OS: $os (the agent supports Linux and macOS)" >&2
    exit 1
    ;;
esac

# --- shared ----------------------------------------------------------------

download_binary() {
  echo "==> downloading agent ($AGENT_VERSION / $target)"
  local url="${DOWNLOAD_BASE}/${AGENT_VERSION}/download/croncompose-agent-${target}"
  curl -fSL --retry 3 -o "$BIN_PATH.tmp" "$url"
  chmod 0755 "$BIN_PATH.tmp"
  mv "$BIN_PATH.tmp" "$BIN_PATH"
}

# Load install/lib/agent_sudoers.sh from a checkout, or fetch it when this script is
# piped from curl (no repo on disk).
_source_agent_sudoers_lib() {
  local lib ref="${AGENT_VERSION:-main}" tmp
  [[ "$ref" == "latest" ]] && ref=main
  if [[ -n "${CRONCOMPOSE_REPO_ROOT:-}" && -f "${CRONCOMPOSE_REPO_ROOT}/install/lib/agent_sudoers.sh" ]]; then
    # shellcheck source=../install/lib/agent_sudoers.sh
    . "${CRONCOMPOSE_REPO_ROOT}/install/lib/agent_sudoers.sh"
    return 0
  fi
  local script_path="${BASH_SOURCE[1]:-${BASH_SOURCE[0]}}"
  if [[ -n "$script_path" && "$script_path" != bash && -f "$script_path" ]]; then
    lib="$(cd "$(dirname "$script_path")/../install/lib" 2>/dev/null && pwd)/agent_sudoers.sh"
    if [[ -f "$lib" ]]; then
      # shellcheck source=/dev/null
      . "$lib"
      return 0
    fi
  fi
  tmp="$(mktemp)"
  curl -fsSL "${RAW_BASE:-https://raw.githubusercontent.com/workvar/cron-compose}/${ref}/install/lib/agent_sudoers.sh" -o "$tmp"
  # shellcheck source=/dev/null
  . "$tmp"
  rm -f "$tmp"
}

# --- Linux -----------------------------------------------------------------

install_linux() {
  echo "==> creating service user and data dir"
  id -u croncompose >/dev/null 2>&1 || \
    useradd --system --home-dir "$DATA_DIR" --shell /usr/sbin/nologin croncompose
  install -d -o croncompose -g croncompose -m 0700 "$DATA_DIR"

  download_binary

  echo "==> writing systemd unit"
  cat >"$UNIT_PATH" <<EOF
[Unit]
Description=CronCompose agent
After=network-online.target
Wants=network-online.target

[Service]
User=croncompose
Group=croncompose
Environment=CONTROL_PLANE_ADDR=${CONTROL_PLANE_ADDR}
Environment=CONTROL_PLANE_HTTP=${CONTROL_PLANE_HTTP}
Environment=CONTROL_PLANE_SNI=${SNI}
Environment=DATA_DIR=${DATA_DIR}
ExecStart=${BIN_PATH} run
Restart=always
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF

  echo "==> enrolling"
  sudo -u croncompose \
    CONTROL_PLANE_ADDR="$CONTROL_PLANE_ADDR" \
    CONTROL_PLANE_HTTP="$CONTROL_PLANE_HTTP" \
    CONTROL_PLANE_SNI="$SNI" \
    DATA_DIR="$DATA_DIR" \
    "$BIN_PATH" enroll --token="$TOKEN"

  _source_agent_sudoers_lib
  install_agent_sudoers croncompose

  echo "==> starting service"
  systemctl daemon-reload
  systemctl enable --now croncompose-agent.service

  echo
  echo "done. follow logs with: journalctl -u croncompose-agent -f"
}

# --- macOS -----------------------------------------------------------------

install_darwin() {
  # No dedicated service account: macOS has no useradd, and creating a system user
  # through dscl means hand-picking a free UID in the reserved range. The daemon runs
  # as root, which is also what run_as_user needs in order to switch users at all.
  echo "==> creating data and log dirs"
  install -d -o root -g wheel -m 0700 "$DATA_DIR"
  install -d -o root -g wheel -m 0755 "$(dirname "$MAC_LOG")"

  download_binary

  # A binary fetched with curl carries no quarantine attribute, but one copied off a
  # browser download or an AirDrop does, and Gatekeeper then kills it on exec with a
  # message launchd reports only as "Killed: 9". Stripping it is a no-op when absent.
  xattr -d com.apple.quarantine "$BIN_PATH" 2>/dev/null || true

  echo "==> writing launchd daemon"
  cat >"$PLIST_PATH" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${PLIST_LABEL}</string>
    <key>ProgramArguments</key>
    <array>
        <string>${BIN_PATH}</string>
        <string>run</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>CONTROL_PLANE_ADDR</key>
        <string>${CONTROL_PLANE_ADDR}</string>
        <key>CONTROL_PLANE_HTTP</key>
        <string>${CONTROL_PLANE_HTTP}</string>
        <key>CONTROL_PLANE_SNI</key>
        <string>${SNI}</string>
        <key>DATA_DIR</key>
        <string>${DATA_DIR}</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>ThrottleInterval</key>
    <integer>10</integer>
    <key>StandardOutPath</key>
    <string>${MAC_LOG}</string>
    <key>StandardErrorPath</key>
    <string>${MAC_LOG}</string>
    <key>WorkingDirectory</key>
    <string>${DATA_DIR}</string>
</dict>
</plist>
EOF
  chown root:wheel "$PLIST_PATH"
  chmod 0644 "$PLIST_PATH"
  # launchd refuses to load a plist it cannot parse, with an error that does not say
  # which key is wrong. Catching it here, against the file we just wrote, is cheaper
  # than reading it out of the system log later.
  plutil -lint "$PLIST_PATH" >/dev/null

  echo "==> enrolling"
  CONTROL_PLANE_ADDR="$CONTROL_PLANE_ADDR" \
  CONTROL_PLANE_HTTP="$CONTROL_PLANE_HTTP" \
  CONTROL_PLANE_SNI="$SNI" \
  DATA_DIR="$DATA_DIR" \
    "$BIN_PATH" enroll --token="$TOKEN"

  echo "==> starting service"
  # bootstrap fails outright if the label is already loaded, which is exactly what a
  # re-run of this installer hits, so unload first and ignore "not loaded".
  launchctl bootout "system/${PLIST_LABEL}" 2>/dev/null || true
  launchctl bootstrap system "$PLIST_PATH"
  launchctl enable "system/${PLIST_LABEL}"

  echo
  echo "done. follow logs with: tail -f ${MAC_LOG}"
  echo "status:               sudo launchctl print system/${PLIST_LABEL}"
  echo "stop and remove:      sudo launchctl bootout system/${PLIST_LABEL} && sudo rm ${PLIST_PATH}"
  echo
  echo "note: resource limits (cpu_quota, memory_max, tasks_max, io_weight) are not"
  echo "      enforced on macOS. There is no launchd equivalent of a systemd transient"
  echo "      scope, so jobs that declare them run unlimited and say so in the run log."
}

case "$os" in
  Linux) install_linux ;;
  Darwin) install_darwin ;;
esac

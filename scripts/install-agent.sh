#!/usr/bin/env bash
# CronCompose agent installer for Linux (systemd) and macOS (launchd).
#
# Clones the release tag, builds the agent from source, enrolls it against the
# control plane, installs it as a service, then deletes the source tree.
#
# Required env (or flags):
#   TOKEN                 one-time enrollment token from the UI (required)
#   CONTROL_PLANE_HTTP    public REST base for enroll, e.g. https://cc.example.com/api
#                         (not …/api/v1 — that doubles under a Next.js front and 401s)
#   CONTROL_PLANE_ADDR    host:port of the mTLS gRPC endpoint, e.g. cc.example.com:9090
#   CONTROL_PLANE_SNI     server name to verify against (defaults to host portion of ADDR)
#   AGENT_VERSION         release tag to build; defaults to this script's baked tag (or latest)
#   GITHUB_REPO           owner/repo to clone; default workvar/cron-compose
#   DATA_DIR              agent state directory; defaults per platform (see below)
#
# Run example:
#   curl -sSL https://github.com/workvar/cron-compose/releases/latest/download/install-agent.sh | \
#     sudo TOKEN=abc CONTROL_PLANE_HTTP=https://cc.example.com/api \
#          CONTROL_PLANE_ADDR=cc.example.com:9090 bash
#
# Needs git and Go 1.25+ on the target. This file stays a single self-contained script
# on purpose: it is fetched and piped straight into bash.

set -euo pipefail

: "${TOKEN:?TOKEN env var is required}"
: "${CONTROL_PLANE_HTTP:?CONTROL_PLANE_HTTP env var is required}"
: "${CONTROL_PLANE_ADDR:?CONTROL_PLANE_ADDR env var is required}"

# CI replaces __VERSION__ / __REPO__ when attaching this file to a GitHub release.
AGENT_VERSION="${AGENT_VERSION:-__VERSION__}"
GITHUB_REPO="${GITHUB_REPO:-__REPO__}"
if [ "$GITHUB_REPO" = "__REPO__" ] || [ -z "$GITHUB_REPO" ]; then
  GITHUB_REPO="workvar/cron-compose"
fi
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

# sudo's default PATH is often /usr/sbin:/usr/bin and does not include a user
# install of Go (e.g. /usr/local/go/bin). Search the usual places, then the
# invoking user's PATH.
find_go() {
  local candidate home
  if command -v go >/dev/null 2>&1; then
    command -v go
    return 0
  fi
  for candidate in \
      /usr/local/go/bin/go \
      /usr/lib/go/bin/go \
      /opt/go/bin/go \
      /usr/lib/go-1.25/bin/go \
      /usr/lib/go-1.26/bin/go; do
    if [[ -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  if [[ -n "${SUDO_USER:-}" && "$SUDO_USER" != "root" ]]; then
    home="$(getent passwd "$SUDO_USER" 2>/dev/null | cut -d: -f6)"
    [[ -z "$home" ]] && home="$(eval echo "~$SUDO_USER")"
    for candidate in \
        "$home/go/bin/go" \
        "$home/.go/bin/go" \
        "$home/.local/go/bin/go" \
        "$home/sdk/go/bin/go"; do
      if [[ -x "$candidate" ]]; then
        printf '%s\n' "$candidate"
        return 0
      fi
    done
    candidate="$(sudo -u "$SUDO_USER" -H bash -lc 'command -v go' 2>/dev/null || true)"
    if [[ -n "$candidate" && -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  fi
  return 1
}

command -v git >/dev/null 2>&1 || { echo "git is required to build the agent from source" >&2; exit 1; }
GO_BIN="$(find_go)" || {
  echo "Go 1.25+ is required to build the agent from source." >&2
  echo "sudo does not use your user PATH; install Go system-wide or rerun with:" >&2
  echo "  curl ... | sudo env PATH=\"\$PATH\" TOKEN=... CONTROL_PLANE_HTTP=... CONTROL_PLANE_ADDR=... bash" >&2
  exit 1
}
export PATH="$(dirname "$GO_BIN"):$PATH"
echo "==> using $($GO_BIN version)"

# --- platform detection ----------------------------------------------------

os="$(uname -s)"
arch="$(uname -m)"

case "$os" in
  Linux)
    case "$arch" in
      x86_64|amd64) ;;
      aarch64|arm64) ;;
      armv7l) ;;
      *) echo "unsupported arch: $arch" >&2; exit 1 ;;
    esac
    DATA_DIR="${DATA_DIR:-/var/lib/croncompose}"
    ;;
  Darwin)
    case "$arch" in
      arm64|x86_64) ;;
      *) echo "unsupported arch: $arch" >&2; exit 1 ;;
    esac
    DATA_DIR="${DATA_DIR:-/usr/local/var/croncompose}"
    ;;
  *)
    echo "unsupported OS: $os (the agent supports Linux and macOS)" >&2
    exit 1
    ;;
esac

resolve_version() {
  if [ "$AGENT_VERSION" != "__VERSION__" ] && [ "$AGENT_VERSION" != "latest" ] && [ -n "$AGENT_VERSION" ]; then
    return 0
  fi
  echo "==> resolving latest release of $GITHUB_REPO"
  AGENT_VERSION="$(curl -fsSL -H 'Accept: application/vnd.github+json' \
    "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
    | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)"
  if [ -z "$AGENT_VERSION" ]; then
    echo "could not read the latest GitHub release tag for $GITHUB_REPO" >&2
    exit 1
  fi
}

build_from_source() {
  resolve_version
  echo "==> cloning $GITHUB_REPO @$AGENT_VERSION"
  SRC="$(mktemp -d /tmp/croncompose-agent.XXXXXX)"
  trap 'rm -rf "$SRC"' EXIT
  git clone --depth 1 --branch "$AGENT_VERSION" "https://github.com/${GITHUB_REPO}.git" "$SRC"
  echo "==> building agent"
  local ver="${AGENT_VERSION#v}"
  mkdir -p "$SRC/agent/bin"
  (
    cd "$SRC/agent"
    export GOTOOLCHAIN=local
    go build -trimpath \
      -ldflags="-s -w -X github.com/croncompose/croncompose/agent/internal/config.buildVersion=${ver}" \
      -o "$BIN_PATH.tmp" \
      ./cmd/agent
  )
  chmod 0755 "$BIN_PATH.tmp"
  mv "$BIN_PATH.tmp" "$BIN_PATH"
  rm -rf "$SRC"
  trap - EXIT
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

  build_from_source
  chown root:root "$BIN_PATH"
  # Let the service user replace the binary on later source updates.
  chown croncompose:croncompose "$BIN_PATH"

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
Environment=AGENT_VERSION=${AGENT_VERSION}
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
    AGENT_VERSION="$AGENT_VERSION" \
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
  echo "==> creating data and log dirs"
  install -d -o root -g wheel -m 0700 "$DATA_DIR"
  install -d -o root -g wheel -m 0755 "$(dirname "$MAC_LOG")"

  build_from_source
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
        <key>AGENT_VERSION</key>
        <string>${AGENT_VERSION}</string>
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
  plutil -lint "$PLIST_PATH" >/dev/null

  echo "==> enrolling"
  CONTROL_PLANE_ADDR="$CONTROL_PLANE_ADDR" \
  CONTROL_PLANE_HTTP="$CONTROL_PLANE_HTTP" \
  CONTROL_PLANE_SNI="$SNI" \
  DATA_DIR="$DATA_DIR" \
  AGENT_VERSION="$AGENT_VERSION" \
    "$BIN_PATH" enroll --token="$TOKEN"

  echo "==> starting service"
  launchctl bootout "system/${PLIST_LABEL}" 2>/dev/null || true
  launchctl bootstrap system "$PLIST_PATH"
  launchctl enable "system/${PLIST_LABEL}"

  echo
  echo "done. follow logs with: tail -f ${MAC_LOG}"
  echo "status:               sudo launchctl print system/${PLIST_LABEL}"
  echo "stop and remove:      sudo launchctl bootout system/${PLIST_LABEL} && sudo rm ${PLIST_PATH}"
}

case "$os" in
  Linux) install_linux ;;
  Darwin) install_darwin ;;
esac

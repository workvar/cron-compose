#!/usr/bin/env bash
# CronCompose agent installer for Linux (systemd) and macOS (launchd).
#
# Clones the release tag, builds the agent from source, enrolls it against the
# control plane, installs it as a service, then deletes the source tree.
#
# Required env (or flags):
#   TOKEN                 one-time enrollment token from the UI (required)
#   AGENT_ENROLL_HTTP     public REST base for enroll, e.g. https://cc.example.com/api
#                         (not …/api/v1 — that doubles under a Next.js front and 401s)
#   AGENT_GRPC_ADDR       mTLS gRPC endpoint: host:port, or a bare host (port 443) when its
#                         hostname already maps to the gRPC port, e.g. grpc.example.com
#   AGENT_GRPC_SNI        server name to verify against (defaults to host portion of ADDR)
#   (the old CONTROL_PLANE_HTTP / _ADDR / _SNI names are still accepted)
#   AGENT_VERSION         release tag to build; defaults to this script's baked tag (or latest)
#   GITHUB_REPO           owner/repo to clone; default workvar/cron-compose
#   DATA_DIR              agent state directory; defaults per platform (see below)
#   AGENT_RUN_AS_ROOT     1 to run the agent as root from install, skipping the
#                         croncompose service user (Linux only). No Agent root
#                         access toggle needed afterwards, it already runs as root.
#
# Run example:
#   curl -sSL https://github.com/workvar/cron-compose/releases/latest/download/install-agent.sh | \
#     sudo TOKEN=abc AGENT_ENROLL_HTTP=https://cc.example.com/api \
#          AGENT_GRPC_ADDR=cc.example.com:9090 bash
#
# Needs git and Go 1.25+ on the target. This file stays a single self-contained script
# on purpose: it is fetched and piped straight into bash.

set -euo pipefail

: "${TOKEN:?TOKEN env var is required}"
# The agent's own names win; the old CONTROL_PLANE_* names are still accepted.
CONTROL_PLANE_HTTP="${AGENT_ENROLL_HTTP:-${CONTROL_PLANE_HTTP:-}}"
CONTROL_PLANE_ADDR="${AGENT_GRPC_ADDR:-${CONTROL_PLANE_ADDR:-}}"
CONTROL_PLANE_SNI="${AGENT_GRPC_SNI:-${CONTROL_PLANE_SNI:-}}"
: "${CONTROL_PLANE_HTTP:?AGENT_ENROLL_HTTP (or CONTROL_PLANE_HTTP) env var is required}"
: "${CONTROL_PLANE_ADDR:?AGENT_GRPC_ADDR (or CONTROL_PLANE_ADDR) env var is required}"

# CI replaces __VERSION__ / __REPO__ when attaching this file to a GitHub release.
AGENT_VERSION="${AGENT_VERSION:-__VERSION__}"
GITHUB_REPO="${GITHUB_REPO:-__REPO__}"
if [ "$GITHUB_REPO" = "__REPO__" ] || [ -z "$GITHUB_REPO" ]; then
  GITHUB_REPO="workvar/cron-compose"
fi
SNI="${CONTROL_PLANE_SNI:-${CONTROL_PLANE_ADDR%%:*}}"
BIN_PATH=/usr/local/bin/croncompose-agent
PRIVCTL_PATH=/usr/libexec/croncompose/agent-privctl

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
  echo "  curl ... | sudo env PATH=\"\$PATH\" TOKEN=... AGENT_ENROLL_HTTP=... AGENT_GRPC_ADDR=... bash" >&2
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
    go build -trimpath \
      -ldflags="-s -w" \
      -o "$PRIVCTL_PATH.tmp" \
      ./cmd/agent-privctl
  )
  chmod 0755 "$BIN_PATH.tmp"
  mv "$BIN_PATH.tmp" "$BIN_PATH"
  install -d -m 0755 "$(dirname "$PRIVCTL_PATH")"
  install -m 0755 -o root -g root "$PRIVCTL_PATH.tmp" "$PRIVCTL_PATH"
  rm -f "$PRIVCTL_PATH.tmp"
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

# --- systemd service helpers (Linux) -----------------------------------------
# A reinstall must replace a running agent. `systemctl enable --now` never
# restarts a unit that is already active, so on a reinstall the old process (often
# a different user, e.g. croncompose when switching to root) would keep running and
# keep reporting its old privileges. These helpers stop it before re-enrolling and
# restart it afterwards, then report who the agent really runs as.

AGENT_UNIT=croncompose-agent.service

stop_existing_agent() {
  if systemctl is-active --quiet "$AGENT_UNIT" 2>/dev/null; then
    echo "==> stopping the running agent before re-enrolling"
    systemctl stop "$AGENT_UNIT"
  fi
}

start_agent_service() {
  systemctl daemon-reload
  systemctl enable "$AGENT_UNIT"
  systemctl restart "$AGENT_UNIT"
}

# report_agent_user <1|0>: 1 when the install asked for a root agent. Prints the
# user the agent process runs as and warns on a mismatch or on a stray copy that
# systemd does not manage (pm2, nohup). Never fails the install.
report_agent_user() {
  local want_root="${1:-0}" pid="" uid="" stray="" i
  for i in 1 2 3 4 5; do
    pid="$(systemctl show -p MainPID --value "$AGENT_UNIT" 2>/dev/null || true)"
    if [[ -n "$pid" && "$pid" != "0" ]]; then break; fi
    sleep 1
  done
  if [[ -z "$pid" || "$pid" == "0" ]]; then
    echo "warning: $AGENT_UNIT has no running process yet; check: journalctl -u croncompose-agent -n 50" >&2
    return 0
  fi
  uid="$(awk '/^Uid:/ {print $2; exit}' "${PROC_ROOT:-/proc}/$pid/status" 2>/dev/null || true)"
  if [[ "$want_root" == "1" && "$uid" != "0" ]]; then
    echo "warning: asked for a root agent but pid $pid runs as uid ${uid:-unknown}; check: systemctl cat $AGENT_UNIT" >&2
  elif [[ "$want_root" != "1" && "$uid" == "0" ]]; then
    echo "warning: asked for a non-root agent but pid $pid runs as root; check: systemctl cat $AGENT_UNIT" >&2
  else
    echo "==> agent pid $pid runs as uid ${uid:-unknown}"
  fi
  stray="$(pgrep -f '(^|/)(croncompose-agent|agent) run$' 2>/dev/null | grep -vx "$pid" | tr '\n' ' ' || true)"
  if [[ -n "${stray// /}" ]]; then
    echo "warning: another agent process not managed by systemd is running (pid ${stray}); stop it, two agents on one server fight over its state" >&2
  fi
}

# check_agent_endpoint: warns (never fails) when the address the service really
# dials differs from the one this install asked for (a systemd drop-in overriding
# the unit), or when that address cannot be reached. An agent that cannot dial never
# sends its Hello, so the UI keeps showing its old privileges.
check_agent_endpoint() {
  local want="$CONTROL_PLANE_ADDR" env="" got="" legacy="" drop=""
  env="$(systemctl show -p Environment --value "$AGENT_UNIT" 2>/dev/null || true)"
  got="$(printf '%s\n' $env | sed -n 's/^AGENT_GRPC_ADDR=//p' | tail -1)"
  legacy="$(printf '%s\n' $env | sed -n 's/^CONTROL_PLANE_ADDR=//p' | tail -1)"
  drop="$(systemctl show -p DropInPaths --value "$AGENT_UNIT" 2>/dev/null || true)"
  if [[ -n "$legacy" && "$legacy" != "${got:-$want}" ]]; then
    echo "warning: a stale CONTROL_PLANE_ADDR=$legacy is set and ignored in favour of AGENT_GRPC_ADDR (${drop:-see: systemctl cat $AGENT_UNIT}); remove it" >&2
  fi
  if [[ -n "$got" && "$got" != "$want" ]]; then
    echo "warning: the service dials $got, not the $want this install asked for; a systemd drop-in overrides it (${drop:-see: systemctl cat $AGENT_UNIT})" >&2
    want="$got"
  fi
  local probe_host="${want%:*}" probe_port="${want##*:}"
  if [[ "$want" != *:* ]]; then probe_host="$want" probe_port=443; fi   # a bare host means 443
  if ! timeout 5 bash -c "exec 3<>/dev/tcp/${probe_host}/${probe_port}" 2>/dev/null; then
    echo "warning: cannot open a TCP connection to $want, so the agent will retry forever and never report its privileges. Raw gRPC does not pass Cloudflare's proxy; on the control plane host use AGENT_GRPC_ADDR=127.0.0.1:<port> with AGENT_GRPC_SNI set to the certificate hostname" >&2
  fi
}

# --- Linux -----------------------------------------------------------------

install_linux() {
  local run_as_root="${AGENT_RUN_AS_ROOT:-0}"

  if [[ "$run_as_root" == "1" ]]; then
    echo "==> creating data dir (running as root, no service user)"
    install -d -m 0700 "$DATA_DIR"
    build_from_source
    chown root:root "$BIN_PATH"
  else
    echo "==> creating service user and data dir"
    id -u croncompose >/dev/null 2>&1 || \
      useradd --system --home-dir "$DATA_DIR" --shell /usr/sbin/nologin croncompose
    install -d -o croncompose -g croncompose -m 0700 "$DATA_DIR"

    build_from_source
    chown root:root "$BIN_PATH"
    # Let the service user replace the binary on later source updates.
    chown croncompose:croncompose "$BIN_PATH"
  fi

  echo "==> writing systemd unit"
  local unit_user_lines=""
  if [[ "$run_as_root" != "1" ]]; then
    unit_user_lines=$'User=croncompose\nGroup=croncompose\n'
  fi
  cat >"$UNIT_PATH" <<EOF
[Unit]
Description=CronCompose agent
After=network-online.target
Wants=network-online.target

[Service]
${unit_user_lines}Environment=AGENT_GRPC_ADDR=${CONTROL_PLANE_ADDR}
Environment=AGENT_ENROLL_HTTP=${CONTROL_PLANE_HTTP}
Environment=AGENT_GRPC_SNI=${SNI}
Environment=DATA_DIR=${DATA_DIR}
ExecStart=${BIN_PATH} run
Restart=always
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF

  stop_existing_agent

  echo "==> enrolling"
  # AGENT_VERSION is only for this one-shot enroll process. Do not bake it into
  # the unit: self-update replaces the binary's linked version, and a pinned
  # Environment=AGENT_VERSION would keep Hello reporting the install-time tag
  # forever (UI stuck on "restarting").
  if [[ "$run_as_root" == "1" ]]; then
    AGENT_GRPC_ADDR="$CONTROL_PLANE_ADDR" \
      AGENT_ENROLL_HTTP="$CONTROL_PLANE_HTTP" \
      AGENT_GRPC_SNI="$SNI" \
      DATA_DIR="$DATA_DIR" \
      AGENT_VERSION="$AGENT_VERSION" \
      "$BIN_PATH" enroll --token="$TOKEN"
  else
    sudo -u croncompose \
      AGENT_GRPC_ADDR="$CONTROL_PLANE_ADDR" \
      AGENT_ENROLL_HTTP="$CONTROL_PLANE_HTTP" \
      AGENT_GRPC_SNI="$SNI" \
      DATA_DIR="$DATA_DIR" \
      AGENT_VERSION="$AGENT_VERSION" \
      "$BIN_PATH" enroll --token="$TOKEN"

    _source_agent_sudoers_lib
    install_agent_sudoers croncompose
  fi

  echo "==> starting service"
  start_agent_service
  report_agent_user "$run_as_root"
  check_agent_endpoint

  echo
  if [[ "$run_as_root" == "1" ]]; then
    echo "done, running as root. follow logs with: journalctl -u croncompose-agent -f"
  else
    echo "done. follow logs with: journalctl -u croncompose-agent -f"
  fi
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
        <key>AGENT_GRPC_ADDR</key>
        <string>${CONTROL_PLANE_ADDR}</string>
        <key>AGENT_ENROLL_HTTP</key>
        <string>${CONTROL_PLANE_HTTP}</string>
        <key>AGENT_GRPC_SNI</key>
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
  plutil -lint "$PLIST_PATH" >/dev/null

  echo "==> enrolling"
  AGENT_GRPC_ADDR="$CONTROL_PLANE_ADDR" \
  AGENT_ENROLL_HTTP="$CONTROL_PLANE_HTTP" \
  AGENT_GRPC_SNI="$SNI" \
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

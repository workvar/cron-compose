#!/usr/bin/env bash
# Install passwordless sudo grants so the CronCompose agent can inspect listen
# sockets (Ports page), drive connectors (systemctl, nginx, …), and invoke the
# agent-privctl elevate/demote helper. Never grants NOPASSWD: ALL.
#
# Usage:
#   source this file, then:  install_agent_sudoers <unix-user>
#   or:  ./install/lib/agent_sudoers.sh <unix-user>
#
# Writes /etc/sudoers.d/croncompose-agent when run as root or via sudo.
# Replaces the file when it already contains the managed marker.

AGENT_SUDOERS_MARKER='# CronCompose agent (managed by installer)'
AGENT_SUDOERS_FILE=/etc/sudoers.d/croncompose-agent
# Documented install path. Sudoers pins this exact argv; do not grant NOPASSWD: ALL.
AGENT_PRIVCTL_BIN="${AGENT_PRIVCTL_BIN:-/usr/libexec/croncompose/agent-privctl}"

_as_log() {
  if declare -F step >/dev/null 2>&1; then step "$@"
  else printf '==> %s\n' "$*" >&2; fi
}
_as_ok() {
  if declare -F ok >/dev/null 2>&1; then ok "$@"
  else printf 'ok: %s\n' "$*" >&2; fi
}
_as_warn() {
  if declare -F warn >/dev/null 2>&1; then warn "$@"
  else printf 'warning: %s\n' "$*" >&2; fi
}
_as_dim() {
  if declare -F dim >/dev/null 2>&1; then dim "$@"
  else printf '    %s\n' "$*" >&2; fi
}

# Absolute path for a binary, if present on PATH or in common locations.
_agent_sudoers_bin() {
  local name="$1" p
  p="$(command -v "$name" 2>/dev/null || true)"
  [ -n "$p" ] && { printf '%s' "$p"; return 0; }
  case "$name" in
    ss)
      for p in /usr/bin/ss /usr/sbin/ss; do
        [ -x "$p" ] && { printf '%s' "$p"; return 0; }
      done
      ;;
  esac
  return 1
}

# Comma-separated absolute paths for the agent priv allowlist (see agent privexec.go).
agent_sudoers_path_list() {
  local paths="" name p
  for name in systemctl systemd-analyze ss lsof nginx tee cp mv install ufw; do
    p="$(_agent_sudoers_bin "$name" || true)"
    [ -n "$p" ] || continue
    case ",$paths," in
      *,"$p",*) ;;
      *) paths="${paths:+$paths, }$p" ;;
    esac
  done
  printf '%s' "$paths"
}

agent_sudoers_content() {
  local user="$1" paths privctl="${AGENT_PRIVCTL_BIN}"
  paths="$(agent_sudoers_path_list)"
  printf '%s\n' "$AGENT_SUDOERS_MARKER"
  printf '%s\n' "# Socket inspection (Ports page) and connector lifecycle for user $user."
  if [ -n "$paths" ]; then
    printf '%s ALL=(root) NOPASSWD: %s\n' "$user" "$paths"
  fi
  printf '%s\n' "# Agent root access: only elevate|demote. Never NOPASSWD: ALL."
  printf 'Defaults:%s env_keep += "DATA_DIR CC_RUNTIME_DIR"\n' "$user"
  printf '%s ALL=(root) NOPASSWD: %s elevate, %s demote\n' "$user" "$privctl" "$privctl"
}

_write_agent_sudoers_file() {
  local dest="$1" tmp
  tmp="$(mktemp)"
  chmod 0600 "$tmp"
  agent_sudoers_content "$2" >"$tmp" || { rm -f "$tmp"; return 1; }
  install -m 0440 "$tmp" "$dest"
  rm -f "$tmp"
  if command -v visudo >/dev/null 2>&1; then
    visudo -cf "$dest" >/dev/null || { rm -f "$dest"; return 1; }
  fi
  return 0
}

# Install or refresh sudoers for the user that runs the agent process.
install_agent_sudoers() {
  local user="${1:-}"
  [ -n "$user" ] || { _as_warn "install_agent_sudoers: missing user"; return 1; }
  [ "$(uname -s)" = "Linux" ] || return 0

  local paths dest="$AGENT_SUDOERS_FILE" privctl="${AGENT_PRIVCTL_BIN}"
  paths="$(agent_sudoers_path_list)"
  if [ -z "$paths" ]; then
    _as_warn "no connector binaries found (ss, lsof, systemctl, …); writing privctl grants only"
  fi

  if [ -f "$dest" ] && ! grep -qF "$AGENT_SUDOERS_MARKER" "$dest" 2>/dev/null; then
    _as_warn "$dest exists and is not managed by CronCompose; leaving it unchanged"
    [ -n "$paths" ] && _as_dim "Add manually: $user ALL=(root) NOPASSWD: $paths"
    _as_dim "And: $user ALL=(root) NOPASSWD: $privctl elevate, $privctl demote"
    return 0
  fi

  if [ "$(id -u)" -eq 0 ]; then
    _as_log "Installing agent sudoers for $user"
    _write_agent_sudoers_file "$dest" "$user" || {
      _as_warn "sudoers validation failed; not installed"
      return 1
    }
    _as_ok "wrote $dest"
    return 0
  fi

  if command -v sudo >/dev/null 2>&1; then
    _as_log "Installing agent sudoers for $user (sudo password may be required)"
    local tmp content
    tmp="$(mktemp)"
    chmod 0600 "$tmp"
    agent_sudoers_content "$user" >"$tmp" || { rm -f "$tmp"; return 1; }
    if sudo install -m 0440 "$tmp" "$dest" && sudo visudo -cf "$dest" >/dev/null 2>&1; then
      rm -f "$tmp"
      _as_ok "wrote $dest"
      return 0
    fi
    rm -f "$tmp"
    sudo rm -f "$dest" 2>/dev/null || true
    _as_warn "could not install $dest (sudo denied or visudo rejected the file)"
    [ -n "$paths" ] && _as_dim "Add manually: $user ALL=(root) NOPASSWD: $paths"
    _as_dim "And: $user ALL=(root) NOPASSWD: $privctl elevate, $privctl demote"
    return 1
  fi

  _as_warn "cannot write $dest without root"
  [ -n "$paths" ] && _as_dim "Add manually: $user ALL=(root) NOPASSWD: $paths"
  _as_dim "And: $user ALL=(root) NOPASSWD: $privctl elevate, $privctl demote"
  return 1
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  set -euo pipefail
  install_agent_sudoers "${1:-}"
fi

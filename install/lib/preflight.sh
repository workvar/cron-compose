#!/usr/bin/env bash
# Preflight: detect the platform and verify build prerequisites. Sets globals:
#   PLATFORM (linux|macos), ARCH, HAVE_PSQL, HAVE_DOCKER, HAVE_PG_ISREADY.

detect_platform() {
  ARCH="$(uname -m)"
  case "$(uname -s)" in
    Linux)  PLATFORM="linux" ;;
    Darwin) PLATFORM="macos" ;;
    *)      PLATFORM="unknown" ;;
  esac
  ok "platform: $PLATFORM/$ARCH"
}

# Compare two dotted versions; returns 0 if $1 >= $2. Pads missing components.
version_ge() {
  local a="$1" b="$2" IFS=.
  # shellcheck disable=SC2086
  set -- $a; local a1="${1:-0}" a2="${2:-0}"
  set -- $b; local b1="${1:-0}" b2="${2:-0}"
  if [ "$a1" -ne "$b1" ]; then [ "$a1" -gt "$b1" ]; return; fi
  [ "$a2" -ge "$b2" ]
}

check_go() {
  command -v go >/dev/null 2>&1 || die "Go is required (need 1.25+). Install from https://go.dev/dl/ (or 'brew install go' / your package manager), then re-run."
  local v; v="$(go version 2>/dev/null | awk '{print $3}' | sed 's/^go//')"
  if version_ge "$v" "1.25"; then ok "go $v"
  else die "Go $v found but 1.25+ is required. Upgrade from https://go.dev/dl/ and re-run."; fi
}

check_node() {
  command -v node >/dev/null 2>&1 || die "Node.js is required (need 20+). Install from https://nodejs.org/ (or 'brew install node' / nvm), then re-run."
  local v; v="$(node --version 2>/dev/null | sed 's/^v//')"
  if version_ge "$v" "20"; then ok "node $v"
  else die "Node $v found but 20+ is required. Upgrade from https://nodejs.org/ and re-run."; fi
  command -v npm >/dev/null 2>&1 || die "npm not found alongside Node. Reinstall Node.js."
  ok "npm $(npm --version 2>/dev/null)"
}

check_curl() {
  command -v curl >/dev/null 2>&1 || die "curl is required for health checks and agent enrollment."
}

# Probe optional tooling that unlocks extra database options. Never fatal.
detect_db_tools() {
  HAVE_PSQL=0; HAVE_DOCKER=0; HAVE_PG_ISREADY=0
  command -v psql >/dev/null 2>&1 && { HAVE_PSQL=1; ok "psql detected (lets the installer create a database in your local PostgreSQL)"; }
  command -v pg_isready >/dev/null 2>&1 && HAVE_PG_ISREADY=1
  if command -v docker >/dev/null 2>&1; then HAVE_DOCKER=1; dim "docker detected (a containerized Postgres is available via --advanced)"; fi
  detect_pkg_manager
}

# Find a package manager we can install PostgreSQL with. Sets PKG_MGR + HAVE_PKG.
detect_pkg_manager() {
  PKG_MGR=""; HAVE_PKG=0
  if [ "$PLATFORM" = "macos" ]; then
    command -v brew >/dev/null 2>&1 && PKG_MGR="brew"
  else
    for m in apt-get dnf yum pacman apk zypper; do
      if command -v "$m" >/dev/null 2>&1; then PKG_MGR="$m"; break; fi
    done
  fi
  if [ -n "$PKG_MGR" ]; then HAVE_PKG=1; ok "$PKG_MGR detected (the installer can install + configure PostgreSQL for you)"; fi
  if [ "$HAVE_PSQL" = "0" ] && [ "$HAVE_PKG" = "0" ]; then
    dim "no local PostgreSQL tooling found: you'll supply a Postgres connection string"
  fi
  return 0  # trailing tests must not set a non-zero status under `set -e`
}

run_preflight() {
  step "Checking prerequisites"
  detect_platform
  check_go
  check_node
  check_curl
  detect_db_tools
}

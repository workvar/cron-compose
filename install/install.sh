#!/usr/bin/env bash
# CronCompose interactive installer for Linux and macOS.
#
# Builds the control plane (Postgres-backed Go API + Next.js web UI) from source,
# applies database migrations, starts everything, and optionally enrolls a local
# agent on this machine. Run it from a checkout of the repo:
#
#   ./install/install.sh
#
# Flags:
#   --advanced          ask the long-form questions (runtime dir, the
#                       database method menu, log level, OIDC, extra env vars)
#   --non-interactive   accept defaults / CC_* env vars without prompting
#   --no-agent          do not enroll/run a local agent (control plane only)
#   --no-web            do not build or run the web UI (API-only install)
#   --runtime-dir DIR   where to keep logs, pids, TLS, and agent data
#   -h, --help          show this help
#
# In --non-interactive mode, values come from CC_* environment variables (see
# install/README.md) falling back to sensible defaults.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LIB_DIR="$SCRIPT_DIR/lib"

# Defaults (overridable by flags).
NONINTERACTIVE=0
ENABLE_AGENT=1
ENABLE_WEB=1
ADVANCED=0

usage() { awk 'NR>1{ if ($0 !~ /^#/) exit; sub(/^# ?/, ""); print }' "${BASH_SOURCE[0]}"; exit 0; }

while [ $# -gt 0 ]; do
  case "$1" in
    --non-interactive) NONINTERACTIVE=1 ;;
    --advanced)        ADVANCED=1 ;;
    --no-agent)        ENABLE_AGENT=0 ;;
    --no-web)          ENABLE_WEB=0 ;;
    --runtime-dir)     shift; CC_RUNTIME_DIR="${1:-}" ;;
    -h|--help)         usage ;;
    *) echo "unknown option: $1 (try --help)" >&2; exit 2 ;;
  esac
  shift
done
export NONINTERACTIVE ENABLE_AGENT ENABLE_WEB ADVANCED

# shellcheck source=lib/common.sh
. "$LIB_DIR/common.sh"
# shellcheck source=lib/preflight.sh
. "$LIB_DIR/preflight.sh"
# shellcheck source=lib/pgdetect.sh
. "$LIB_DIR/pgdetect.sh" # local PostgreSQL discovery, used by configure.sh
# shellcheck source=lib/url.sh
. "$LIB_DIR/url.sh"       # advertise-host parsing, used by configure.sh and services.sh
# shellcheck source=lib/configure.sh
. "$LIB_DIR/configure.sh"
# shellcheck source=lib/configure_db.sh
. "$LIB_DIR/configure_db.sh" # the database question (configure_database)
# shellcheck source=lib/database.sh
. "$LIB_DIR/database.sh"
# shellcheck source=lib/codegen.sh
. "$LIB_DIR/codegen.sh"   # needs need_root/db_run from database.sh, PKG_MGR from preflight
# shellcheck source=lib/build.sh
. "$LIB_DIR/build.sh"
# shellcheck source=lib/cleanup.sh
. "$LIB_DIR/cleanup.sh"
# shellcheck source=lib/pm2.sh
. "$LIB_DIR/pm2.sh"       # process management: pm2 + ecosystem.config.js
# shellcheck source=lib/services.sh
. "$LIB_DIR/services.sh"

banner() {
  printf '%s\n' "$C_CYAN$C_BOLD" >&2
  cat >&2 <<'ART'
   ___                 ___
  / __|_ _ ___ _ _    / __|___ _ __  _ __  ___ ___ ___
 | (__| '_/ _ \ ' \  | (__/ _ \ '  \| '_ \/ _ (_-</ -_)
  \___|_| \___/_||_|  \___\___/_|_|_| .__/\___/__/\___|
                                    |_|   installer
ART
  printf '%s' "$C_RESET" >&2
  dim "Builds and runs the CronCompose control plane from source."
  dim "Single entry point: the control plane serves the UI (/app) and REST (/api) on the"
  dim "backend HTTP port; agents connect on the gRPC port."
  [ "$ADVANCED" = "1" ] && dim "Mode: advanced (all questions)." || dim "Asks URL, ports, database, and admin login. --advanced asks the rest."
  [ "$ENABLE_AGENT" = "1" ] && dim "Scope: control plane + a local agent on this machine."
  [ "$NONINTERACTIVE" = "1" ] && dim "Mode: non-interactive (using defaults / CC_* env)."
  return 0  # never let a false test above abort the script under `set -e`
}

main() {
  banner
  run_preflight
  stop_existing_stack  # free a prior install's ports before we probe (avoids collisions)
  run_configure        # sets ports, secrets, DATABASE_URL, writes .env
  provision_database   # create/start the DB (no-op for an existing DSN)
  run_build            # go binaries + web
  apply_migrations     # uses the freshly built migrate tool
  cleanup_build_tree "$REPO_ROOT"
  run_services         # generate ctl script, start stack, enroll agent, summary
}

main "$@"

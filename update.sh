#!/usr/bin/env bash
# CronCompose one-shot updater.
#
# Pulls the latest source, rebuilds, applies database migrations, and restarts
# the stack. The run mode is auto-detected:
#   source   the installer's croncompose-ctl.sh is present -> rebuild Go binaries
#            and the web UI from source, migrate, `croncompose-ctl.sh restart`.
#   compose  docker-compose.prod.yml is present -> `docker compose build`, run
#            migrations in a one-off container, then `docker compose up -d`.
#
# Usage: ./update.sh [options]
#   --mode source|compose   force a mode instead of auto-detecting
#   --no-pull               skip `git pull`
#   --no-build              skip rebuilding (binaries / images)
#   --no-migrate            skip database migrations
#   --no-restart            skip (re)starting services
#   --no-web                source mode: skip the web UI build
#   -h, --help              show this help
#
# The whole script is wrapped in a brace group so bash reads it fully before
# executing. That makes it safe to update itself: a `git pull` that rewrites this
# file mid-run cannot affect the already-loaded logic.
{
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="docker-compose.prod.yml"

if [ -t 1 ]; then C_B=$'\033[1m'; C_G=$'\033[32m'; C_Y=$'\033[33m'; C_R=$'\033[31m'; C_0=$'\033[0m'
else C_B=; C_G=; C_Y=; C_R=; C_0=; fi
step() { printf '\n%s==>%s %s\n' "$C_B" "$C_0" "$*"; }
ok()   { printf '  %s\xe2\x9c\x93%s %s\n' "$C_G" "$C_0" "$*"; }
warn() { printf '  %s!%s %s\n' "$C_Y" "$C_0" "$*" >&2; }
die()  { printf '%serror:%s %s\n' "$C_R" "$C_0" "$*" >&2; exit 1; }

usage() {
  cat <<'EOF'
CronCompose one-shot updater: pull, rebuild, migrate, restart. Mode auto-detected
(source = croncompose-ctl.sh present; compose = docker-compose.prod.yml present).

Usage: ./update.sh [options]
  --mode source|compose   force a mode instead of auto-detecting
  --no-pull               skip `git pull`
  --no-build              skip rebuilding (binaries / images)
  --no-migrate            skip database migrations
  --no-restart            skip (re)starting services
  --no-web                source mode: skip the web UI build
  -h, --help              show this help
EOF
  exit "${1:-0}"
}

MODE=auto; DO_PULL=1; DO_BUILD=1; DO_MIGRATE=1; DO_RESTART=1; DO_WEB=1
while [ $# -gt 0 ]; do
  case "$1" in
    --mode)       MODE="${2:?--mode needs a value}"; shift 2 ;;
    --mode=*)     MODE="${1#*=}"; shift ;;
    --no-pull)    DO_PULL=0; shift ;;
    --no-build)   DO_BUILD=0; shift ;;
    --no-migrate) DO_MIGRATE=0; shift ;;
    --no-restart) DO_RESTART=0; shift ;;
    --no-web)     DO_WEB=0; shift ;;
    -h|--help)    usage 0 ;;
    *)            die "unknown option: $1 (try --help)" ;;
  esac
done

cd "$REPO_ROOT"

# ---------------------------------------------------------------------------
# Shared steps
# ---------------------------------------------------------------------------

pull_source() {
  [ "$DO_PULL" = 1 ] || { warn "skipping git pull (--no-pull)"; return 0; }
  step "Updating source (git pull --ff-only)"
  command -v git >/dev/null 2>&1 || die "git not found"
  [ -d .git ] || die "$REPO_ROOT is not a git checkout"
  local before after
  before="$(git rev-parse HEAD 2>/dev/null || echo none)"
  git pull --ff-only \
    || die "git pull failed (uncommitted or diverged changes?). Resolve, or re-run with --no-pull."
  after="$(git rev-parse HEAD 2>/dev/null || echo none)"
  if [ "$before" = "$after" ]; then ok "already up to date (${after:0:9})"
  else ok "updated ${before:0:9} -> ${after:0:9}"; fi
}

# Load .env the same way croncompose-ctl.sh does: first '=' splits key/value,
# values kept verbatim (sourcing would choke on spaces / special characters).
load_env() {
  [ -f "$REPO_ROOT/.env" ] || die "missing .env at $REPO_ROOT (run the installer first)"
  local line key val
  while IFS= read -r line; do
    case "$line" in ''|\#*) continue ;; esac
    key="${line%%=*}"; val="${line#*=}"
    [ -n "$key" ] && export "$key=$val"
  done < "$REPO_ROOT/.env"
}

# ---------------------------------------------------------------------------
# Source mode
# ---------------------------------------------------------------------------

build_go_source() {
  step "Building Go binaries"
  command -v go >/dev/null 2>&1 || die "go not found (needed to rebuild from source)"
  export GOTOOLCHAIN=local   # never let `go build` fetch a different toolchain
  ( cd control-plane && go build -o bin/control-plane ./cmd/server ) || die "control-plane build failed"; ok "control-plane"
  ( cd control-plane && go build -o bin/migrate       ./cmd/migrate ) || die "migrate build failed";       ok "migrate"
  ( cd cli           && go build -o bin/cc            ./cmd/cc      ) || die "cli build failed";           ok "cc"
  if [ "${CC_ENABLE_AGENT:-0}" = 1 ] && [ -d agent ]; then
    ( cd agent && go build -o bin/agent ./cmd/agent ) || die "agent build failed"; ok "agent"
  fi
}

build_web_source() {
  [ "$DO_WEB" = 1 ] && [ "${CC_ENABLE_WEB:-1}" = 1 ] || { warn "skipping web build"; return 0; }
  step "Building web UI (npm install + next build)"
  command -v npm >/dev/null 2>&1 || die "npm not found (needed to rebuild the web UI)"
  ( cd web && npm install --no-audit --no-fund ) || die "npm install failed"
  # API_BASE is inlined at build time by next.config.ts, so set it from .env here.
  ( cd web && API_BASE="${API_BASE:-}" NEXT_TELEMETRY_DISABLED=1 npm run build ) || die "next build failed"
  # output:"standalone" -> runtime is web/.next/standalone/server.js; static and
  # public/ must sit alongside it (mirrors web/Dockerfile and the installer).
  local std="web/.next/standalone"
  rm -rf "$std/.next/static" && cp -r web/.next/static "$std/.next/static"
  [ -d web/public ] && { rm -rf "$std/public" && cp -r web/public "$std/public"; }
  ok "web built (standalone runtime staged)"
}

migrate_source() {
  [ "$DO_MIGRATE" = 1 ] || { warn "skipping migrations (--no-migrate)"; return 0; }
  step "Applying database migrations"
  local bin="control-plane/bin/migrate"
  [ -x "$bin" ] || die "migrate tool not built ($bin); run without --no-build"
  [ -n "${DATABASE_URL:-}" ] || die "DATABASE_URL not set in .env"
  DATABASE_URL="$DATABASE_URL" "$bin" -dir "$REPO_ROOT/migrations" \
    || die "migrations failed (is the database reachable and can the role create tables?)"
  ok "schema is up to date"
}

restart_source() {
  [ "$DO_RESTART" = 1 ] || { warn "skipping restart (--no-restart)"; return 0; }
  step "Restarting services (croncompose-ctl.sh restart)"
  [ -x "$REPO_ROOT/croncompose-ctl.sh" ] || die "croncompose-ctl.sh not found or not executable"
  "$REPO_ROOT/croncompose-ctl.sh" restart || die "croncompose-ctl.sh restart failed"
  if [ "${CC_ENABLE_AGENT:-0}" = 1 ] && [ "$(uname -s)" = "Linux" ]; then
    # shellcheck source=install/lib/agent_sudoers.sh
    . "$REPO_ROOT/install/lib/agent_sudoers.sh"
    install_agent_sudoers "$(id -un)" || true
  fi
}

run_source() {
  load_env
  if [ "$DO_BUILD" = 1 ]; then build_go_source; build_web_source; else warn "skipping build (--no-build)"; fi
  migrate_source
  restart_source
}

# ---------------------------------------------------------------------------
# Compose mode
# ---------------------------------------------------------------------------

compose_cmd() {
  if docker compose version >/dev/null 2>&1; then printf 'docker compose'
  elif command -v docker-compose >/dev/null 2>&1; then printf 'docker-compose'
  else return 1; fi
}

run_compose() {
  local DC; DC="$(compose_cmd)" || die "docker compose not available"
  [ -f "$REPO_ROOT/$COMPOSE_FILE" ] || die "$COMPOSE_FILE not found"

  if [ "$DO_BUILD" = 1 ]; then
    step "Building images ($DC build)"
    $DC -f "$COMPOSE_FILE" build || die "image build failed"
    ok "images built"
  else warn "skipping build (--no-build)"; fi

  if [ "$DO_MIGRATE" = 1 ]; then
    step "Applying database migrations"
    $DC -f "$COMPOSE_FILE" up -d postgres || die "failed to start postgres"
    local i=0
    until $DC -f "$COMPOSE_FILE" exec -T postgres pg_isready -U croncompose >/dev/null 2>&1; do
      i=$((i + 1)); [ "$i" -ge 60 ] && die "postgres did not become ready in time"; sleep 1
    done
    # The control-plane image bundles the migrate binary + /migrations.
    $DC -f "$COMPOSE_FILE" run --rm --no-deps --entrypoint /usr/local/bin/migrate \
      control-plane -dir /migrations || die "migrations failed"
    ok "schema is up to date"
  else warn "skipping migrations (--no-migrate)"; fi

  if [ "$DO_RESTART" = 1 ]; then
    step "Starting / restarting services ($DC up -d)"
    $DC -f "$COMPOSE_FILE" up -d || die "compose up failed"
    ok "stack is up; the control plane is the published entry point"
  else warn "skipping restart (--no-restart)"; fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

detect_mode() {
  if [ "$MODE" != auto ]; then printf '%s' "$MODE"; return 0; fi
  if [ -f "$REPO_ROOT/croncompose-ctl.sh" ] && [ -f "$REPO_ROOT/.env" ]; then printf 'source'; return 0; fi
  if [ -f "$REPO_ROOT/$COMPOSE_FILE" ] && compose_cmd >/dev/null 2>&1; then printf 'compose'; return 0; fi
  die "could not detect run mode. Expected croncompose-ctl.sh + .env (source) or docker compose + $COMPOSE_FILE (compose). Use --mode."
}

main() {
  pull_source
  local mode; mode="$(detect_mode)"
  step "Mode: $mode"
  case "$mode" in
    source)  run_source ;;
    compose) run_compose ;;
    *)       die "unknown mode: $mode (use --mode source|compose)" ;;
  esac
  step "Done"
  ok "CronCompose update complete ($mode mode)"
}

main
exit 0
}

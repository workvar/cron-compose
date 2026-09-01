#!/usr/bin/env bash
# Removes everything ./install/install.sh put on this machine, wherever it landed:
# pm2 processes and their boot entry, systemd units, the runtime directory (which may
# sit outside the repo), the generated control script, built artifacts, the database
# and its role, and finally .env.
#
#   ./uninstall.sh              # interactive; asks before dropping the database
#   ./uninstall.sh --yes        # no prompts, drops the database too
#   ./uninstall.sh --keep-db    # remove everything except the database
#   ./uninstall.sh --dry-run    # print what would be removed, touch nothing
#
# The Postgres server package itself is never uninstalled: other things on this box
# may be using it. Same for pm2 and Node.
set -u

REPO_ROOT="$(cd "$(dirname "$0")" && pwd)"
ASSUME_YES=0
KEEP_DB=0
DRY_RUN=0

usage() { awk 'NR>1{ if ($0 !~ /^#/) exit; sub(/^# ?/, ""); print }' "$0"; exit 0; }

while [ $# -gt 0 ]; do
  case "$1" in
    --yes|-y)  ASSUME_YES=1 ;;
    --keep-db) KEEP_DB=1 ;;
    --dry-run) DRY_RUN=1 ;;
    -h|--help) usage ;;
    *) echo "unknown option: $1 (try --help)" >&2; exit 2 ;;
  esac
  shift
done

C_RED=$'\033[31m'; C_GREEN=$'\033[32m'; C_YELLOW=$'\033[33m'; C_CYAN=$'\033[36m'
C_DIM=$'\033[2m'; C_BOLD=$'\033[1m'; C_RESET=$'\033[0m'
step() { printf '\n%s==>%s %s%s%s\n' "$C_CYAN" "$C_RESET" "$C_BOLD" "$*" "$C_RESET"; }
ok()   { printf '  %s✓%s %s\n' "$C_GREEN" "$C_RESET" "$*"; }
warn() { printf '  %s!%s %s\n' "$C_YELLOW" "$C_RESET" "$*"; }
dim()  { printf '    %s%s%s\n' "$C_DIM" "$*" "$C_RESET"; }
skip() { printf '  %s-%s %s\n' "$C_DIM" "$C_RESET" "$*"; }

# Every destructive action goes through this, so --dry-run is honoured everywhere.
run() {
  if [ "$DRY_RUN" = "1" ]; then printf '  %s[dry-run]%s %s\n' "$C_DIM" "$C_RESET" "$*"; return 0; fi
  "$@"
}

# rm + report, honouring --dry-run (which prints the command instead of claiming
# the file is gone).
drop_path() { # <path> <label>
  [ -e "$1" ] || return 0
  run rm -rf "$1" || { warn "could not remove $1"; return 0; }
  [ "$DRY_RUN" = "1" ] || ok "removed $2"
}

confirm() { # <question>
  [ "$ASSUME_YES" = "1" ] && return 0
  printf '%s [y/N]: ' "$1" > /dev/tty
  local a; read -r a < /dev/tty || a=""
  case "$a" in y|Y|yes|YES) return 0 ;; *) return 1 ;; esac
}

# ---------------------------------------------------------------- read .env ----
# .env is the map of where this install put things: runtime dir, ports, database
# credentials. Read it first, delete it last.
ENV_FILE="$REPO_ROOT/.env"
RUNTIME_DIR="$REPO_ROOT/.run"
DATABASE_URL=""
DB_METHOD=""
if [ -f "$ENV_FILE" ]; then
  while IFS= read -r line; do
    case "$line" in ''|\#*) continue ;; esac
    key="${line%%=*}"; val="${line#*=}"
    case "$key" in
      CC_RUNTIME_DIR) RUNTIME_DIR="$val" ;;
      DATABASE_URL)   DATABASE_URL="$val" ;;
      CC_DB_METHOD)   DB_METHOD="$val" ;;
    esac
  done < "$ENV_FILE"
else
  warn "no .env at $REPO_ROOT (uninstalling what can be found anyway)"
fi

# True when the database is the docker-compose Postgres, so the container and its
# volume are what has to go rather than a DROP DATABASE.
db_is_docker() {
  [ "$DB_METHOD" = "docker" ] && return 0
  # Any other recorded method is authoritative: a compose Postgres that happens to be
  # running on this box is somebody else's, and must not be torn down here.
  [ -z "${DB_METHOD:-}" ] || return 1
  command -v docker >/dev/null 2>&1 || return 1
  [ -f "$REPO_ROOT/docker-compose.yml" ] || return 1
  [ -n "$(docker compose -f "$REPO_ROOT/docker-compose.yml" ps -q postgres 2>/dev/null)" ]
}

# postgres://user:pass@host:port/dbname?params -> the pieces we need to drop it.
parse_db_url() {
  local u="${DATABASE_URL#postgres://}"; u="${u#postgresql://}"
  local creds="${u%%@*}" rest="${u#*@}"
  DB_USER="${creds%%:*}"
  DB_PASS="${creds#*:}"; [ "$DB_PASS" = "$creds" ] && DB_PASS=""
  local hostport="${rest%%/*}" path="${rest#*/}"
  DB_HOST="${hostport%%:*}"
  DB_PORT="${hostport#*:}"; [ "$DB_PORT" = "$hostport" ] && DB_PORT=5432
  DB_NAME="${path%%\?*}"
}

printf '\n%sCronCompose uninstaller%s\n' "$C_BOLD" "$C_RESET"
dim "repo:        $REPO_ROOT"
dim "runtime dir: $RUNTIME_DIR"
[ "$DRY_RUN" = "1" ] && dim "mode:        dry run (nothing will be removed)"

if [ "$ASSUME_YES" != "1" ] && [ "$DRY_RUN" != "1" ]; then
  confirm "Remove the CronCompose install from this machine?" || { echo "aborted."; exit 0; }
fi

# ------------------------------------------------------------------- pm2 ------
step "Stopping pm2 processes"
if command -v pm2 >/dev/null 2>&1; then
  if [ -f "$REPO_ROOT/ecosystem.config.js" ]; then
    run bash -c "cd '$REPO_ROOT' && pm2 delete ecosystem.config.js >/dev/null 2>&1" || true
  fi
  # Catch processes started by name if the config is already gone.
  for proc in croncompose-control-plane croncompose-web croncompose-agent; do
    if pm2 describe "$proc" >/dev/null 2>&1; then run pm2 delete "$proc" >/dev/null 2>&1 || true; fi
  done
  run pm2 save --force >/dev/null 2>&1 || true
  # Only tear down the boot hook when pm2 has nothing left to resurrect; other apps
  # on this box may rely on it.
  if [ "$(pm2 jlist 2>/dev/null | grep -c '"name"')" = "0" ]; then
    if confirm "  pm2 has no other apps. Remove its startup (boot) entry?"; then
      run sudo env PATH="$PATH" pm2 unstartup >/dev/null 2>&1 || run pm2 unstartup >/dev/null 2>&1 || true
      ok "removed pm2 startup entry"
    fi
  else
    skip "pm2 still manages other apps; leaving its startup entry alone"
  fi
  ok "pm2 processes removed"
else
  skip "pm2 not installed"
fi

# --------------------------------------------------------------- systemd ------
step "Removing systemd units"
UNITS_FOUND=0
for unit in croncompose-control-plane.service croncompose-web.service croncompose-agent.service; do
  unit_path="/etc/systemd/system/$unit"
  [ -f "$unit_path" ] || continue
  UNITS_FOUND=1
  run sudo systemctl disable --now "$unit" >/dev/null 2>&1 || true
  run sudo rm -f "$unit_path"
  ok "removed $unit"
done
if [ "$UNITS_FOUND" = "1" ]; then
  run sudo systemctl daemon-reload || true
else
  skip "no systemd units installed"
fi

# ---------------------------------------------------------- legacy daemons ----
# Installs from before the pm2 refactor left pidfiles behind.
if [ -d "$RUNTIME_DIR/run" ]; then
  step "Stopping any leftover pidfile daemons"
  for pf in "$RUNTIME_DIR"/run/*.pid; do
    [ -f "$pf" ] || continue
    pid="$(cat "$pf" 2>/dev/null || true)"
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then run kill "$pid" 2>/dev/null || true; ok "killed $(basename "$pf" .pid) ($pid)"; fi
  done
fi

# -------------------------------------------------------------- database ------
step "Database"
if [ "$KEEP_DB" = "1" ]; then
  skip "--keep-db: leaving the database in place"
elif [ -z "$DATABASE_URL" ]; then
  skip "no DATABASE_URL in .env; nothing to drop"
else
  parse_db_url
  dim "database '$DB_NAME' on $DB_HOST:$DB_PORT (role '$DB_USER')"
  warn "dropping it deletes every job, run, log and stored secret. This cannot be undone."
  DROP=0
  if [ "$ASSUME_YES" = "1" ]; then
    DROP=1
  else
    printf "  Type the database name (%s) to confirm, anything else to keep it: " "$DB_NAME" > /dev/tty
    read -r answer < /dev/tty || answer=""
    [ "$answer" = "$DB_NAME" ] && DROP=1
  fi

  if [ "$DROP" != "1" ]; then
    skip "kept database '$DB_NAME'"
  elif db_is_docker; then
    # Docker-provisioned: the container and its volume are the database.
    run bash -c "cd '$REPO_ROOT' && docker compose down -v" || warn "docker compose down failed"
    ok "removed the Postgres container and its volume"
  elif command -v psql >/dev/null 2>&1; then
    # Terminate open sessions first, or DROP DATABASE fails while the app is connected.
    run env PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres \
      -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '$DB_NAME' AND pid <> pg_backend_pid();" \
      >/dev/null 2>&1 || true
    if run env PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "DROP DATABASE IF EXISTS \"$DB_NAME\";" >/dev/null 2>&1; then
      ok "dropped database '$DB_NAME'"
    elif command -v sudo >/dev/null 2>&1 && id postgres >/dev/null 2>&1; then
      # The app role usually cannot drop its own database; fall back to the superuser.
      run sudo -u postgres psql -c "DROP DATABASE IF EXISTS \"$DB_NAME\";" >/dev/null 2>&1 \
        && ok "dropped database '$DB_NAME' (as postgres)" || warn "could not drop '$DB_NAME'; drop it by hand"
      run sudo -u postgres psql -c "DROP ROLE IF EXISTS \"$DB_USER\";" >/dev/null 2>&1 \
        && ok "dropped role '$DB_USER'" || true
    else
      warn "could not drop '$DB_NAME'; drop it by hand:  dropdb $DB_NAME"
    fi
  else
    warn "psql not found; drop it by hand:  dropdb -h $DB_HOST -p $DB_PORT $DB_NAME"
  fi
  dim "The PostgreSQL server itself was left installed."
fi

# ----------------------------------------------------------------- files ------
step "Removing files"
# Runtime state first: TLS material, agent identity and logs may live outside the repo.
if [ -d "$RUNTIME_DIR" ]; then
  drop_path "$RUNTIME_DIR" "$RUNTIME_DIR"
else
  skip "no runtime directory at $RUNTIME_DIR"
fi

for f in "$REPO_ROOT/croncompose-ctl.sh" "$REPO_ROOT/croncompose-ctl.ps1"; do
  drop_path "$f" "$(basename "$f")"
done

# Build artifacts. The checkout itself is left alone: removing it is the user's call.
for d in "$REPO_ROOT/control-plane/bin" "$REPO_ROOT/agent/bin" "$REPO_ROOT/cli/bin" "$REPO_ROOT/web/.next"; do
  drop_path "$d" "${d#"$REPO_ROOT"/}"
done

# A local agent enrolled by the installer keeps its identity under the runtime dir
# (already gone above); this catches the packaged agent's default location.
for d in /var/lib/croncompose /etc/croncompose; do
  [ -d "$d" ] || continue
  if confirm "  Remove $d?"; then run sudo rm -rf "$d" && { [ "$DRY_RUN" = "1" ] || ok "removed $d"; }; fi
done

# .env last: everything above needed it.
drop_path "$ENV_FILE" ".env"

printf '\n%s============ CronCompose removed ============%s\n' "$C_GREEN" "$C_RESET"
dim "Left in place: this checkout, PostgreSQL, Node, pm2, Docker."
dim "Reinstall any time with: ./install/install.sh"

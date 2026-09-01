#!/usr/bin/env bash
# Shared database wizard for setup.sh: local vs remote, create vs pick existing.

# --- helpers ----------------------------------------------------------------

urlencode() {
  if command -v python3 >/dev/null 2>&1; then
    python3 -c 'import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1], safe=""))' "$1"
  elif command -v node >/dev/null 2>&1; then
    node -e 'process.stdout.write(encodeURIComponent(process.argv[1]))' "$1"
  else
    printf '%s' "$1"
  fi
}

build_database_url() {
  local user enc_pass
  user="$(urlencode "$DB_USER")"
  enc_pass="$(urlencode "$DB_PASS")"
  DATABASE_URL="postgres://${user}:${enc_pass}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE:-disable}"
}

setup_superuser_defaults() {
  if [ -n "${CC_DB_SUPERUSER:-}" ]; then
    DB_SUPERUSER="$CC_DB_SUPERUSER"
  elif [ "${PLATFORM:-}" = "macos" ]; then
    DB_SUPERUSER="$(id -un)"
  else
    DB_SUPERUSER="postgres"
  fi
  DB_SUPER_PASS="${CC_DB_SUPER_PASS:-}"
}

# Connect as superuser to the maintenance DB. Prompts once for a password when needed.
ensure_superuser_session() { # <host> <port>
  DB_HOST="$1"; DB_PORT="$2"
  setup_superuser_defaults
  local super_url="postgres://${DB_SUPERUSER}@${DB_HOST}:${DB_PORT}/postgres?sslmode=${DB_SSLMODE:-disable}"
  [ -n "$DB_SUPER_PASS" ] && super_url="postgres://${DB_SUPERUSER}:$(urlencode "$DB_SUPER_PASS")@${DB_HOST}:${DB_PORT}/postgres?sslmode=${DB_SSLMODE:-disable}"

  if psql -w "$super_url" -tAc "select 1" >/dev/null 2>&1; then
    ok "connected as superuser $DB_SUPERUSER"
    return 0
  fi
  if [ "${NONINTERACTIVE:-0}" = "1" ]; then
    warn "could not connect as superuser (set DB_SUPER_PASS / CC_DB_SUPER_PASS)"
    return 1
  fi
  local tries=0 supw=""
  while [ "$tries" -lt 3 ]; do
    printf '%sPassword for superuser %s%s: ' "$C_BOLD" "$DB_SUPERUSER" "$C_RESET" >/dev/tty
    IFS= read -rs supw </dev/tty || supw=""
    printf '\n' >/dev/tty
    DB_SUPER_PASS="$supw"
    super_url="postgres://${DB_SUPERUSER}:$(urlencode "$DB_SUPER_PASS")@${DB_HOST}:${DB_PORT}/postgres?sslmode=${DB_SSLMODE:-disable}"
    if psql -w "$super_url" -tAc "select 1" >/dev/null 2>&1; then
      ok "connected as superuser $DB_SUPERUSER"
      return 0
    fi
    warn "could not connect; check the password and try again"
    tries=$((tries + 1))
  done
  return 1
}

superuser_url() {
  if [ -n "${DB_SUPER_PASS:-}" ]; then
    printf 'postgres://%s:%s@%s:%s/postgres?sslmode=%s' \
      "$(urlencode "$DB_SUPERUSER")" "$(urlencode "$DB_SUPER_PASS")" "$DB_HOST" "$DB_PORT" "${DB_SSLMODE:-disable}"
  else
    printf 'postgres://%s@%s:%s/postgres?sslmode=%s' \
      "$(urlencode "$DB_SUPERUSER")" "$DB_HOST" "$DB_PORT" "${DB_SSLMODE:-disable}"
  fi
}

list_user_databases() {
  psql -w "$(superuser_url)" -tAc \
    "select datname from pg_database where datistemplate = false order by datname" 2>/dev/null | sed '/^$/d'
}

# Parse postgres://user[:pass]@host[:port]/db?query into connection pieces.
# Sets DB_HOST DB_PORT DB_USER DB_PASS DB_NAME DB_SSLMODE when found.
parse_database_url() { # <url>
  local raw="$1" rest="" userinfo="" hostport="" query=""
  raw="${raw#postgres://}"
  raw="${raw#postgresql://}"

  case "$raw" in
    *@*)
      userinfo="${raw%%@*}"
      rest="${raw#*@}"
      ;;
    *)
      rest="$raw"
      ;;
  esac

  case "$userinfo" in
    *:*)
      DB_USER="${userinfo%%:*}"
      DB_PASS="${userinfo#*:}"
      ;;
    *)
      DB_USER="$userinfo"
      DB_PASS=""
      ;;
  esac

  case "$rest" in
    *?*)
      hostport="${rest%%\?*}"
      query="${rest#*\?}"
      ;;
    *)
      hostport="$rest"
      ;;
  esac
  hostport="${hostport%%/*}"
  DB_NAME="${rest#*/}"
  DB_NAME="${DB_NAME%%\?*}"

  DB_SSLMODE="disable"
  case "$query" in
    *sslmode=*) DB_SSLMODE="${query#*sslmode=}"; DB_SSLMODE="${DB_SSLMODE%%&*}" ;;
  esac

  case "$hostport" in
    \[*\]:*)
      DB_HOST="${hostport#[}"
      DB_HOST="${DB_HOST%]:*}"
      DB_PORT="${hostport##*:}"
      ;;
    *:*)
      DB_HOST="${hostport%%:*}"
      DB_PORT="${hostport##*:}"
      ;;
    *)
      DB_HOST="$hostport"
      DB_PORT="5432"
      ;;
  esac
  [ -z "$DB_HOST" ] && DB_HOST="localhost"
  [ -z "$DB_PORT" ] && DB_PORT="5432"
}

# --- local postgres service -------------------------------------------------

postgres_client_installed() {
  command -v psql >/dev/null 2>&1
}

postgres_server_installed() {
  postgres_client_installed && return 0
  command -v postgres >/dev/null 2>&1 && return 0
  command -v pg_ctl >/dev/null 2>&1 && return 0
  if [ "${PLATFORM:-}" = "macos" ] && command -v brew >/dev/null 2>&1; then
    brew list postgresql@16 >/dev/null 2>&1 && return 0
    brew list postgresql@15 >/dev/null 2>&1 && return 0
    brew list postgresql >/dev/null 2>&1 && return 0
  fi
  return 1
}

start_local_postgres() {
  step "Starting local PostgreSQL"
  case "${PLATFORM:-}" in
    macos)
      command -v brew >/dev/null 2>&1 || die "Homebrew not found; start PostgreSQL manually"
      for svc in postgresql@16 postgresql@15 postgresql@14 postgresql; do
        if brew services start "$svc" >/dev/null 2>&1; then
          ok "started $svc"
          sleep 2
          return 0
        fi
      done
      die "could not start PostgreSQL via brew services (try: brew install postgresql@16)"
      ;;
    linux)
      if command -v systemctl >/dev/null 2>&1; then
        sudo systemctl start postgresql && ok "started postgresql" && sleep 2 && return 0
      fi
      if command -v service >/dev/null 2>&1; then
        sudo service postgresql start && ok "started postgresql" && sleep 2 && return 0
      fi
      die "start PostgreSQL manually (systemctl start postgresql)"
      ;;
    *)
      die "automatic start is not supported on this platform"
      ;;
  esac
}

offer_install_local_postgres() {
  [ "${PLATFORM:-}" = "macos" ] && command -v brew >/dev/null 2>&1 || return 1
  confirm "Install PostgreSQL with Homebrew (postgresql@16)?" y || return 1
  step "Installing PostgreSQL"
  brew install postgresql@16 || die "brew install postgresql@16 failed"
  ok "PostgreSQL installed"
  start_local_postgres
  return 0
}

# --- create / pick flows ------------------------------------------------------

setup_create_new_db() { # local|remote
  local mode="$1"
  step "New database"
  DB_NAME="$(prompt "Database name" "${CC_DB_NAME:-croncompose}")"
  DB_USER="$(prompt "Database role" "${CC_DB_USER:-croncompose}")"
  DB_PASS="$(prompt_secret "Role password (blank = generate)" "${CC_DB_PASS:-}")"
  if [ -z "$DB_PASS" ]; then
    DB_PASS="$(gen_hex 12)"
    ok "generated role password: $DB_PASS"
  fi
  build_database_url
  if [ "$mode" = "local" ]; then
    DB_METHOD="psql"
    ok "will create role '$DB_USER' and database '$DB_NAME' on $DB_HOST:$DB_PORT"
  else
    DB_METHOD="remote_create"
    ok "will create role '$DB_USER' and database '$DB_NAME' on the remote server"
  fi
}

setup_pick_existing_db() { # local|remote
  local mode="$1"
  step "Existing database"

  if [ "$mode" = "local" ]; then
    [ "${HAVE_PSQL:-0}" = "1" ] || die "psql is required to list databases (install postgresql-client)"
    ensure_superuser_session "$DB_HOST" "$DB_PORT" || die "could not connect as superuser to list databases"
    local dbs db_choice n=1
    dbs="$(list_user_databases)"
    if [ -z "$dbs" ]; then
      warn "no databases found; creating a new one instead"
      setup_create_new_db local
      return 0
    fi
    info "Databases on this server:"
    while IFS= read -r db; do
      info "  $n) $db"
      n=$((n + 1))
    done <<< "$dbs"
    db_choice="$(prompt "Pick by name or number" "")"
    if [[ "$db_choice" =~ ^[0-9]+$ ]]; then
      DB_NAME="$(echo "$dbs" | sed -n "${db_choice}p")"
      [ -n "$DB_NAME" ] || die "invalid selection: $db_choice"
    else
      DB_NAME="$db_choice"
    fi
  else
    DB_NAME="$(prompt "Existing database name" "${CC_DB_NAME:-croncompose}")"
  fi

  DB_USER="$(prompt "Role username for CronCompose" "${CC_DB_USER:-croncompose}")"
  DB_PASS="$(prompt_secret "Role password" "${CC_DB_PASS:-}")"
  [ -n "$DB_PASS" ] || die "a password is required for the database role"
  build_database_url
  DB_METHOD="existing"
  db_check_dsn "$DATABASE_URL" || warn "could not verify the connection string yet"
  ok "will use database '$DB_NAME' as role '$DB_USER'"
}

setup_database_local() {
  if detect_local_postgres; then
    ok "PostgreSQL is listening on $LOCAL_PG_HOST:$LOCAL_PG_PORT"
  else
    warn "no PostgreSQL server is listening on this machine"
    if postgres_server_installed || postgres_client_installed; then
      if confirm "Try to start local PostgreSQL now?" y; then
        start_local_postgres
      fi
    elif offer_install_local_postgres; then
      :
    else
      die "install and start PostgreSQL locally, then re-run ./setup.sh"
    fi
    detect_local_postgres || die "PostgreSQL is still not reachable on localhost"
    ok "PostgreSQL is listening on $LOCAL_PG_HOST:$LOCAL_PG_PORT"
  fi

  DB_HOST="$LOCAL_PG_HOST"
  DB_PORT="$LOCAL_PG_PORT"
  DB_SSLMODE="disable"
  setup_superuser_defaults

  step "Local database"
  info "  1) Create a new database for CronCompose (recommended)"
  info "  2) Use an existing database"
  case "$(prompt "Select" "1")" in
    2) setup_pick_existing_db local ;;
    *) setup_create_new_db local ;;
  esac
}

setup_database_remote() {
  step "Remote PostgreSQL"
  dim "Use an admin URL to create a database, or an app URL if the database already exists."

  info "  1) Create a new database on the remote server"
  info "  2) Connect to an existing database (full connection string)"
  case "$(prompt "Select" "1")" in
    2)
      local url=""
      while [ -z "$url" ]; do
        url="$(prompt "DATABASE_URL" "${CC_DATABASE_URL:-}")"
        [ -n "$url" ] || warn "a connection string is required"
      done
      parse_database_url "$url"
      build_database_url
      DB_METHOD="existing"
      db_check_dsn "$DATABASE_URL"
      ok "using remote database '$DB_NAME' at $DB_HOST:$DB_PORT"
      ;;
    *)
      dim "Admin URL example: postgres://admin:pass@db.example.com:5432/postgres?sslmode=require"
      local admin_url=""
      while [ -z "$admin_url" ]; do
        admin_url="$(prompt "Admin connection URL" "${CC_DB_ADMIN_URL:-}")"
        [ -n "$admin_url" ] || warn "an admin connection URL is required to create objects"
      done
      parse_database_url "$admin_url"
      DB_SUPERUSER="$DB_USER"
      DB_SUPER_PASS="$DB_PASS"
      ensure_superuser_session "$DB_HOST" "$DB_PORT" || die "could not connect with the admin URL"
      setup_create_new_db remote
      ;;
  esac
}

# Provision role/db on remote when DB_METHOD=remote_create (same SQL as provision_db_psql).
provision_remote_create() {
  local super_url
  super_url="$(superuser_url)"
  if [ "$(psql -w "$super_url" -tAc "select 1 from pg_roles where rolname='$DB_USER'" 2>/dev/null)" != "1" ]; then
    psql -w "$super_url" -v ON_ERROR_STOP=1 -c "create role \"$DB_USER\" login password '$DB_PASS'" >/dev/null \
      || die "failed to create role $DB_USER"
    ok "created role $DB_USER"
  else
    psql -w "$super_url" -c "alter role \"$DB_USER\" login password '$DB_PASS'" >/dev/null 2>&1
    ok "role $DB_USER already existed (password updated)"
  fi
  if [ "$(psql -w "$super_url" -tAc "select 1 from pg_database where datname='$DB_NAME'" 2>/dev/null)" != "1" ]; then
    psql -w "$super_url" -v ON_ERROR_STOP=1 -c "create database \"$DB_NAME\" owner \"$DB_USER\"" >/dev/null \
      || die "failed to create database $DB_NAME"
    ok "created database $DB_NAME"
  else
    ok "database $DB_NAME already existed"
  fi
}

db_check_dsn() { # <dsn>
  [ "${HAVE_PSQL:-0}" = "1" ] || return 0
  if psql -w "$1" -tAc "select 1" >/dev/null 2>&1; then
    ok "connected to the database"
  else
    warn "could not connect with that string right now"
    return 1
  fi
  return 0
}

# Entry point called by setup.sh
setup_database() {
  step "PostgreSQL"
  detect_db_tools

  if postgres_server_installed; then
    ok "PostgreSQL client/server tools are installed"
  else
    warn "PostgreSQL does not appear to be installed on this machine"
    dim "you can still point at a remote server in the next step"
  fi

  if detect_local_postgres; then
    ok "a PostgreSQL server is listening on $LOCAL_PG_HOST:$LOCAL_PG_PORT"
  else
    dim "no PostgreSQL server is listening locally right now"
  fi

  if [ -n "${CC_DATABASE_URL:-}" ]; then
    parse_database_url "$CC_DATABASE_URL"
    build_database_url
    DB_METHOD="existing"
    ok "using CC_DATABASE_URL"
    return 0
  fi

  step "Where is PostgreSQL?"
  info "  1) Local (this machine)"
  info "  2) Remote (another host or managed service)"
  case "$(prompt "Select" "$(detect_local_postgres && printf 1 || printf 2)")" in
    2) setup_database_remote ;;
    *) setup_database_local ;;
  esac
}

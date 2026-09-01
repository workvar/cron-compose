#!/usr/bin/env bash
# The database question, split out of configure.sh.
#
# Default path (deliberately short):
#   1. Probe for a PostgreSQL already listening on this machine.
#   2. If found, one yes/no: create croncompose role+db with defaults (migrations
#      run later in the install). Declining falls through.
#   3. Otherwise ask for a remote DATABASE_URL.
#
# Package-manager install and Docker Postgres stay behind --advanced (or an
# explicit CC_DB_METHOD=native|docker).
#
# Every path ends with DATABASE_URL set and DB_METHOD one of:
#   psql      create the role + database inside an already-running server
#   native    install the server package, then create the role + database
#   existing  a DSN the user supplied; the installer only applies migrations to it
#   docker    docker-compose Postgres (--advanced / CC_DB_METHOD only)

configure_database() {
  step "Database (PostgreSQL)"
  if [ "${ADVANCED:-0}" = "1" ]; then configure_database_advanced; return 0; fi

  # An explicit DSN in the environment answers the question outright.
  if [ -n "${CC_DATABASE_URL:-}" ]; then
    DB_METHOD="existing"; DATABASE_URL="$CC_DATABASE_URL"
    ok "using the Postgres from CC_DATABASE_URL"
    return 0
  fi

  # Anyone who asked for a specific method by hand gets it, docker included.
  if [ -n "${CC_DB_METHOD:-}" ]; then
    case "$CC_DB_METHOD" in
      native) db_setup_native; return 0 ;;
      docker) db_setup_docker; return 0 ;;
      psql)
        if detect_local_postgres; then
          db_setup_in_local_postgres && return 0
        fi
        die "CC_DB_METHOD=psql but no local PostgreSQL is listening"
        ;;
      existing) db_ask_connection_string; return 0 ;;
    esac
  fi

  info "looking for a PostgreSQL server on this machine..."
  if detect_local_postgres; then
    ok "PostgreSQL is listening on $LOCAL_PG_HOST:$LOCAL_PG_PORT"
    local db_name="${CC_DB_NAME:-croncompose}"
    local db_user="${CC_DB_USER:-croncompose}"
    if confirm "Use it? Creates database '$db_name' / role '$db_user' (defaults), then migrations run later." y; then
      if db_setup_in_local_postgres; then return 0; fi
      warn "could not prepare the local server; paste a connection string instead"
    fi
  else
    dim "nothing is listening on the usual PostgreSQL ports"
  fi

  db_ask_connection_string
}

# Defaults-only setup against the server we just found. Returns non-zero when we
# cannot (no psql), so the caller can fall through to the connection-string prompt.
# Superuser password is left blank: provision_db_psql asks only if the server needs it.
db_setup_in_local_postgres() {
  if [ "${HAVE_PSQL:-0}" != "1" ]; then
    warn "psql is not installed, so the role and database cannot be created for you"
    dim "install the postgresql-client package, or paste a connection string below"
    return 1
  fi

  DB_HOST="$LOCAL_PG_HOST"
  DB_PORT="$LOCAL_PG_PORT"
  DB_NAME="${CC_DB_NAME:-croncompose}"
  DB_USER="${CC_DB_USER:-croncompose}"
  DB_PASS="${CC_DB_PASS:-$(gen_hex 12)}"
  if [ -n "${CC_DB_SUPERUSER:-}" ]; then
    DB_SUPERUSER="$CC_DB_SUPERUSER"
  elif [ "${PLATFORM:-}" = "macos" ]; then
    DB_SUPERUSER="$(id -un)"
  else
    DB_SUPERUSER="postgres"
  fi
  DB_SUPER_PASS="${CC_DB_SUPER_PASS:-}"

  DB_METHOD="psql"
  DATABASE_URL="postgres://$DB_USER:$DB_PASS@$DB_HOST:$DB_PORT/$DB_NAME?sslmode=disable"
  ok "will create '$DB_NAME' (role '$DB_USER') in PostgreSQL at $DB_HOST:$DB_PORT"
  dim "generated role password is written to .env as part of DATABASE_URL"
  return 0
}

# Install the server package, then create the role and database (database.sh does both).
db_setup_native() {
  [ "${HAVE_PKG:-0}" = "1" ] || die "no package manager here to install PostgreSQL with; re-run and supply a connection string"
  DB_METHOD="native"
  DB_HOST="127.0.0.1"
  DB_PORT="${CC_DB_PORT:-5432}"
  DB_NAME="${CC_DB_NAME:-croncompose}"
  DB_USER="${CC_DB_USER:-croncompose}"
  DB_PASS="${CC_DB_PASS:-$(gen_hex 12)}"
  DATABASE_URL="postgres://$DB_USER:$DB_PASS@$DB_HOST:$DB_PORT/$DB_NAME?sslmode=disable"
  ok "will install PostgreSQL with $PKG_MGR and create database '$DB_NAME'"
}

# Opt-in only. The credentials are fixed by docker-compose.yml.
db_setup_docker() {
  [ "${HAVE_DOCKER:-0}" = "1" ] || die "CC_DB_METHOD=docker but docker is not on PATH"
  DB_METHOD="docker"
  DB_HOST="127.0.0.1"
  DB_NAME="croncompose"; DB_USER="croncompose"; DB_PASS="croncompose"
  DB_PORT="$(find_free_port "${CC_DB_PORT:-5432}")"
  DATABASE_URL="postgres://$DB_USER:$DB_PASS@$DB_HOST:$DB_PORT/$DB_NAME?sslmode=disable"
  ok "will start Postgres in Docker on $DB_HOST:$DB_PORT"
}

# Fallback: point at a Postgres that already exists (local declined or none found).
db_ask_connection_string() {
  step "Connection string"
  dim "Example: postgres://user:password@host:5432/croncompose?sslmode=disable"
  dim "The database must already exist; the installer only applies migrations to it."
  if [ "${NONINTERACTIVE:-0}" = "1" ]; then
    die "no local PostgreSQL accepted. Re-run with CC_DATABASE_URL set, or CC_DB_METHOD=native|docker."
  fi
  local url=""
  while [ -z "$url" ]; do
    url="$(prompt "Remote DATABASE_URL" "")"
    if [ -z "$url" ]; then warn "a connection string is required to continue"; fi
  done
  DB_METHOD="existing"; DATABASE_URL="$url"
  db_check_dsn "$url"
  ok "using the Postgres you provided"
}

# Best-effort reachability check so a typo surfaces here and not fifteen minutes later
# in the migration step. Never fatal: the server may be firewalled off from this shell.
db_check_dsn() { # <dsn>
  [ "${HAVE_PSQL:-0}" = "1" ] || return 0
  if psql -w "$1" -tAc "select 1" >/dev/null 2>&1; then
    ok "connected to it"
  else
    warn "could not connect with that string right now; continuing (migrations will retry)"
  fi
  return 0
}

# The long form, kept for --advanced: pick the method and every name yourself.
configure_database_advanced() {
  local default_method="${CC_DB_METHOD:-}"
  if [ -z "$default_method" ]; then
    if detect_local_postgres; then default_method="psql"
    elif [ "${HAVE_PKG:-0}" = "1" ]; then default_method="native"
    else default_method="existing"; fi
  fi
  if [ "${NONINTERACTIVE:-0}" != "1" ]; then
    if [ "${HAVE_PSQL:-0}" = "1" ]; then   info "1) Create a database in an already-running local PostgreSQL (psql)"; fi
    info "2) Use an existing Postgres (enter a connection string)"
    if [ "${HAVE_PKG:-0}" = "1" ]; then    info "3) Install PostgreSQL for me and create the database ($PKG_MGR; needs sudo)"; fi
    if [ "${HAVE_DOCKER:-0}" = "1" ]; then info "4) Run Postgres in Docker (docker-compose.yml)"; fi
    local def_choice="2"
    case "$default_method" in psql) def_choice="1" ;; native) def_choice="3" ;; docker) def_choice="4" ;; esac
    local choice; choice="$(prompt "Select" "$def_choice")"
    case "$choice" in
      1) [ "${HAVE_PSQL:-0}" = "1" ]   && default_method="psql"   || default_method="existing" ;;
      2) default_method="existing" ;;
      3) [ "${HAVE_PKG:-0}" = "1" ]    && default_method="native" || default_method="existing" ;;
      4) [ "${HAVE_DOCKER:-0}" = "1" ] && default_method="docker" || default_method="existing" ;;
      *) default_method="existing" ;;
    esac
  fi
  DB_METHOD="$default_method"

  case "$DB_METHOD" in
    native)
      DB_HOST="127.0.0.1"; DB_PORT="${CC_DB_PORT:-5432}"
      DB_NAME="$(prompt "Database name to create" "${CC_DB_NAME:-croncompose}")"
      DB_USER="$(prompt "Database role to create" "${CC_DB_USER:-croncompose}")"
      DB_PASS="$(prompt_secret "Password for the role (blank = generate)" "${CC_DB_PASS:-}")"
      if [ -z "$DB_PASS" ]; then DB_PASS="$(gen_hex 12)"; ok "generated db password: $DB_PASS"; fi
      DATABASE_URL="postgres://$DB_USER:$DB_PASS@127.0.0.1:$DB_PORT/$DB_NAME?sslmode=disable"
      ok "will install PostgreSQL with $PKG_MGR and create database '$DB_NAME'"
      ;;
    docker)
      DB_NAME="croncompose"; DB_USER="croncompose"; DB_PASS="croncompose"
      DB_HOST="127.0.0.1"; DB_PORT="$(prompt_port "Host port to expose Postgres on" "${CC_DB_PORT:-5432}")"
      DATABASE_URL="postgres://$DB_USER:$DB_PASS@$DB_HOST:$DB_PORT/$DB_NAME?sslmode=disable"
      ok "will start Postgres in Docker on $DB_HOST:$DB_PORT"
      ;;
    psql)
      if [ -z "${LOCAL_PG_HOST:-}" ]; then detect_local_postgres || true; fi
      DB_HOST="$(prompt "Postgres host" "${CC_DB_HOST:-${LOCAL_PG_HOST:-localhost}}")"
      DB_PORT="$(prompt "Postgres port" "${CC_DB_PORT:-${LOCAL_PG_PORT:-5432}}")"
      local default_su="${CC_DB_SUPERUSER:-}"
      if [ -z "$default_su" ]; then
        if [ "${PLATFORM:-}" = "macos" ]; then default_su="$(id -un)"; else default_su="postgres"; fi
      fi
      DB_SUPERUSER="$(prompt "Superuser to create role/db with" "$default_su")"
      DB_SUPER_PASS="$(prompt_secret "Superuser password (blank if trust/peer auth)" "${CC_DB_SUPER_PASS:-}")"
      DB_NAME="$(prompt "New database name" "${CC_DB_NAME:-croncompose}")"
      DB_USER="$(prompt "New database role" "${CC_DB_USER:-croncompose}")"
      DB_PASS="$(prompt_secret "Password for new role (blank = generate)" "${CC_DB_PASS:-}")"
      if [ -z "$DB_PASS" ]; then DB_PASS="$(gen_hex 12)"; ok "generated db password: $DB_PASS"; fi
      DATABASE_URL="postgres://$DB_USER:$DB_PASS@$DB_HOST:$DB_PORT/$DB_NAME?sslmode=disable"
      ;;
    *)
      DATABASE_URL="$(prompt "DATABASE_URL" "${CC_DATABASE_URL:-postgres://croncompose:croncompose@localhost:5432/croncompose?sslmode=disable}")"
      ;;
  esac
}

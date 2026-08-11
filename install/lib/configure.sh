#!/usr/bin/env bash
# Interactive configuration. Collects ports, admin credentials, secrets, database
# connection, and any extra env vars, then writes a 0600 .env at the repo root.
#
# Reads CC_* env vars as defaults so the whole thing can run --non-interactive.

EXTRA_ENV_LINES=""

configure_runtime() {
  step "Where to keep runtime state"
  RUNTIME_DIR="$(prompt "Runtime directory (logs, pids, TLS, agent data)" "${CC_RUNTIME_DIR:-$REPO_ROOT/.run}")"
  ADVERTISE_HOST="$(prompt "Advertise host/IP (what agents and browsers use to reach this box)" "${CC_ADVERTISE_HOST:-localhost}")"
}

configure_ports() {
  step "Choose ports (a free one is suggested for each)"
  # Thread the ports chosen so far so no two services are offered (or accept) the same
  # one, even when a default is occupied by a leftover daemon from a previous install.
  local taken=""
  info "The control plane is the single public entry: its HTTP port serves the UI (/app)"
  info "and the REST API (/api); agents reach it on the gRPC port. The web UI port is internal."
  API_PORT="$(prompt_port "HTTP port (public: UI + REST API)" "${CC_API_PORT:-8080}" "$taken")"; taken="$taken $API_PORT"
  GRPC_PORT="$(prompt_port "Agent gRPC port (public)" "${CC_GRPC_PORT:-9090}" "$taken")"; taken="$taken $GRPC_PORT"
  WEB_PORT="$(prompt_port "Web UI port (internal)" "${CC_WEB_PORT:-3000}" "$taken")";  taken="$taken $WEB_PORT"
  ok "http=$API_PORT  grpc=$GRPC_PORT  web=$WEB_PORT (internal)"
}

configure_admin() {
  step "Seed administrator account (used to sign in to the UI)"
  ADMIN_EMAIL="$(prompt "Admin email" "${CC_ADMIN_EMAIL:-admin@example.com}")"
  case "$ADMIN_EMAIL" in *@*.*) : ;; *) warn "that doesn't look like an email, continuing anyway" ;; esac
  ADMIN_PASSWORD="$(prompt_secret "Admin password (blank = generate one)" "${CC_ADMIN_PASSWORD:-}")"
  if [ -z "$ADMIN_PASSWORD" ]; then
    ADMIN_PASSWORD="$(gen_hex 12)"
    ADMIN_PASSWORD_GENERATED=1
    ok "generated admin password: $ADMIN_PASSWORD"
  fi
}

configure_secrets() {
  step "Generating secrets"
  SESSION_SECRET="$(gen_hex 32)"
  SECRETS_MASTER_KEY="$(gen_hex 32)"
  ok "SESSION_SECRET and SECRETS_MASTER_KEY generated"
  TLS_HOSTS="${CC_TLS_HOSTS:-localhost,127.0.0.1,$ADVERTISE_HOST}"
  LOG_LEVEL="$(prompt "Log level (debug|info|warn|error)" "${CC_LOG_LEVEL:-info}")"
  # SNI the local agent verifies the server cert against. localhost when advertising
  # locally; otherwise the advertise host (which is also added to TLS_HOSTS above).
  case "$ADVERTISE_HOST" in localhost|127.0.0.1) AGENT_SNI="localhost" ;; *) AGENT_SNI="$ADVERTISE_HOST" ;; esac
}

# Database method: existing connection string, auto-create via psql, or Docker Postgres.
configure_database() {
  step "Database (PostgreSQL)"
  # Default to installing Postgres when we can; otherwise an existing DSN.
  local default_method="${CC_DB_METHOD:-}"
  if [ -z "$default_method" ]; then
    if [ "$HAVE_PKG" = "1" ]; then default_method="native"; else default_method="existing"; fi
  fi
  if [ "${NONINTERACTIVE:-0}" != "1" ]; then
    [ "$HAVE_PKG" = "1" ]    && info "1) Install PostgreSQL for me and create the database ($PKG_MGR; needs sudo)"
    info "2) Use an existing Postgres (enter a connection string)"
    [ "$HAVE_PSQL" = "1" ]   && info "3) Create a database in an already-running local Postgres (psql)"
    [ "$HAVE_DOCKER" = "1" ] && info "4) Run Postgres in Docker (docker-compose.yml)"
    local def_choice="1"; [ "$HAVE_PKG" = "1" ] || def_choice="2"
    local choice; choice="$(prompt "Select" "$def_choice")"
    case "$choice" in
      1) [ "$HAVE_PKG" = "1" ]    && default_method="native"   || default_method="existing" ;;
      2) default_method="existing" ;;
      3) [ "$HAVE_PSQL" = "1" ]   && default_method="psql"     || default_method="existing" ;;
      4) [ "$HAVE_DOCKER" = "1" ] && default_method="docker"   || default_method="existing" ;;
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
      [ -z "$DB_PASS" ] && { DB_PASS="$(gen_hex 12)"; ok "generated db password: $DB_PASS"; }
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
      DB_HOST="$(prompt "Postgres host" "${CC_DB_HOST:-localhost}")"
      DB_PORT="$(prompt "Postgres port" "${CC_DB_PORT:-5432}")"
      DB_SUPERUSER="$(prompt "Superuser to create role/db with" "${CC_DB_SUPERUSER:-postgres}")"
      DB_SUPER_PASS="$(prompt_secret "Superuser password (blank if trust/peer auth)" "${CC_DB_SUPER_PASS:-}")"
      DB_NAME="$(prompt "New database name" "${CC_DB_NAME:-croncompose}")"
      DB_USER="$(prompt "New database role" "${CC_DB_USER:-croncompose}")"
      DB_PASS="$(prompt_secret "Password for new role (blank = generate)" "${CC_DB_PASS:-}")"
      [ -z "$DB_PASS" ] && { DB_PASS="$(gen_hex 12)"; ok "generated db password: $DB_PASS"; }
      DATABASE_URL="postgres://$DB_USER:$DB_PASS@$DB_HOST:$DB_PORT/$DB_NAME?sslmode=disable"
      ;;
    *)
      DATABASE_URL="$(prompt "DATABASE_URL" "${CC_DATABASE_URL:-postgres://croncompose:croncompose@localhost:5432/croncompose?sslmode=disable}")"
      ;;
  esac
}

configure_oidc() {
  if confirm "Configure OIDC single sign-on now?" n; then
    step "OIDC SSO"
    OIDC_ISSUER_URL="$(prompt "OIDC issuer URL" "${CC_OIDC_ISSUER_URL:-}")"
    OIDC_CLIENT_ID="$(prompt "OIDC client id" "${CC_OIDC_CLIENT_ID:-}")"
    OIDC_CLIENT_SECRET="$(prompt_secret "OIDC client secret (blank for public client)" "${CC_OIDC_CLIENT_SECRET:-}")"
    OIDC_REDIRECT_URL="$(prompt "OIDC redirect URL" "http://$ADVERTISE_HOST:$API_PORT/api/v1/auth/oidc/callback")"
    OIDC_DEFAULT_ROLE="$(prompt "Default role for new SSO users" "viewer")"
  fi
}

# "and others": free-form KEY=VALUE pairs appended verbatim to .env.
configure_extras() {
  [ "${NONINTERACTIVE:-0}" = "1" ] && return 0
  if confirm "Add any extra environment variables?" n; then
    step "Extra environment variables (blank line to finish)"
    while :; do
      local line; line="$(prompt "KEY=VALUE" "")"
      [ -z "$line" ] && break
      case "$line" in *=*) EXTRA_ENV_LINES="$EXTRA_ENV_LINES$line"$'\n' ; ok "added ${line%%=*}" ;; *) warn "expected KEY=VALUE, got: $line" ;; esac
    done
  fi
}

write_env_file() {
  step "Writing configuration"
  ENV_FILE="$REPO_ROOT/.env"
  API_BASE="http://127.0.0.1:$API_PORT/api/v1"
  # Externally-reachable address: the control plane's public HTTP port fronts the UI
  # (/app) and REST (/api). The control plane derives PUBLIC_HTTP_URL / the OIDC
  # redirect / TLS SAN from it, and PUBLIC_GRPC_ADDR from this host + the gRPC port.
  PUBLIC_BASE_URL="${CC_PUBLIC_BASE_URL:-http://$ADVERTISE_HOST:$API_PORT}"

  umask 077
  {
    echo "# Generated by install.sh on $(date). Contains secrets, keep private."
    echo "APP_ENV=prod"
    echo "LOG_LEVEL=$LOG_LEVEL"
    echo "DATABASE_URL=$DATABASE_URL"
    echo "HTTP_ADDR=:$API_PORT"
    echo "GRPC_ADDR=:$GRPC_PORT"
    echo "TLS_DIR=$RUNTIME_DIR/tls"
    echo "TLS_HOSTS=$TLS_HOSTS"
    echo "SESSION_SECRET=$SESSION_SECRET"
    echo "SECRETS_MASTER_KEY=$SECRETS_MASTER_KEY"
    echo "SEED_ADMIN_EMAIL=$ADMIN_EMAIL"
    echo "SEED_ADMIN_PASSWORD=$ADMIN_PASSWORD"
    echo "# Single point of change for the externally-reachable address. Edit this one"
    echo "# line (e.g. https://cron.example.com) and restart; it derives the public REST"
    echo "# URL, the OIDC redirect, and the TLS SAN."
    echo "PUBLIC_BASE_URL=$PUBLIC_BASE_URL"
    echo "# web UI (internal; the control plane reverse-proxies /app to it)"
    echo "PORT=$WEB_PORT"
    echo "API_BASE=$API_BASE"
    if [ "${ENABLE_WEB:-1}" = "1" ]; then
      echo "WEB_UPSTREAM=http://127.0.0.1:$WEB_PORT"
    fi
    if [ -n "${OIDC_ISSUER_URL:-}" ]; then
      echo "# OIDC SSO"
      echo "OIDC_ISSUER_URL=$OIDC_ISSUER_URL"
      echo "OIDC_CLIENT_ID=$OIDC_CLIENT_ID"
      echo "OIDC_CLIENT_SECRET=${OIDC_CLIENT_SECRET:-}"
      echo "OIDC_REDIRECT_URL=$OIDC_REDIRECT_URL"
      echo "OIDC_DEFAULT_ROLE=${OIDC_DEFAULT_ROLE:-viewer}"
    fi
    if [ "${ENABLE_AGENT:-0}" = "1" ]; then
      echo "# local agent (enroll + run on this machine)"
      echo "CONTROL_PLANE_HTTP=http://127.0.0.1:$API_PORT/api/v1"
      echo "CONTROL_PLANE_ADDR=127.0.0.1:$GRPC_PORT"
      echo "CONTROL_PLANE_SNI=$AGENT_SNI"
      echo "DATA_DIR=$RUNTIME_DIR/agent"
    fi
    echo "# installer metadata (read by croncompose-ctl)"
    echo "CC_WEB_PORT=$WEB_PORT"
    echo "CC_API_PORT=$API_PORT"
    echo "CC_GRPC_PORT=$GRPC_PORT"
    echo "CC_RUNTIME_DIR=$RUNTIME_DIR"
    echo "CC_ADVERTISE_HOST=$ADVERTISE_HOST"
    echo "CC_ENABLE_AGENT=${ENABLE_AGENT:-0}"
    echo "CC_ENABLE_WEB=${ENABLE_WEB:-1}"
    if [ -n "$EXTRA_ENV_LINES" ]; then
      echo "# extra vars"
      printf '%s' "$EXTRA_ENV_LINES"
    fi
  } > "$ENV_FILE"
  chmod 600 "$ENV_FILE"
  ok "wrote $ENV_FILE (mode 600)"
}

run_configure() {
  configure_runtime
  configure_ports
  configure_admin
  configure_secrets
  configure_database
  configure_oidc
  configure_extras
  write_env_file
}

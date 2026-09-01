#!/usr/bin/env bash
# Interactive configuration: public URL, ports, database, admin login, then a 0600
# .env at the repo root. Reads CC_* env vars as defaults so the whole thing can run
# --non-interactive. --advanced (ADVANCED=1) restores the long form: runtime dir,
# the database method menu, log level, OIDC and free-form env vars.

EXTRA_ENV_LINES=""

# Everything the installer asks: the URL, the ports behind it, the database, and the
# admin login. Anything else (runtime dir, log level, TLS SANs, OIDC, extra env) is
# derived or defaulted, and is still overridable through CC_* environment variables
# or --advanced for the people who need it.

configure_runtime() {
  # Not a question: one directory under the repo unless CC_RUNTIME_DIR says otherwise.
  RUNTIME_DIR="${CC_RUNTIME_DIR:-$REPO_ROOT/.run}"
  if [ "${ADVANCED:-0}" = "1" ]; then
    step "Where to keep runtime state"
    RUNTIME_DIR="$(prompt "Runtime directory (logs, TLS, agent data)" "$RUNTIME_DIR")"
  fi

  step "Where will people reach CronCompose?"
  dim "A URL is fine (https://cron.example.com), so is a hostname or an IP."
  dim "Behind a TLS proxy? Give the public https:// URL; the port below stays internal."
  local advertise_raw
  # The default is deliberately bare: a scheme in the answer means "something fronts
  # me on the standard port", which would wrongly drop the port from every printed URL
  # if it arrived from a default the user never typed.
  advertise_raw="$(prompt "Public URL" "${CC_ADVERTISE_HOST:-localhost}")"
  # Splits a URL / host:port / bare host into scheme + host + port, so nothing later
  # concatenates a scheme onto a value that already had one.
  normalize_advertise_host "$advertise_raw"
}

configure_ports() {
  step "Ports"
  # Backend HTTP is the public entry point (UI at /app, REST at /api). Frontend is
  # the internal Next.js listener the control plane proxies /app to. Agent gRPC is
  # where enrolled agents dial in — always asked, because remote agents use it even
  # when this machine is not running a local agent.
  dim "A free port is suggested for each. Occupied ones are flagged."
  local default_api="${ADVERTISE_PORT:-${CC_API_PORT:-8080}}"
  API_PORT="$(prompt_port "Backend HTTP port (UI + REST API)" "$default_api" "")"
  ADVERTISE_PORT="${ADVERTISE_PORT:+$API_PORT}"   # keep an explicit URL port in step

  local taken="$API_PORT"
  if [ "${ENABLE_WEB:-1}" = "1" ]; then
    WEB_PORT="$(prompt_port "Frontend port (internal web UI)" "${CC_WEB_PORT:-3000}" "$taken")"
    taken="$taken $WEB_PORT"
  else
    WEB_PORT="$(find_free_port "${CC_WEB_PORT:-3000}" "$taken")"
    taken="$taken $WEB_PORT"
  fi
  GRPC_PORT="$(prompt_port "Agent gRPC port" "${CC_GRPC_PORT:-9090}" "$taken")"
  ok "backend=$API_PORT  frontend=$WEB_PORT  agent=$GRPC_PORT"
}

configure_admin() {
  step "Administrator account (your login)"
  ADMIN_EMAIL="$(prompt "Email" "${CC_ADMIN_EMAIL:-admin@example.com}")"
  case "$ADMIN_EMAIL" in *@*.*) : ;; *) warn "that doesn't look like an email, continuing anyway" ;; esac
  ADMIN_PASSWORD="$(prompt_secret "Password (blank = generate one)" "${CC_ADMIN_PASSWORD:-}")"
  if [ -z "$ADMIN_PASSWORD" ]; then
    ADMIN_PASSWORD="$(gen_hex 12)"
    ADMIN_PASSWORD_GENERATED=1
    ok "generated admin password: $ADMIN_PASSWORD"
  fi
}

# Reuse the session and encryption keys from a previous install when one is present.
# Rotating SECRETS_MASTER_KEY would make every stored secret undecryptable, so a
# re-run must never do it silently.
reuse_existing_secrets() {
  local env_file="$REPO_ROOT/.env"
  [ -f "$env_file" ] || return 0
  local prev_session prev_master
  prev_session="$(sed -n 's/^SESSION_SECRET=//p' "$env_file" | head -1)"
  prev_master="$(sed -n 's/^SECRETS_MASTER_KEY=//p' "$env_file" | head -1)"
  [ -n "$prev_session" ] && SESSION_SECRET="$prev_session"
  [ -n "$prev_master" ]  && SECRETS_MASTER_KEY="$prev_master"
  if [ -n "$prev_master" ]; then
    ok "reusing the keys from the existing .env (stored secrets stay readable)"
  fi
  return 0
}

configure_secrets() {
  step "Generating secrets"
  SESSION_SECRET="$(gen_hex 32)"
  SECRETS_MASTER_KEY="$(gen_hex 32)"
  reuse_existing_secrets
  LOG_LEVEL="${CC_LOG_LEVEL:-info}"
  case "$ADVERTISE_HOST" in
    localhost|127.0.0.1) TLS_HOSTS="${CC_TLS_HOSTS:-localhost,127.0.0.1}" ;;
    *)                   TLS_HOSTS="${CC_TLS_HOSTS:-localhost,127.0.0.1,$ADVERTISE_HOST}" ;;
  esac
  # SNI the local agent verifies the server cert against. localhost when advertising
  # locally; otherwise the advertise host (which is also added to TLS_HOSTS above).
  case "$ADVERTISE_HOST" in localhost|127.0.0.1) AGENT_SNI="localhost" ;; *) AGENT_SNI="$ADVERTISE_HOST" ;; esac
  ok "session and encryption keys ready"
}

# The database question lives in configure_db.sh (configure_database).

# Advanced-only: OIDC is configurable after the fact through .env, so it does not
# belong in a four-question install.
configure_oidc() {
  [ "${ADVANCED:-0}" = "1" ] || return 0
  if confirm "Configure OIDC single sign-on now?" n; then
    step "OIDC SSO"
    OIDC_ISSUER_URL="$(prompt "OIDC issuer URL" "${CC_OIDC_ISSUER_URL:-}")"
    OIDC_CLIENT_ID="$(prompt "OIDC client id" "${CC_OIDC_CLIENT_ID:-}")"
    OIDC_CLIENT_SECRET="$(prompt_secret "OIDC client secret (blank for public client)" "${CC_OIDC_CLIENT_SECRET:-}")"
    OIDC_REDIRECT_URL="$(prompt "OIDC redirect URL" "$(public_base_url "$API_PORT")/api/v1/auth/oidc/callback")"
    OIDC_DEFAULT_ROLE="$(prompt "Default role for new SSO users" "viewer")"
  fi
}

# "and others": free-form KEY=VALUE pairs appended verbatim to .env.
configure_extras() {
  [ "${ADVANCED:-0}" = "1" ] || return 0
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
  PUBLIC_BASE_URL="${CC_PUBLIC_BASE_URL:-$(public_base_url "$API_PORT")}"

  umask 077
  {
    echo "# Generated by install.sh on $(date). Contains secrets, keep private."
    env_line APP_ENV prod
    env_line LOG_LEVEL "$LOG_LEVEL"
    env_line DATABASE_URL "$DATABASE_URL"
    env_line HTTP_ADDR ":$API_PORT"
    env_line GRPC_ADDR ":$GRPC_PORT"
    env_line TLS_DIR "$RUNTIME_DIR/tls"
    env_line TLS_HOSTS "$TLS_HOSTS"
    env_line SESSION_SECRET "$SESSION_SECRET"
    env_line SECRETS_MASTER_KEY "$SECRETS_MASTER_KEY"
    env_line SEED_ADMIN_EMAIL "$ADMIN_EMAIL"
    env_line SEED_ADMIN_PASSWORD "$ADMIN_PASSWORD"
    echo "# Single point of change for the externally-reachable address. Edit this one"
    echo "# line (e.g. https://cron.example.com) and restart; it derives the public REST"
    echo "# URL, the OIDC redirect, and the TLS SAN."
    env_line PUBLIC_BASE_URL "$PUBLIC_BASE_URL"
    echo "# web UI (internal; the control plane reverse-proxies /app to it)"
    env_line PORT "$WEB_PORT"
    env_line API_BASE "$API_BASE"
    if [ "${ENABLE_WEB:-1}" = "1" ]; then
      env_line WEB_UPSTREAM "http://127.0.0.1:$WEB_PORT"
    fi
    if [ -n "${OIDC_ISSUER_URL:-}" ]; then
      echo "# OIDC SSO"
      env_line OIDC_ISSUER_URL "$OIDC_ISSUER_URL"
      env_line OIDC_CLIENT_ID "$OIDC_CLIENT_ID"
      env_line OIDC_CLIENT_SECRET "${OIDC_CLIENT_SECRET:-}"
      env_line OIDC_REDIRECT_URL "$OIDC_REDIRECT_URL"
      env_line OIDC_DEFAULT_ROLE "${OIDC_DEFAULT_ROLE:-viewer}"
    fi
    if [ "${ENABLE_AGENT:-0}" = "1" ]; then
      echo "# local agent (enroll + run on this machine)"
      env_line CONTROL_PLANE_HTTP "http://127.0.0.1:$API_PORT/api/v1"
      env_line CONTROL_PLANE_ADDR "127.0.0.1:$GRPC_PORT"
      env_line CONTROL_PLANE_SNI "$AGENT_SNI"
      env_line DATA_DIR "$RUNTIME_DIR/agent"
    fi
    echo "# installer metadata (read by croncompose-ctl)"
    env_line CC_WEB_PORT "$WEB_PORT"
    env_line CC_API_PORT "$API_PORT"
    env_line CC_GRPC_PORT "$GRPC_PORT"
    env_line CC_RUNTIME_DIR "$RUNTIME_DIR"
    env_line CC_ADVERTISE_HOST "$ADVERTISE_HOST"
    env_line CC_ADVERTISE_SCHEME "$ADVERTISE_SCHEME"
    env_line CC_ADVERTISE_PORT "${ADVERTISE_PORT:-}"
    env_line CC_ENABLE_AGENT "${ENABLE_AGENT:-0}"
    env_line CC_ENABLE_WEB "${ENABLE_WEB:-1}"
    env_line CC_DB_METHOD "${DB_METHOD:-existing}"
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

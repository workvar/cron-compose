#!/usr/bin/env bash
# Advertise-host parsing. Users type anything into the "advertise host" prompt:
# a bare host, an IP, a host:port, or a full URL. Everything downstream (TLS SANs,
# gRPC dialing, SNI, printed URLs) needs the pieces separately, so parse once here
# instead of pasting the raw string into "http://$HOST:$PORT" and ending up with
# "http://https://host:8232".

# Sets, from a single user-supplied value:
#   ADVERTISE_HOST     bare host or IP, never a URL
#   ADVERTISE_SCHEME   http | https
#   ADVERTISE_PORT     explicit port from the value, else empty
#   ADVERTISE_PROXIED  1 when the value carried a scheme (something fronts us)
normalize_advertise_host() {
  local raw="${1:-}" scheme="" port=""

  raw="${raw#"${raw%%[![:space:]]*}"}"   # trim leading space
  raw="${raw%"${raw##*[![:space:]]}"}"   # trim trailing space

  case "$raw" in
    http://*)  scheme="http";  raw="${raw#http://}" ;;
    https://*) scheme="https"; raw="${raw#https://}" ;;
    *://*)     raw="${raw#*://}" ;;      # unknown scheme: drop it, assume http
  esac

  raw="${raw%%/*}"                       # path
  raw="${raw%%\?*}"                      # query
  raw="${raw%%#*}"                       # fragment
  raw="${raw##*@}"                       # userinfo

  case "$raw" in
    \[*\]:*) port="${raw##*:}"; raw="${raw%:*}" ;;   # [::1]:8080
    \[*\])   : ;;                                    # [::1]
    *:*:*)   : ;;                                    # bare IPv6, no port
    *:*)     port="${raw##*:}"; raw="${raw%%:*}" ;;  # host:8080
  esac

  ADVERTISE_HOST="${raw:-localhost}"
  ADVERTISE_HOST="${ADVERTISE_HOST#[}"; ADVERTISE_HOST="${ADVERTISE_HOST%]}"
  ADVERTISE_SCHEME="${scheme:-http}"
  ADVERTISE_PORT="$port"
  if [ -n "$scheme" ]; then ADVERTISE_PROXIED=1; else ADVERTISE_PROXIED=0; fi
}

# Bracket IPv6 literals so they can sit in a URL authority.
url_host() {
  case "${ADVERTISE_HOST}" in
    *:*) printf '[%s]' "$ADVERTISE_HOST" ;;
    *)   printf '%s'   "$ADVERTISE_HOST" ;;
  esac
}

# Browser-facing origin, e.g. https://cron.example.com or http://10.0.0.4:8232.
# A value that arrived with a scheme is assumed to be fronted by a proxy on the
# default port, so the local listener port is not appended unless the user spelled
# one out themselves.
public_base_url() { # <local-http-port>
  local port="${ADVERTISE_PORT:-}"
  [ -z "$port" ] && [ "${ADVERTISE_PROXIED:-0}" = "0" ] && port="${1:-}"
  case "${ADVERTISE_SCHEME}:${port}" in http:80|https:443) port="" ;; esac
  printf '%s://%s%s' "$ADVERTISE_SCHEME" "$(url_host)" "${port:+:$port}"
}

# host:port agents dial over gRPC. Always carries a port and never a scheme.
public_grpc_addr() { # <grpc-port>
  printf '%s:%s' "$(url_host)" "$1"
}

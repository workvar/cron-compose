#!/usr/bin/env bash
# Edge mode setup, shared by install.sh and update.sh so nobody edits .env by hand.
#
# Agents reach the control plane over mutual TLS, which a proxy that ends TLS (a
# Cloudflare proxy or tunnel) cannot carry. When the gRPC hostname is behind such a
# proxy, the control plane needs its loopback-only edge listener and must tell new
# agents to use edge mode. This file detects that and writes the settings:
#
#   PUBLIC_GRPC_MODE=edge   new agents use edge mode
#   EDGE_GRPC_ADDR=...      the loopback listener the tunnel route points at
#   PUBLIC_GRPC_ADDR=host   bare host, so agents dial 443 (no port to pin)
#
# CC_GRPC_MODE=edge|mtls|auto overrides detection for this run. A saved
# PUBLIC_GRPC_MODE=mtls is upgraded when detection says edge, because a Tunnel TCP
# route does not answer HTTPS and an earlier run often saved the wrong mode.
# CC_GRPC_MODE=mtls keeps mutual TLS. Standalone on purpose: update.sh does not
# source common.sh, so nothing here depends on it.

# True for hosts a proxy cannot be in front of: loopback, IP literals, .local names.
edge_host_is_local() { # <host>
  case "$1" in ''|localhost|127.*|*.local|*:*) return 0 ;; esac
  case "$1" in *[!0-9.]*) return 1 ;; *) return 0 ;; esac
}

# proxied, plain, or fail. A Tunnel TCP route (tcp://localhost:9077) does not speak
# HTTPS, so curl fails even though Cloudflare is in front. An HTTP hostname on the
# same tunnel answers with Cloudflare's headers.
edge_host_probe() { # <host>
  local host="$1" headers rc=0
  if edge_host_is_local "$host"; then printf plain; return 0; fi
  command -v curl >/dev/null 2>&1 || { printf fail; return 0; }
  headers="$(curl -sI --connect-timeout 4 --max-time 8 "https://$host/" 2>/dev/null)" || rc=$?
  if [ "$rc" -ne 0 ]; then printf fail; return 0; fi
  headers="$(printf '%s\n' "$headers" | tr -d '\r' | tr 'A-Z' 'a-z')"
  if printf '%s\n' "$headers" | grep -Eq '^(server: cloudflare|cf-ray:)'; then
    printf proxied
  else
    printf plain
  fi
}

# True when https://<host>/ answers with Cloudflare's headers. A proxied hostname, and
# a tunnel public hostname that speaks HTTP, both do, and neither can pass a client
# certificate.
edge_host_is_proxied() { # <host>
  [ "$(edge_host_probe "$1")" = proxied ]
}

# Host of an http(s) URL, or of a bare host / host:port.
edge_url_host() { # <url>
  local u="$1"
  u="${u#*://}"
  u="${u%%/*}"
  u="${u%%\?*}"
  case "$u" in *@*) u="${u#*@}" ;; esac
  edge_host_of "$u"
}

# Prints edge or mtls. The optional second host is the control plane's public URL.
# When the gRPC hostname is a Tunnel TCP route, probing it fails; the public URL is
# on the same tunnel and is what shows Cloudflare.
edge_choose_mode() { # <grpc-host> [public-http-host]
  case "$(printf '%s' "${CC_GRPC_MODE:-auto}" | tr 'A-Z' 'a-z')" in
    edge) printf edge; return 0 ;;
    mtls) printf mtls; return 0 ;;
  esac
  if edge_host_is_local "$1"; then printf mtls; return 0; fi
  case "$(edge_host_probe "$1")" in
    proxied) printf edge; return 0 ;;
    plain)   printf mtls; return 0 ;;
  esac
  local http="${2:-}"
  if [ -n "$http" ] && [ "$http" != "$1" ] && ! edge_host_is_local "$http"; then
    if [ "$(edge_host_probe "$http")" = proxied ]; then printf edge; return 0; fi
  fi
  printf mtls
}

_edge_port_in_use() { # <port>
  local p="$1"
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP:"$p" -sTCP:LISTEN >/dev/null 2>&1 && return 0 || return 1
  fi
  if command -v nc >/dev/null 2>&1; then
    nc -z 127.0.0.1 "$p" >/dev/null 2>&1 && return 0 || return 1
  fi
  ( exec 3<>"/dev/tcp/127.0.0.1/$p" ) >/dev/null 2>&1
}

# First free port at or after <start>, skipping the ports in <reserved> (space list).
edge_pick_port() { # <start> [reserved]
  local p="$1" n=0
  while { _edge_port_in_use "$p" || case " ${2:-} " in *" $p "*) true ;; *) false ;; esac; } && [ "$n" -lt 200 ]; do
    p=$((p + 1)); n=$((n + 1))
  done
  printf '%s' "$p"
}

# Value of KEY in an env file (last wins), unquoted, or fails when absent.
edge_env_get() { # <file> <key>
  local line
  line="$(grep -E "^$2=" "$1" 2>/dev/null | tail -1)" || true
  [ -n "$line" ] || return 1
  line="${line#*=}"
  line="${line#\"}"; line="${line%\"}"
  printf '%s' "$line"
}

# Sets KEY=value, replacing the line or appending it. Only plain host/addr values are
# accepted, so no quoting is needed and nothing odd can reach the file.
edge_env_set() { # <file> <key> <value>
  case "$3" in ''|*[!]A-Za-z0-9._:/[-]*) echo "refusing to write an unexpected value for $2" >&2; return 1 ;; esac
  local tmp; tmp="$(mktemp "${TMPDIR:-/tmp}/ccenv.XXXXXX")" || return 1
  K="$2" V="$3" awk '
    BEGIN { k = ENVIRON["K"]; v = ENVIRON["V"] }
    index($0, k "=") == 1 { print k "=" v; done = 1; next }
    { print }
    END { if (!done) print k "=" v }
  ' "$1" > "$tmp" && cat "$tmp" > "$1"
  local rc=$?
  rm -f "$tmp"
  return $rc
}

# Host part of a host:port or [v6]:port value.
edge_host_of() { # <addr>
  local a="$1"
  case "$a" in
    \[*\]*) a="${a#\[}"; printf '%s' "${a%%]*}" ;;
    *:*)    printf '%s' "${a%%:*}" ;;
    *)      printf '%s' "$a" ;;
  esac
}

# Port of a listen address such as ":9077" or "0.0.0.0:9077".
edge_port_of() { # <addr> <default>
  case "$1" in *:[0-9]*) printf '%s' "${1##*:}" ;; *) printf '%s' "$2" ;; esac
}

# For an existing install: switch it to edge mode when agents would cross Cloudflare.
# Idempotent. Prints "edge <port>" when it changed the file, nothing otherwise.
# A saved PUBLIC_GRPC_MODE=mtls is upgraded when detection says edge. CC_GRPC_MODE=mtls
# keeps mutual TLS for this run.
edge_migrate_env() { # <env file>
  local f="$1" addr host http_host base chosen grpc_port port current
  [ -f "$f" ] || return 0

  current="$(edge_env_get "$f" PUBLIC_GRPC_MODE || true)"
  addr="$(edge_env_get "$f" PUBLIC_GRPC_ADDR || true)"
  host="$(edge_host_of "$addr")"
  # Fully configured edge installs are left untouched, including the listener port.
  # The nginx bridge still runs: cloudflared cannot speak plain HTTP/2, and an
  # update.sh that is already loaded only calls this function.
  if [ "$current" = edge ] && [ -n "$addr" ] && [ "$addr" = "$host" ] \
      && edge_env_get "$f" EDGE_GRPC_ADDR >/dev/null; then
    edge_ensure_nginx_bridge "$f" hint || true
    return 0
  fi

  [ -n "$addr" ] || return 0
  http_host=""
  if base="$(edge_env_get "$f" PUBLIC_BASE_URL || true)" && [ -n "$base" ]; then
    http_host="$(edge_url_host "$base")"
  elif base="$(edge_env_get "$f" PUBLIC_HTTP_URL || true)" && [ -n "$base" ]; then
    http_host="$(edge_url_host "$base")"
  fi
  if [ "$current" = edge ]; then
    chosen=edge
  else
    chosen="$(edge_choose_mode "$host" "$http_host")"
  fi
  [ "$chosen" = edge ] || return 0

  grpc_port="$(edge_port_of "$(edge_env_get "$f" GRPC_ADDR || true)" 9090)"
  if ! edge_env_get "$f" EDGE_GRPC_ADDR >/dev/null; then
    port="$(edge_pick_port "$((grpc_port + 1))" "$grpc_port")"
    edge_env_set "$f" EDGE_GRPC_ADDR "127.0.0.1:$port" || return 1
  else
    port="$(edge_port_of "$(edge_env_get "$f" EDGE_GRPC_ADDR)" 9091)"
  fi
  edge_env_set "$f" PUBLIC_GRPC_ADDR "$host" || return 1
  edge_env_set "$f" PUBLIC_GRPC_MODE edge || return 1
  # The caller prints the Cloudflare hint after this returns. Nginx is still
  # installed here so a running update.sh, which only sources this file, sets
  # the bridge up on the same run.
  edge_ensure_nginx_bridge "$f" quiet || true
  printf '%s %s' edge "$port"
}

# cloudflared speaks HTTP/2 only to an https:// origin. The edge listener is plain
# HTTP/2, so nginx on loopback terminates TLS with the control plane certificate
# and forwards gRPC to it. CC_EDGE_NGINX=0 skips this (tests).
edge_nginx_config() { # <listen port> <upstream port> <cert> <key>
  cat <<EOF
# Managed by CronCompose. TLS front for the edge gRPC listener.
server {
    listen 127.0.0.1:${1} ssl http2;
    server_name localhost;
    ssl_certificate     ${3};
    ssl_certificate_key ${4};
    location / {
        grpc_pass grpc://127.0.0.1:${2};
        grpc_read_timeout 1d;
        grpc_send_timeout 1d;
    }
}
EOF
}

edge_as_root() {
  if [ "$(id -u)" -eq 0 ]; then "$@"; else sudo "$@"; fi
}

edge_install_nginx_pkg() {
  command -v nginx >/dev/null 2>&1 && return 0
  local mgr=""
  case "$(uname -s)" in
    Darwin)
      command -v brew >/dev/null 2>&1 || { printf '  nginx: brew is not installed, so the TLS bridge was not set up\n' >&2; return 1; }
      brew install nginx
      return
      ;;
  esac
  local m
  for m in apt-get dnf yum pacman apk zypper; do
    command -v "$m" >/dev/null 2>&1 && mgr="$m" && break
  done
  [ -n "$mgr" ] || { printf '  nginx: no package manager found, so the TLS bridge was not set up\n' >&2; return 1; }
  printf '  installing nginx (%s)\n' "$mgr" >&2
  case "$mgr" in
    apt-get) edge_as_root apt-get update -qq && edge_as_root apt-get install -y nginx ;;
    dnf|yum) edge_as_root "$mgr" install -y nginx ;;
    pacman)  edge_as_root pacman -Sy --noconfirm nginx ;;
    apk)     edge_as_root apk add --no-cache nginx ;;
    zypper)  edge_as_root zypper --non-interactive install nginx ;;
  esac
}

# Installs or refreshes the loopback TLS bridge. Never prints to stdout.
# Pass "hint" to print the Cloudflare steps when the bridge was created or changed.
edge_ensure_nginx_bridge() { # <env file> [hint]
  [ "${CC_EDGE_NGINX:-1}" = 1 ] || return 0
  local f="$1" tls_dir cert key upstream listen dest dir changed=0 body cur host
  [ -f "$f" ] || return 0
  [ "$(edge_env_get "$f" PUBLIC_GRPC_MODE || true)" = edge ] || return 0
  tls_dir="$(edge_env_get "$f" TLS_DIR || true)"
  [ -n "$tls_dir" ] || tls_dir="$(edge_env_get "$f" CC_RUNTIME_DIR || true)/tls"
  cert="$tls_dir/server.crt"
  key="$tls_dir/server.key"
  if [ ! -f "$cert" ] || [ ! -f "$key" ]; then
    printf '  nginx: %s is missing, so the TLS bridge waits until the control plane has started\n' "$cert" >&2
    return 0
  fi
  upstream="$(edge_port_of "$(edge_env_get "$f" EDGE_GRPC_ADDR || true)" "")"
  [ -n "$upstream" ] || return 0
  listen="$(edge_env_get "$f" EDGE_NGINX_PORT || true)"
  if [ -z "$listen" ]; then
    listen="$(edge_pick_port 9443 "$upstream")"
    edge_env_set "$f" EDGE_NGINX_PORT "$listen" || return 1
    changed=1
  fi
  edge_install_nginx_pkg || return 1
  if [ -d /etc/nginx/sites-enabled ] && [ ! -e /etc/nginx/croncompose/.keep-default-site ]; then
    # A fresh nginx package also listens on port 80. This bridge does not use it.
    if [ ! -e /etc/nginx/croncompose/server.crt ]; then
      edge_as_root rm -f /etc/nginx/sites-enabled/default
    fi
  fi
  dir=/etc/nginx/croncompose
  dest=/etc/nginx/conf.d/croncompose-edge.conf
  edge_as_root mkdir -p "$dir" /etc/nginx/conf.d
  if ! edge_as_root cmp -s "$cert" "$dir/server.crt" 2>/dev/null \
      || ! edge_as_root cmp -s "$key" "$dir/server.key" 2>/dev/null; then
    edge_as_root cp "$cert" "$dir/server.crt"
    edge_as_root cp "$key" "$dir/server.key"
    edge_as_root chmod 0644 "$dir/server.crt"
    edge_as_root chmod 0600 "$dir/server.key"
    changed=1
  fi
  body="$(edge_nginx_config "$listen" "$upstream" "$dir/server.crt" "$dir/server.key")"
  cur="$(edge_as_root cat "$dest" 2>/dev/null || true)"
  if [ "$body" != "$cur" ]; then
    printf '%s' "$body" | edge_as_root tee "$dest" >/dev/null
    changed=1
  fi
  edge_as_root nginx -t >&2 || return 1
  if command -v systemctl >/dev/null 2>&1; then
    edge_as_root systemctl enable nginx >/dev/null 2>&1 || true
    if edge_as_root systemctl is-active --quiet nginx; then
      [ "$changed" = 1 ] && edge_as_root systemctl reload nginx
    else
      edge_as_root systemctl restart nginx
      changed=1
    fi
  else
    edge_as_root nginx -s reload >/dev/null 2>&1 || edge_as_root nginx
    changed=1
  fi
  if [ "$changed" = 1 ]; then
    host="$(edge_host_of "$(edge_env_get "$f" PUBLIC_GRPC_ADDR || true)")"
    printf '  nginx is listening on https://127.0.0.1:%s and forwarding to 127.0.0.1:%s\n' "$listen" "$upstream" >&2
    [ "${2:-}" = hint ] && edge_print_cloudflare_hint "$host" "$upstream"
  fi
}

# The one step the scripts cannot do for you: the Cloudflare route.
# The port in the dashboard is the nginx TLS bridge (EDGE_NGINX_PORT), not the
# plain edge listener. cloudflared ignores "HTTP/2 to origin" for an http:// URL.
edge_print_cloudflare_hint() { # <host> <edge port>
  local https=""
  if [ -n "${REPO_ROOT:-}" ] && [ -f "${REPO_ROOT}/.env" ]; then
    https="$(edge_env_get "${REPO_ROOT}/.env" EDGE_NGINX_PORT || true)"
  fi
  {
    printf '\n  Agents reach %s through Cloudflare, so edge mode is on.\n' "$1"
    printf '  One step is left in the Cloudflare dashboard:\n'
    printf '    - Network: turn gRPC on for the zone.\n'
    printf '    - Zero Trust, Networks, Tunnels, your tunnel, Public hostname %s:\n' "$1"
    if [ -n "$https" ]; then
      printf '      URL https://localhost:%s\n' "$https"
      printf '      Additional application settings, TLS:\n'
      printf '        Use HTTP/2 to origin: on\n'
      printf '        Disable TLS certificate verification: on\n'
      printf '      (this replaces an http:// or tcp:// route on that name; the plain\n'
      printf '      listener stays on 127.0.0.1:%s and is not the tunnel origin).\n\n' "$2"
    else
      printf '      URL https://localhost:<EDGE_NGINX_PORT from .env>\n'
      printf '      Use HTTP/2 to origin, and disable TLS certificate verification.\n\n'
    fi
  } >&2
}

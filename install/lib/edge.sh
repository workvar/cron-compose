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
  if [ "$current" = edge ] && [ -n "$addr" ] && [ "$addr" = "$host" ] \
      && edge_env_get "$f" EDGE_GRPC_ADDR >/dev/null; then
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
  printf '%s %s' edge "$port"
}

# The one step the scripts cannot do for you: the Cloudflare route.
edge_print_cloudflare_hint() { # <host> <edge port>
  {
    printf '\n  Agents reach %s through Cloudflare, so edge mode is on.\n' "$1"
    printf '  One step is left in the Cloudflare dashboard:\n'
    printf '    - Network: turn gRPC on for the zone.\n'
    printf '    - Zero Trust, Networks, Tunnels, your tunnel, Public hostname %s:\n' "$1"
    printf '      service type HTTP, URL localhost:%s, and enable "HTTP2 connection"\n' "$2"
    printf '      (this replaces a tcp://localhost:<grpc port> route on that name).\n\n'
  } >&2
}

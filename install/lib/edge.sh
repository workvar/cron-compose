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
# CC_GRPC_MODE=edge|mtls|auto overrides detection. An existing PUBLIC_GRPC_MODE in
# .env is an operator decision and is never changed. Standalone on purpose: update.sh
# does not source common.sh, so nothing here depends on it.

# True for hosts a proxy cannot be in front of: loopback, IP literals, .local names.
edge_host_is_local() { # <host>
  case "$1" in ''|localhost|127.*|*.local|*:*) return 0 ;; esac
  case "$1" in *[!0-9.]*) return 1 ;; *) return 0 ;; esac
}

# True when https://<host>/ answers with Cloudflare's headers. A proxied hostname, and
# a tunnel public hostname, both do, and neither can pass a client certificate.
edge_host_is_proxied() { # <host>
  command -v curl >/dev/null 2>&1 || return 1
  local headers
  headers="$(curl -sI --connect-timeout 4 --max-time 8 "https://$1/" 2>/dev/null | tr -d '\r' | tr 'A-Z' 'a-z')" || true
  printf '%s\n' "$headers" | grep -Eq '^(server: cloudflare|cf-ray:)'
}

# Prints edge or mtls for a gRPC hostname.
edge_choose_mode() { # <host>
  case "$(printf '%s' "${CC_GRPC_MODE:-auto}" | tr 'A-Z' 'a-z')" in
    edge) printf edge; return 0 ;;
    mtls) printf mtls; return 0 ;;
  esac
  if edge_host_is_local "$1"; then printf mtls; return 0; fi
  if edge_host_is_proxied "$1"; then printf edge; else printf mtls; fi
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

# For an existing install: switch it to edge mode when its gRPC hostname is behind a
# proxy and no mode was ever chosen. Idempotent. Prints "edge <port>" when it changed
# the file, nothing otherwise.
edge_migrate_env() { # <env file>
  local f="$1" mode_now addr host grpc_port port
  [ -f "$f" ] || return 0
  if edge_env_get "$f" PUBLIC_GRPC_MODE >/dev/null; then return 0; fi

  addr="$(edge_env_get "$f" PUBLIC_GRPC_ADDR)" || return 0
  host="$(edge_host_of "$addr")"
  [ "$(edge_choose_mode "$host")" = edge ] || return 0

  grpc_port="$(edge_port_of "$(edge_env_get "$f" GRPC_ADDR || true)" 9090)"
  if ! edge_env_get "$f" EDGE_GRPC_ADDR >/dev/null; then
    port="$(edge_pick_port "$((grpc_port + 1))" "$grpc_port")"
    edge_env_set "$f" EDGE_GRPC_ADDR "127.0.0.1:$port" || return 1
  else
    port="$(edge_port_of "$(edge_env_get "$f" EDGE_GRPC_ADDR)" 9091)"
  fi
  edge_env_set "$f" PUBLIC_GRPC_ADDR "$host" || return 1
  edge_env_set "$f" PUBLIC_GRPC_MODE edge || return 1
  mode_now=edge
  printf '%s %s' "$mode_now" "$port"
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

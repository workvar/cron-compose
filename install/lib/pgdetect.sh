#!/usr/bin/env bash
# Local PostgreSQL discovery. Answers one question for configure.sh: is there a
# Postgres server already listening on this machine, and where?
#
# Sets, on success: LOCAL_PG_HOST, LOCAL_PG_PORT. Returns non-zero when nothing is
# listening. Never fatal, never installs anything.

# Candidate ports to probe, in order: an explicit CC_DB_PORT first, then the default.
pg_candidate_ports() {
  local seen=""
  for p in "${CC_DB_PORT:-}" 5432 5433; do
    [ -n "$p" ] || continue
    case " $seen " in *" $p "*) continue ;; esac
    seen="$seen $p"
    printf '%s\n' "$p"
  done
}

# True when something accepts a TCP connection on <host> <port>. Written with explicit
# if-blocks (not `cmd && return`) so a failing probe never trips `set -e` in the caller.
pg_port_open() { # <host> <port>
  local host="$1" port="$2" rc
  if [ "${HAVE_PG_ISREADY:-0}" = "1" ]; then
    pg_isready -h "$host" -p "$port" -t 2 >/dev/null 2>&1 && return 0
    rc=$?
    # Exit 1 is "server is there but rejecting connections" (e.g. still starting, or
    # auth refuses this probe). A server exists, which is what we are asking about.
    if [ "$rc" = "1" ]; then return 0; fi
  fi
  if command -v nc >/dev/null 2>&1; then
    if nc -z -w 2 "$host" "$port" >/dev/null 2>&1; then return 0; fi
  fi
  # Last resort: bash's own TCP support.
  if (exec 3<>"/dev/tcp/$host/$port") >/dev/null 2>&1; then return 0; fi
  return 1
}

# Probe localhost for a running Postgres. Sets LOCAL_PG_HOST / LOCAL_PG_PORT.
detect_local_postgres() {
  LOCAL_PG_HOST=""; LOCAL_PG_PORT=""
  local host port
  for host in 127.0.0.1 localhost; do
    while IFS= read -r port; do
      if pg_port_open "$host" "$port"; then
        LOCAL_PG_HOST="$host"; LOCAL_PG_PORT="$port"
        return 0
      fi
    done < <(pg_candidate_ports)
  done
  return 1
}

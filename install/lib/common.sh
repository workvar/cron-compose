#!/usr/bin/env bash
# Shared helpers for the CronCompose installer: logging, interactive prompts, port
# detection, and secret generation. Kept POSIX-friendly and bash 3.2 compatible so it
# runs on a stock macOS (no associative arrays, no namerefs).

# --- output -----------------------------------------------------------------
# Everything human-facing goes to stderr so prompt return values stay clean for
# command substitution: VALUE="$(prompt "..." "default")".

if [ -t 2 ]; then
  C_RESET="$(printf '\033[0m')"; C_BOLD="$(printf '\033[1m')"
  C_CYAN="$(printf '\033[36m')"; C_GREEN="$(printf '\033[32m')"
  C_YELLOW="$(printf '\033[33m')"; C_RED="$(printf '\033[31m')"; C_DIM="$(printf '\033[2m')"
else
  C_RESET=""; C_BOLD=""; C_CYAN=""; C_GREEN=""; C_YELLOW=""; C_RED=""; C_DIM=""
fi

step()  { printf '\n%s==>%s %s%s%s\n' "$C_CYAN" "$C_RESET" "$C_BOLD" "$*" "$C_RESET" >&2; }
info()  { printf '    %s\n' "$*" >&2; }
ok()    { printf '  %s✓%s %s\n' "$C_GREEN" "$C_RESET" "$*" >&2; }
warn()  { printf '  %s!%s %s\n' "$C_YELLOW" "$C_RESET" "$*" >&2; }
die()   { printf '  %s✗ %s%s\n' "$C_RED" "$*" "$C_RESET" >&2; exit 1; }
dim()   { printf '    %s%s%s\n' "$C_DIM" "$*" "$C_RESET" >&2; }

# --- prompts ----------------------------------------------------------------
# In non-interactive mode prompts return their default without reading.

prompt() { # <label> <default>
  local label="$1" def="$2" ans=""
  if [ "${NONINTERACTIVE:-0}" = "1" ]; then printf '%s' "$def"; return; fi
  if [ -n "$def" ]; then printf '%s%s%s [%s]: ' "$C_BOLD" "$label" "$C_RESET" "$def" >/dev/tty
  else printf '%s%s%s: ' "$C_BOLD" "$label" "$C_RESET" >/dev/tty; fi
  IFS= read -r ans </dev/tty || ans=""
  [ -z "$ans" ] && ans="$def"
  printf '%s' "$ans"
}

prompt_secret() { # <label> <default>
  local label="$1" def="$2" ans=""
  if [ "${NONINTERACTIVE:-0}" = "1" ]; then printf '%s' "$def"; return; fi
  printf '%s%s%s: ' "$C_BOLD" "$label" "$C_RESET" >/dev/tty
  IFS= read -rs ans </dev/tty || ans=""
  printf '\n' >/dev/tty
  [ -z "$ans" ] && ans="$def"
  printf '%s' "$ans"
}

confirm() { # <question> <default y|n> -> exit 0 if yes
  local q="$1" def="${2:-n}" ans="" hint="[y/N]"
  [ "$def" = "y" ] && hint="[Y/n]"
  if [ "${NONINTERACTIVE:-0}" = "1" ]; then [ "$def" = "y" ]; return; fi
  printf '%s%s%s %s ' "$C_BOLD" "$q" "$C_RESET" "$hint" >/dev/tty
  IFS= read -r ans </dev/tty || ans=""
  [ -z "$ans" ] && ans="$def"
  case "$ans" in [Yy]*) return 0 ;; *) return 1 ;; esac
}

# --- ports ------------------------------------------------------------------

port_in_use() { # <port> -> 0 if a listener is bound
  local p="$1"
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP:"$p" -sTCP:LISTEN >/dev/null 2>&1 && return 0 || return 1
  fi
  if command -v nc >/dev/null 2>&1; then
    nc -z 127.0.0.1 "$p" >/dev/null 2>&1 && return 0 || return 1
  fi
  ( exec 3<>"/dev/tcp/127.0.0.1/$p" ) >/dev/null 2>&1 && { exec 3>&- 3<&- 2>/dev/null; return 0; }
  return 1
}

# Membership test: is <port> a word in the space-separated <list>? Used to keep already
# chosen ports out of later suggestions, so two services never share a port even when
# their defaults are occupied (e.g. by a leftover daemon from a previous install). The
# list is threaded through arguments because each prompt runs in a command-substitution
# subshell and cannot mutate a parent global.
_in_list() { case " ${2:-} " in *" $1 "*) return 0 ;; *) return 1 ;; esac; }

find_free_port() { # <start> [reserved-list] -> first free, unreserved port at/after start
  local p="$1" n=0
  while { port_in_use "$p" || _in_list "$p" "${2:-}"; } && [ "$n" -lt 500 ]; do p=$((p + 1)); n=$((n + 1)); done
  printf '%s' "$p"
}

is_valid_port() { case "$1" in ''|*[!0-9]*) return 1 ;; *) [ "$1" -ge 1 ] && [ "$1" -le 65535 ] ;; esac; }

# Prompt for a port, defaulting to the first free, unreserved port at/after <default>.
# <reserved-list> (optional, space-separated) holds ports already given to other services
# this run; the prompt rejects a manual entry that duplicates one. Warns (but allows) if
# the chosen port is merely occupied; reprompts on invalid input.
prompt_port() { # <label> <default-start> [reserved-list]
  local label="$1" start="$2" reserved="${3:-}" suggest chosen
  suggest="$(find_free_port "$start" "$reserved")"
  while :; do
    chosen="$(prompt "$label" "$suggest")"
    if ! is_valid_port "$chosen"; then warn "not a valid port: $chosen"; [ "${NONINTERACTIVE:-0}" = "1" ] && return 1; continue; fi
    if _in_list "$chosen" "$reserved"; then
      warn "port $chosen is already assigned to another CronCompose service"
      suggest="$(find_free_port "$chosen" "$reserved")"
      [ "${NONINTERACTIVE:-0}" = "1" ] && chosen="$suggest" || continue
    fi
    if port_in_use "$chosen"; then
      warn "port $chosen is already in use"
      if [ "${NONINTERACTIVE:-0}" != "1" ]; then confirm "  Use it anyway?" n || continue; fi
    fi
    printf '%s' "$chosen"; return
  done
}

# --- secrets ----------------------------------------------------------------

gen_hex() { # <bytes> -> hex string of length 2*bytes
  local n="${1:-32}"
  if command -v openssl >/dev/null 2>&1; then openssl rand -hex "$n" && return; fi
  if [ -r /dev/urandom ]; then head -c "$n" /dev/urandom | od -An -v -tx1 | tr -d ' \n'; return; fi
  date +%s%N | shasum -a 256 2>/dev/null | awk '{print $1}'
}

# --- env files --------------------------------------------------------------

# Quote a .env value only when a bare KEY=value line would be ambiguous (whitespace,
# #, quotes, dollar, backtick, backslash). Ports, URLs, and hex secrets stay bare so
# pm2, Next.js, and systemd EnvironmentFile all see PORT=3107 rather than PORT="3107".
env_quote() {
  local s=$1
  if [ -n "$s" ] && [[ "$s" =~ [[:space:]#\'\"\\$\`] ]]; then
    s=${s//\\/\\\\}
    s=${s//\"/\\\"}
    printf '"%s"' "$s"
    return
  fi
  printf '%s' "$s"
}

# Inverse of env_quote, for re-reading keys from an existing .env on reinstall.
env_unquote() {
  local v=$1
  local n=${#v}
  if [ "$n" -ge 2 ] && [ "${v:0:1}" = '"' ] && [ "${v:$((n-1)):1}" = '"' ]; then
    v=${v:1:$((n-2))}
    v=${v//\\\"/\"}
    v=${v//\\\\/\\}
  elif [ "$n" -ge 2 ] && [ "${v:0:1}" = "'" ] && [ "${v:$((n-1)):1}" = "'" ]; then
    v=${v:1:$((n-2))}
  fi
  printf '%s' "$v"
}

env_line() { printf '%s=%s\n' "$1" "$(env_quote "$2")"; }

detect_host_ip() { # best-effort primary non-loopback IPv4, else 127.0.0.1
  local ip=""
  if command -v hostname >/dev/null 2>&1; then ip="$(hostname -I 2>/dev/null | awk '{print $1}')"; fi
  if [ -z "$ip" ] && command -v ipconfig >/dev/null 2>&1; then ip="$(ipconfig getifaddr en0 2>/dev/null)"; fi
  [ -z "$ip" ] && ip="127.0.0.1"
  printf '%s' "$ip"
}

# wait_http <url> <attempts> <sleep-seconds> -> 0 if it ever responds 2xx/3xx
wait_http() {
  local url="$1" attempts="${2:-40}" delay="${3:-0.5}" i=0
  while [ "$i" -lt "$attempts" ]; do
    curl -fsS -o /dev/null "$url" >/dev/null 2>&1 && return 0
    i=$((i + 1)); sleep "$delay"
  done
  return 1
}

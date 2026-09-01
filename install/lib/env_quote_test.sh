#!/usr/bin/env bash
# Tests for env_quote / env_unquote. Run from repo root:
#   bash install/lib/env_quote_test.sh
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=common.sh
. "$HERE/common.sh"

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

eq() {
  local got="$1" want="$2" name="$3"
  [ "$got" = "$want" ] || fail "$name: got $(printf %q "$got") want $(printf %q "$want")"
}

eq "$(env_quote "3107")" "3107" "plain port stays unquoted"
eq "$(env_quote "https://admin.workvar.com")" "https://admin.workvar.com" "URL stays unquoted"
eq "$(env_quote ":8787")" ":8787" "listen addr stays unquoted"
eq "$(env_quote "postgres://u:p@127.0.0.1:5432/db?sslmode=disable")" \
   "postgres://u:p@127.0.0.1:5432/db?sslmode=disable" "DSN stays unquoted"
eq "$(env_quote "localhost,127.0.0.1")" "localhost,127.0.0.1" "CSV hosts stay unquoted"
eq "$(env_quote "")" "" "empty stays empty"

quoted="$(env_quote "p#ass word")"
eq "$quoted" '"p#ass word"' "space and hash are quoted"

quoted="$(env_quote 'say "hi"')"
eq "$quoted" '"say \"hi\""' "embedded quotes are escaped"

eq "$(env_unquote '"3107"')" "3107" "unquote strips wrapper"
eq "$(env_unquote '3107')" "3107" "unquote leaves plain values"
eq "$(env_unquote '"https://admin.workvar.com"')" "https://admin.workvar.com" "unquote URL"

eq "$(env_line PORT 3107)" "PORT=3107" "env_line omits quotes for a port"
eq "$(env_line PUBLIC_BASE_URL "https://admin.workvar.com")" \
   "PUBLIC_BASE_URL=https://admin.workvar.com" "env_line omits quotes for a URL"

printf 'ok\n'

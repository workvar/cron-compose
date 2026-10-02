#!/usr/bin/env bash
# Tests for install/lib/edge.sh. Run from the repo root:
#   bash install/lib/edge_test.sh
# curl, lsof and nc are stubbed, so nothing touches the network or real ports.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="${EDGE_SCRIPT:-$HERE/edge.sh}"
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/bin"
cat >"$WORK/bin/curl" <<'SH'
#!/usr/bin/env bash
echo "$*" >>"$WORK/curl.log"
case "${FAKE_CURL:-plain}" in
  cloudflare) printf 'HTTP/2 200\r\nServer: cloudflare\r\nCF-RAY: abc-BOM\r\n\r\n' ;;
  plain)      printf 'HTTP/1.1 200 OK\r\nServer: nginx\r\n\r\n' ;;
  fail)       exit 7 ;;
  # gRPC hostname is a Tunnel TCP route (curl fails); the public URL is Cloudflare.
  grpc-fail)
    case "$*" in
      *grpc.example.com*) exit 7 ;;
      *) printf 'HTTP/2 200\r\nServer: cloudflare\r\nCF-RAY: abc-BOM\r\n\r\n' ;;
    esac
    ;;
  # gRPC hostname is a normal server; the public URL being Cloudflare is not enough.
  grpc-plain)
    case "$*" in
      *cron.example.com*) printf 'HTTP/2 200\r\nServer: cloudflare\r\nCF-RAY: abc-BOM\r\n\r\n' ;;
      *) printf 'HTTP/1.1 200 OK\r\nServer: nginx\r\n\r\n' ;;
    esac
    ;;
esac
SH
cat >"$WORK/bin/lsof" <<'SH'
#!/usr/bin/env bash
for p in ${FAKE_BUSY:-}; do case "$*" in *":$p "*|*":$p") exit 0 ;; esac; done
exit 1
SH
chmod +x "$WORK/bin/"*
export WORK

run() { ( export PATH="$WORK/bin:$PATH"; . "$SCRIPT"; "$@" ); }

# 1. Local hosts are never probed.
for h in localhost 127.0.0.1 10.0.0.4 ::1 pi.local; do
  run edge_host_is_local "$h" || fail "$h should be local"
done
run edge_host_is_local grpc.example.com && fail "a DNS name is not local"
: >"$WORK/curl.log"
[ "$(FAKE_CURL=cloudflare run edge_choose_mode 10.0.0.4)" = mtls ] || fail "IP must be mtls"
[ ! -s "$WORK/curl.log" ] || fail "a local host must not be probed"

# 2. Detection from the response headers, with an override.
[ "$(FAKE_CURL=cloudflare run edge_choose_mode grpc.example.com)" = edge ] || fail "cloudflare must be edge"
[ "$(FAKE_CURL=plain run edge_choose_mode grpc.example.com)" = mtls ] || fail "plain must be mtls"
[ "$(FAKE_CURL=fail run edge_choose_mode grpc.example.com)" = mtls ] || fail "unreachable must be mtls"
[ "$(CC_GRPC_MODE=edge FAKE_CURL=plain run edge_choose_mode grpc.example.com)" = edge ] || fail "override to edge"
[ "$(CC_GRPC_MODE=mtls FAKE_CURL=cloudflare run edge_choose_mode grpc.example.com)" = mtls ] || fail "override to mtls"

# 3. Port picking skips busy and reserved ports.
[ "$(run edge_pick_port 9078)" = 9078 ] || fail "free port taken as is"
[ "$(FAKE_BUSY=9078 run edge_pick_port 9078)" = 9079 ] || fail "busy port skipped"
[ "$(FAKE_BUSY=9078 run edge_pick_port 9078 "9079")" = 9080 ] || fail "reserved port skipped"

# 4. env_set replaces or appends, keeps the rest and the mode, and refuses odd values.
ENVF="$WORK/.env"
printf 'A=1\nPUBLIC_GRPC_ADDR=old:9077\nSECRET="a b#c"\n' >"$ENVF"; chmod 600 "$ENVF"
run edge_env_set "$ENVF" PUBLIC_GRPC_ADDR new.example.com
run edge_env_set "$ENVF" NEWKEY 127.0.0.1:9078
grep -qx 'PUBLIC_GRPC_ADDR=new.example.com' "$ENVF" || fail "not replaced"
grep -qx 'NEWKEY=127.0.0.1:9078' "$ENVF" || fail "not appended"
grep -qx 'SECRET="a b#c"' "$ENVF" || fail "other lines must be untouched"
[ "$(grep -c '^PUBLIC_GRPC_ADDR=' "$ENVF")" = 1 ] || fail "duplicate key"
[ "$(stat -c %a "$ENVF" 2>/dev/null || stat -f %Lp "$ENVF")" = 600 ] || fail "file mode changed"
run edge_env_set "$ENVF" BAD 'x; rm -rf /' 2>/dev/null && fail "odd value accepted"
grep -q '^BAD=' "$ENVF" && fail "odd value written"

# 5. Migration: a proxied host with no mode set is switched once.
fresh() { printf 'GRPC_ADDR=:9077\nPUBLIC_GRPC_ADDR=grpc.example.com:9077\nSESSION_SECRET=s\n' >"$ENVF"; }
fresh
out="$(FAKE_CURL=cloudflare run edge_migrate_env "$ENVF")"
[ "$out" = "edge 9078" ] || fail "migrate output: $out"
grep -qx 'PUBLIC_GRPC_MODE=edge' "$ENVF" || fail "mode not set"
grep -qx 'EDGE_GRPC_ADDR=127.0.0.1:9078' "$ENVF" || fail "edge addr not set"
grep -qx 'PUBLIC_GRPC_ADDR=grpc.example.com' "$ENVF" || fail "port must be dropped from the public address"
cp "$ENVF" "$WORK/after"
out="$(FAKE_CURL=cloudflare run edge_migrate_env "$ENVF")"
[ -z "$out" ] && cmp -s "$ENVF" "$WORK/after" || fail "second run must change nothing"

# 6. A saved mtls is upgraded when the host is proxied. CC_GRPC_MODE=mtls keeps it,
# and a host that is not proxied is left alone.
fresh; echo 'PUBLIC_GRPC_MODE=mtls' >>"$ENVF"
out="$(FAKE_CURL=cloudflare run edge_migrate_env "$ENVF")"
[ "$out" = "edge 9078" ] || fail "stored mtls must upgrade: $out"
grep -qx 'PUBLIC_GRPC_MODE=edge' "$ENVF" || fail "stored mtls was not upgraded"
fresh; echo 'PUBLIC_GRPC_MODE=mtls' >>"$ENVF"; cp "$ENVF" "$WORK/before"
CC_GRPC_MODE=mtls FAKE_CURL=cloudflare run edge_migrate_env "$ENVF" >/dev/null
cmp -s "$ENVF" "$WORK/before" || fail "CC_GRPC_MODE=mtls must keep mutual TLS"
fresh; cp "$ENVF" "$WORK/before"
FAKE_CURL=plain run edge_migrate_env "$ENVF" >/dev/null
cmp -s "$ENVF" "$WORK/before" || fail "an unproxied host must be left alone"

# 8. A Tunnel TCP gRPC hostname does not answer HTTPS. The public URL does.
fresh; printf '\nPUBLIC_BASE_URL=https://cron.example.com\n' >>"$ENVF"
out="$(FAKE_CURL=grpc-fail run edge_migrate_env "$ENVF")"
[ "$out" = "edge 9078" ] || fail "tcp tunnel via public URL: $out"
grep -qx 'PUBLIC_GRPC_ADDR=grpc.example.com' "$ENVF" || fail "tcp tunnel must drop the port"
grep -qx 'PUBLIC_GRPC_MODE=edge' "$ENVF" || fail "tcp tunnel must set edge"
fresh; printf '\nPUBLIC_BASE_URL=https://cron.example.com\n' >>"$ENVF"; cp "$ENVF" "$WORK/before"
FAKE_CURL=grpc-plain run edge_migrate_env "$ENVF" >/dev/null
cmp -s "$ENVF" "$WORK/before" || fail "a plain gRPC host must stay mtls when only the UI is proxied"

# 7. An existing EDGE_GRPC_ADDR is kept.
fresh; echo 'EDGE_GRPC_ADDR=127.0.0.1:9555' >>"$ENVF"
out="$(FAKE_CURL=cloudflare run edge_migrate_env "$ENVF")"
[ "$out" = "edge 9555" ] || fail "existing edge addr: $out"
[ "$(grep -c '^EDGE_GRPC_ADDR=' "$ENVF")" = 1 ] || fail "edge addr duplicated"
echo ok

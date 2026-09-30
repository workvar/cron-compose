#!/usr/bin/env bash
# Tests for the systemd helpers in scripts/install-agent.sh (stop_existing_agent,
# start_agent_service, report_agent_user). Run from the repo root:
#   bash scripts/install-agent_test.sh
#
# The installer needs root, git and Go the moment it is sourced, so the helper
# functions are cut out of it and run against stubbed systemctl/pgrep/sleep and a
# fake /proc. Set INSTALL_AGENT_SCRIPT to point the test at another copy of the script.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="${INSTALL_AGENT_SCRIPT:-$HERE/install-agent.sh}"

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/bin" "$WORK/proc"
export CALLS="$WORK/calls.log"

extract() { sed -n "/^$1() {/,/^}/p" "$SCRIPT"; }
{
  echo 'AGENT_UNIT=croncompose-agent.service'
  echo 'BIN_PATH=/usr/local/bin/croncompose-agent'
  extract stop_existing_agent
  extract start_agent_service
  extract report_agent_user
} >"$WORK/helpers.sh"
for fn in stop_existing_agent start_agent_service report_agent_user; do
  grep -q "^$fn() {" "$WORK/helpers.sh" || fail "could not find $fn in $SCRIPT"
done

cat >"$WORK/bin/systemctl" <<'SH'
#!/usr/bin/env bash
echo "systemctl $*" >>"$CALLS"
case "$1" in
  is-active) [ "${FAKE_ACTIVE:-0}" = "1" ] ;;
  show) echo "${FAKE_MAINPID:-0}" ;;
esac
SH
cat >"$WORK/bin/pgrep" <<'SH'
#!/usr/bin/env bash
[ -n "${FAKE_PGREP:-}" ] || exit 1
for p in $FAKE_PGREP; do echo "$p"; done
SH
printf '#!/usr/bin/env bash\nexit 0\n' >"$WORK/bin/sleep"
chmod +x "$WORK/bin/"*

# run_fn <function> [args...]: run a helper in a clean subshell; stdout+stderr on stdout.
run_fn() {
  : >"$CALLS"
  ( export PATH="$WORK/bin:$PATH" PROC_ROOT="$WORK/proc"
    # shellcheck source=/dev/null
    . "$WORK/helpers.sh"
    "$@" ) 2>&1 || true
}

set_uid() { mkdir -p "$WORK/proc/$1"; printf 'Name:\tagent\nUid:\t%s\t%s\t%s\t%s\n' "$2" "$2" "$2" "$2" >"$WORK/proc/$1/status"; }

contains() { case "$1" in *"$2"*) return 0 ;; *) return 1 ;; esac; }

# 1. A reinstall must restart the unit. enable --now alone leaves an active unit running.
run_fn start_agent_service >/dev/null
calls="$(cat "$CALLS")"
contains "$calls" "systemctl daemon-reload" || fail "start: no daemon-reload"
contains "$calls" "systemctl enable croncompose-agent.service" || fail "start: no enable"
contains "$calls" "systemctl restart croncompose-agent.service" || fail "start: unit is never restarted, an old process would keep running"
contains "$calls" "--now" && fail "start: enable --now does not restart an active unit"
[ "$(grep -n 'daemon-reload' "$CALLS" | cut -d: -f1)" -lt "$(grep -n 'restart' "$CALLS" | cut -d: -f1)" ] || fail "start: restart ran before daemon-reload"

# 2. An active agent is stopped before re-enrolling; an inactive one is left alone.
FAKE_ACTIVE=1 run_fn stop_existing_agent >/dev/null
contains "$(cat "$CALLS")" "systemctl stop croncompose-agent.service" || fail "stop: active agent was not stopped"
FAKE_ACTIVE=0 run_fn stop_existing_agent >/dev/null
contains "$(cat "$CALLS")" "systemctl stop" && fail "stop: inactive agent must not be stopped"

# 3. Root install, agent really is root: no warning.
set_uid 123 0
out="$(FAKE_MAINPID=123 run_fn report_agent_user 1)"
contains "$out" "runs as uid 0" || fail "root ok: missing status line: $out"
contains "$out" "warning" && fail "root ok: unexpected warning: $out"

# 4. Root install, but the old non-root process is still the MainPID: must warn.
set_uid 124 1001
out="$(FAKE_MAINPID=124 run_fn report_agent_user 1)"
contains "$out" "asked for a root agent but pid 124 runs as uid 1001" || fail "root mismatch not reported: $out"

# 5. Non-root install that came up as root: must warn.
out="$(FAKE_MAINPID=123 run_fn report_agent_user 0)"
contains "$out" "asked for a non-root agent but pid 123 runs as root" || fail "non-root mismatch not reported: $out"

# 6. A copy started outside systemd (pm2/nohup) is called out.
out="$(FAKE_MAINPID=123 FAKE_PGREP="999 123" run_fn report_agent_user 1)"
contains "$out" "not managed by systemd is running (pid 999" || fail "stray process not reported: $out"
contains "$out" "pid 123 " && contains "$out" "(pid 123" && fail "MainPID must not be reported as stray: $out"

# 7. The unit has no process: warn, do not fail the install.
out="$(FAKE_MAINPID=0 run_fn report_agent_user 1)"
contains "$out" "no running process yet" || fail "missing-process warning absent: $out"

printf 'ok\n'

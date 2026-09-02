#!/usr/bin/env bash
# Service lifecycle: enroll + start a local agent and print the post-install summary.
# Process management lives in lib/pm2.sh (pm2 + ecosystem.config.js); this file only
# deals with what the installer does around it.

# Extract a JSON string value without depending on jq.
json_string() { # <json> <key>
  printf '%s' "$1" | tr -d '\n' | grep -o "\"$2\":\"[^\"]*\"" | head -1 | sed "s/^\"$2\":\"//; s/\"$//"
}

enroll_local_agent() {
  [ "${ENABLE_AGENT:-0}" = "1" ] || return 0
  step "Enrolling local agent"
  local api="http://127.0.0.1:$API_PORT/api/v1"
  local cookie="$RUNTIME_DIR/run/cc-cookies.txt"
  local hostname_label; hostname_label="local-$(hostname 2>/dev/null || echo host)"

  if [ -f "$RUNTIME_DIR/agent/identity.json" ]; then ok "agent already enrolled"; restart_stack; return 0; fi

  curl -fsS -c "$cookie" -X POST "$api/auth/login" -H 'content-type: application/json' \
    -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}" >/dev/null 2>&1 \
    || { warn "could not log in to mint an enrollment token; skipping agent. Enroll later from the UI."; return 0; }

  local resp token
  resp="$(curl -fsS -b "$cookie" -X POST "$api/servers" -H 'content-type: application/json' \
    -d "{\"name\":\"$hostname_label\",\"description\":\"local agent (installer)\"}" 2>/dev/null || true)"
  token="$(json_string "$resp" token)"
  [ -n "$token" ] || { warn "no enrollment token returned; skipping agent."; return 0; }

  DATA_DIR="$RUNTIME_DIR/agent" \
  CONTROL_PLANE_HTTP="$api" \
  CONTROL_PLANE_ADDR="127.0.0.1:$GRPC_PORT" \
  CONTROL_PLANE_SNI="$AGENT_SNI" \
    "$REPO_ROOT/agent/bin/agent" enroll --token="$token" >/dev/null 2>&1 \
    && ok "agent enrolled" || { warn "agent enrollment failed; the stack is up without a local agent."; return 0; }

  restart_stack   # the agent process joins the pm2 list only now that an identity exists
  # Best-effort online confirmation.
  local i=0
  while [ "$i" -lt 15 ]; do
    curl -fsS -b "$cookie" "$api/servers" 2>/dev/null | grep -q '"status":"online"' && { ok "agent is online"; return 0; }
    i=$((i + 1)); sleep 0.5
  done
  warn "agent enrolled but not yet reporting online (check: ./croncompose-ctl.sh logs agent)"
}

print_summary() {
  # The control plane's HTTP port fronts the UI (/app) and REST (/api); agents use
  # the gRPC port directly.
  local origin; origin="$(public_base_url "$API_PORT")"
  local ui_url="$origin/app"
  local rest_url="$origin/api/v1"
  local grpc_addr; grpc_addr="$(public_grpc_addr "$GRPC_PORT")"
  printf '\n%s%s CronCompose is installed and running %s\n' "$C_GREEN" "============" "$C_RESET" >&2
  info ""
  info "Web UI:        $ui_url"
  dim "(the bare host:port redirects to /app)"
  info "REST API:      $rest_url   (health: /healthz)"
  info "Agent gRPC:    $grpc_addr"
  info ""
  info "Sign in with:  $ADMIN_EMAIL"
  if [ "${ADMIN_PASSWORD_GENERATED:-0}" = "1" ]; then
    info "Password:      $ADMIN_PASSWORD   (generated; save it now)"
  else
    info "Password:      (the one you entered)"
  fi
  info ""
  info "The stack runs under pm2. Manage it with the generated control script:"
  info "  ./croncompose-ctl.sh status      # what's running"
  info "  ./croncompose-ctl.sh logs web    # tail a log"
  info "  ./croncompose-ctl.sh restart     # restart everything, re-reading .env"
  info "  ./croncompose-ctl.sh stop        # stop everything"
  info "  ./croncompose-ctl.sh boot        # keep it running across reboots (pm2 startup)"
  dim "  pm2 status / pm2 monit / pm2 logs croncompose-web work directly too."
  info ""
  info "Config + secrets live in .env (mode 600). Runtime state in: $RUNTIME_DIR"
  if [ "${ENABLE_AGENT:-0}" = "1" ]; then
    info "A local agent is enrolled and running on this machine."
    info "Agent sudoers: /etc/sudoers.d/croncompose-agent (for Ports + connectors; re-run install/lib/agent_sudoers.sh if needed)"
  fi
  dim "For production, set Advertise host to a real DNS name, front the API/UI with TLS,"
  dim "and replace the self-signed CA under $RUNTIME_DIR/tls with your own PKI."
}

run_services() {
  ensure_pm2
  write_ctl_script
  start_stack
  enroll_local_agent
  if [ "${ENABLE_AGENT:-0}" = "1" ]; then
    install_agent_sudoers "$(id -un)" || true
  fi
  print_summary
}

#!/usr/bin/env bash
# Build everything from source: the control-plane, the migration tool, the agent
# (Linux/macOS only), the cc CLI, and the Next.js web UI.

build_go() {
  step "Building Go binaries"
  # Use only the locally installed Go; never let `go build` download a different Go
  # toolchain onto the system (preflight already verified the local version is new enough).
  export GOTOOLCHAIN=local
  ( cd "$REPO_ROOT/control-plane" && go build -o bin/control-plane ./cmd/server ) || die "control-plane build failed"
  ok "control-plane"
  ( cd "$REPO_ROOT/control-plane" && go build -o bin/migrate ./cmd/migrate ) || die "migrate tool build failed"
  ok "migrate"
  ( cd "$REPO_ROOT/cli" && go build -o bin/cc ./cmd/cc ) || die "cli build failed"
  ok "cc CLI"
  if [ "${ENABLE_AGENT:-0}" = "1" ]; then
    ( cd "$REPO_ROOT/agent" && go build -o bin/agent ./cmd/agent ) || die "agent build failed"
    ok "agent"
  fi
}

build_web() {
  if [ "${ENABLE_WEB:-1}" != "1" ]; then dim "skipping web build (--no-web)"; return 0; fi
  step "Building the web UI (npm install + next build)"
  info "this can take a minute on first run..."
  ( cd "$REPO_ROOT/web" && npm install --no-audit --no-fund ) || die "npm install failed"
  ok "dependencies installed"
  # API_BASE is inlined at build time by next.config.ts, so it must be set here to
  # point at the REST port chosen during configure.
  ( cd "$REPO_ROOT/web" && API_BASE="$API_BASE" NEXT_TELEMETRY_DISABLED=1 npm run build ) || die "next build failed"
  # next.config.ts uses output:"standalone", so the runtime is .next/standalone/server.js.
  # Static assets and public/ must sit alongside it (mirrors web/Dockerfile).
  local std="$REPO_ROOT/web/.next/standalone"
  rm -rf "$std/.next/static" && cp -r "$REPO_ROOT/web/.next/static" "$std/.next/static"
  if [ -d "$REPO_ROOT/web/public" ]; then rm -rf "$std/public" && cp -r "$REPO_ROOT/web/public" "$std/public"; fi
  ok "web UI built (standalone runtime staged)"
}

run_build() {
  build_go
  build_web
}

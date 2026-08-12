#!/usr/bin/env bash
# Keeps checked-in generated code honest before the Go build runs.
#
# Two failure modes bit real installs, both silent until `go build` exploded with an
# unhelpful message:
#   1. go.sum missing an entry for a newly imported module (nobody ran `go mod tidy`).
#   2. proto/agent/v1/*.pb.go older than proto/agent.proto, so messages added to the
#      schema simply don't exist in Go.
# Both are repo-side mistakes, but every device that clones the repo pays for them, so
# the installer detects and repairs them instead of failing.

# --- go.sum ----------------------------------------------------------------

# `go mod download` resolves every requirement and writes any missing go.sum lines.
# It fails closed on a genuinely broken module graph, where tidy is the real fix.
ensure_go_sums() {
  step "Verifying Go module checksums"
  export GOTOOLCHAIN=local
  local m
  for m in proto control-plane agent cli; do
    [ -f "$REPO_ROOT/$m/go.mod" ] || continue
    if ( cd "$REPO_ROOT/$m" && go mod download all >/dev/null 2>&1 ); then
      continue
    fi
    info "$m: go.sum incomplete, running go mod tidy..."
    ( cd "$REPO_ROOT/$m" && go mod tidy >/dev/null 2>&1 ) \
      || die "$m: go mod tidy failed. Check network access to proxy.golang.org and re-run."
    ok "$m: checksums repaired"
  done
  ok "modules resolved"
}

# --- protobuf --------------------------------------------------------------

PROTO_SRC() { printf '%s/proto/agent.proto' "$REPO_ROOT"; }
PROTO_PB()  { printf '%s/proto/agent/v1/agent.pb.go' "$REPO_ROOT"; }

# Every `message Foo` in the schema must exist as `type Foo struct` in the generated
# file. Cheaper and far more precise than an mtime comparison, which git checkouts
# scramble anyway (clone order decides mtimes, not edit history).
proto_is_current() {
  local src pb missing
  src="$(PROTO_SRC)"; pb="$(PROTO_PB)"
  [ -f "$src" ] || return 0        # no schema, nothing to check
  [ -f "$pb" ]  || return 1
  missing="$(
    awk '/^message [A-Za-z0-9_]+ \{/ { print $2 }' "$src" |
      while read -r msg; do
        grep -q "^type $msg struct" "$pb" || printf '%s ' "$msg"
      done
  )"
  [ -z "$missing" ] || { PROTO_MISSING="$missing"; return 1; }
  return 0
}

# protoc itself. Prefer the package manager; a manual install is a documented fallback
# rather than something we curl into the user's PATH.
install_protoc() {
  local R; R="$(need_root)"
  case "$PKG_MGR" in
    apt-get) db_run $R apt-get update -qq && db_run $R apt-get install -y protobuf-compiler ;;
    dnf|yum) db_run $R "$PKG_MGR" install -y protobuf-compiler ;;
    zypper)  db_run $R zypper --non-interactive install protobuf-devel ;;
    pacman)  db_run $R pacman -S --noconfirm protobuf ;;
    apk)     db_run $R apk add --no-cache protobuf ;;
    brew)    db_run brew install protobuf ;;
    *)       return 1 ;;
  esac
}

# The two codegen plugins, pinned to the versions proto/go.mod already depends on so the
# emitted code matches the runtime library. Installed into GOPATH/bin for this user.
install_proto_plugins() {
  local gobin pbver grpcver
  gobin="$(go env GOPATH)/bin"
  pbver="$(awk '/google.golang.org\/protobuf v/ { print $2; exit }' "$REPO_ROOT/proto/go.mod")"
  grpcver="$(awk '/google.golang.org\/grpc v/ { print $2; exit }' "$REPO_ROOT/proto/go.mod")"
  case ":$PATH:" in *":$gobin:"*) ;; *) export PATH="$PATH:$gobin" ;; esac

  command -v protoc-gen-go >/dev/null 2>&1 || {
    info "installing protoc-gen-go ${pbver:-latest}..."
    GOTOOLCHAIN=local go install "google.golang.org/protobuf/cmd/protoc-gen-go@${pbver:-latest}" \
      || return 1
  }
  command -v protoc-gen-go-grpc >/dev/null 2>&1 || {
    info "installing protoc-gen-go-grpc..."
    # The grpc cmd module versions separately from grpc itself; @latest is correct here.
    GOTOOLCHAIN=local go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest \
      || return 1
  }
  [ -n "$grpcver" ] || true
  return 0
}

regenerate_proto() {
  ( cd "$REPO_ROOT/proto" && protoc \
      --go_out=. --go_opt=module=github.com/croncompose/croncompose/proto \
      --go-grpc_out=. --go-grpc_opt=module=github.com/croncompose/croncompose/proto \
      agent.proto )
}

# Public entry point. Silent when the checked-in code is already correct, which is the
# normal case for a clean release.
ensure_proto() {
  PROTO_MISSING=""
  proto_is_current && return 0

  step "Regenerating protobuf code"
  warn "generated code is stale; missing: ${PROTO_MISSING:-agent.pb.go}"

  if ! command -v protoc >/dev/null 2>&1; then
    info "protoc not found, installing..."
    install_protoc >/dev/null 2>&1 || true
  fi
  command -v protoc >/dev/null 2>&1 || die "protoc is required to regenerate proto/agent/v1.
  Install it (${PKG_MGR:-your package manager}, or a release from
  https://github.com/protocolbuffers/protobuf/releases) and re-run the installer."

  install_proto_plugins || die "could not install protoc-gen-go / protoc-gen-go-grpc"
  regenerate_proto || die "protoc failed to regenerate proto/agent/v1"

  ( cd "$REPO_ROOT/proto" && GOTOOLCHAIN=local go mod tidy >/dev/null 2>&1 ) || true

  proto_is_current || die "regeneration ran but ${PROTO_MISSING:-messages} are still missing.
  Check that proto/agent.proto is valid: cd proto && protoc --go_out=. agent.proto"
  ok "proto/agent/v1 regenerated"
  dim "commit the regenerated files so other machines skip this step"
}

#!/usr/bin/env bash
# Drop compiler inputs that are not needed at runtime so a source install stays small.
# Safe to re-run. Next install/update restores tracked files with restore_source_tree.
#
# Usage:
#   restore_source_tree "$REPO_ROOT"
#   cleanup_build_tree "$REPO_ROOT"
# Relies on: step, ok, die from install/lib/common.sh or update.sh.

restore_source_tree() {
  local root="${1:?restore_source_tree needs the repo root}"
  [ -d "$root/.git" ] || die "$root is not a git checkout; cannot restore source after cleanup"
  command -v git >/dev/null 2>&1 || die "git not found"
  step "Restoring source from git (needed after a previous cleanup)"
  git -C "$root" checkout --force HEAD -- . \
    || die "git checkout failed; cannot rebuild from a stripped tree"
  if [ ! -f "$root/control-plane/go.mod" ]; then
    die "control-plane/go.mod is still missing after git checkout"
  fi
  ok "tracked files restored"
}

cleanup_build_tree() {
  local root="${1:?cleanup_build_tree needs the repo root}"
  [ -d "$root" ] || return 0
  step "Removing build inputs that are no longer needed"

  rm -rf "$root/web/node_modules" \
         "$root/web/.next/cache" \
         "$root/web/.next/types" \
         "$root/web/.next/server" \
         "$root/web/.next/static" \
         "$root/web/.next/diagnostics" \
         "$root/web/.next/trace" \
         "$root/web/.next/lock" 2>/dev/null || true

  # Keep web/.next/standalone (the running UI) and drop the rest of the web source.
  local keep
  for keep in app components lib public next.config.ts next.config.js tsconfig.json package.json package-lock.json; do
    rm -rf "$root/web/$keep" 2>/dev/null || true
  done

  _cleanup_module_src "$root/control-plane"
  _cleanup_module_src "$root/agent"
  _cleanup_module_src "$root/cli"
  rm -rf "$root/proto" "$root/docs" "$root/packaging" "$root/assets" \
         "$root/.github" "$root/migrations" 2>/dev/null || true

  ok "source tree stripped; .git, .env, binaries, and the web standalone runtime remain"
}

_cleanup_module_src() {
  local dir="$1"
  [ -d "$dir" ] || return 0
  local name
  for name in "$dir"/* "$dir"/.[!.]*; do
    [ -e "$name" ] || continue
    case "$(basename "$name")" in
      bin) continue ;;
      *) rm -rf "$name" ;;
    esac
  done
}

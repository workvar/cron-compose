package osuser

// ToolchainPrelude is sourced before every deploy/tools shell command we run as a
// target user. Non-interactive `bash -lc` skips most of ~/.bashrc (the usual
// `case $- in *i*) ;; *) return;; esac` guard), so nvm/fnm/asdf never load and
// `command -v npm` fails even though an interactive SSH session as that user
// finds them. This prelude loads the common user-local Node/Go shims explicitly.
const ToolchainPrelude = `
# CronCompose: load user toolchains for non-interactive shells.
[ -f "$HOME/.profile" ] && . "$HOME/.profile" >/dev/null 2>&1 || true
export NVM_DIR="${NVM_DIR:-$HOME/.nvm}"
[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh" >/dev/null 2>&1 || true
# Fallback when nvm.sh did not put node on PATH (common under non-interactive bash).
if ! command -v node >/dev/null 2>&1 && [ -d "$NVM_DIR/versions/node" ]; then
  _cc_latest=$(ls -1d "$NVM_DIR/versions/node"/v* 2>/dev/null | sort -V | tail -n1)
  if [ -n "$_cc_latest" ] && [ -d "$_cc_latest/bin" ]; then
    PATH="$_cc_latest/bin:$PATH"
  fi
  unset _cc_latest
fi
command -v fnm >/dev/null 2>&1 && eval "$(fnm env)" >/dev/null 2>&1 || true
[ -s "$HOME/.asdf/asdf.sh" ] && . "$HOME/.asdf/asdf.sh" >/dev/null 2>&1 || true
[ -d "$HOME/.local/bin" ] && case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) PATH="$HOME/.local/bin:$PATH" ;; esac
[ -d "$HOME/.bun/bin" ] && case ":$PATH:" in *":$HOME/.bun/bin:"*) ;; *) PATH="$HOME/.bun/bin:$PATH" ;; esac
[ -d "$HOME/.local/go/bin" ] && case ":$PATH:" in *":$HOME/.local/go/bin:"*) ;; *) PATH="$HOME/.local/go/bin:$PATH" ;; esac
export PATH
`

// WrapScript prefixes a user command with ToolchainPrelude so nvm-installed npm
// (and similar) are visible to preflight and install scripts.
func WrapScript(script string) string {
	return ToolchainPrelude + "\n" + script
}

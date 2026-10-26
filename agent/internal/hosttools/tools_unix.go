//go:build !windows

package hosttools

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/croncompose/croncompose/agent/internal/osuser"
)

// scriptCommand runs an install/uninstall script under the account's login bash
// with the toolchain prelude, so nvm and friends are on PATH.
func scriptCommand(ctx context.Context, script string) (*exec.Cmd, func(), error) {
	return exec.CommandContext(ctx, "/bin/bash", "-lc", osuser.WrapScript(script)), func() {}, nil
}

func probe(ctx context.Context, cred *osuser.Credential, bin string) (path, version string, ok bool) {
	inner := fmt.Sprintf(`bin=$(command -v %s) || exit 1; echo "PATH:$bin"; ("%s" --version || "%s" -V || "%s" version) 2>/dev/null | head -n1`,
		shellQuote(bin), shellQuote(bin), shellQuote(bin), shellQuote(bin))
	cmd := exec.CommandContext(ctx, "/bin/bash", "-lc", osuser.WrapScript(inner))
	cmd.Env = buildEnv(cred)
	applyCred(cmd, cred)
	out, err := cmd.Output()
	if err != nil {
		return "", "", false
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "PATH:") {
			path = strings.TrimPrefix(line, "PATH:")
			ok = path != ""
			continue
		}
		if version == "" {
			version = strings.TrimSpace(line)
		}
	}
	return path, version, ok
}

func installScript(tool string, cred *osuser.Credential) (string, error) {
	home := "$HOME"
	if cred != nil && cred.Home != "" {
		home = shellQuote(cred.Home)
	}
	tmp := filepath.Join("$HOME", "tmp")
	if cred != nil && cred.Home != "" {
		tmp = shellQuote(filepath.Join(cred.Home, "tmp"))
	}
	switch tool {
	case "node":
		// Prefer nvm into the account home so login shells pick it up. Falls back
		// to NodeSource apt only when the caller is root and nvm is unavailable.
		return fmt.Sprintf(`
set -euo pipefail
mkdir -p %s %s/.nvm
export NVM_DIR=%s/.nvm
if [ ! -s "$NVM_DIR/nvm.sh" ]; then
  curl -fsSL https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.2/install.sh | bash
fi
. "$NVM_DIR/nvm.sh"
nvm install --lts
nvm alias default 'lts/*'
node -v
npm -v
`, tmp, home, home), nil
	case "go":
		return fmt.Sprintf(`
set -euo pipefail
mkdir -p %s %s/.local
ver=$(curl -fsSL https://go.dev/VERSION?m=text | head -n1)
arch=$(uname -m)
case "$arch" in
  aarch64|arm64) goarch=arm64 ;;
  x86_64|amd64) goarch=amd64 ;;
  *) echo "unsupported arch: $arch"; exit 1 ;;
esac
os=$(uname -s | tr '[:upper:]' '[:lower:]')
tarball="$ver.$os-$goarch.tar.gz"
curl -fsSL "https://go.dev/dl/$tarball" -o %s/"$tarball"
rm -rf %s/.local/go
tar -C %s/.local -xzf %s/"$tarball"
mkdir -p %s/.local/bin
ln -sfn %s/.local/go/bin/go %s/.local/bin/go
ln -sfn %s/.local/go/bin/gofmt %s/.local/bin/gofmt
# Ensure future login shells see ~/.local/bin
profile=%s/.profile
grep -q '\.local/bin' "$profile" 2>/dev/null || echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$profile"
export PATH="%s/.local/bin:$PATH"
go version
`, tmp, home, tmp, home, home, tmp, home, home, home, home, home, home, home), nil
	case "python":
		return `
set -euo pipefail
if command -v apt-get >/dev/null 2>&1; then
  if [ "$(id -u)" -eq 0 ]; then
    apt-get update -y
    DEBIAN_FRONTEND=noninteractive apt-get install -y python3 python3-pip python3-venv
  else
    echo "python install via apt needs root; ask an admin or use the system package manager"
    exit 1
  fi
elif command -v brew >/dev/null 2>&1; then
  brew install python
else
  echo "no supported package manager for python"
  exit 1
fi
python3 --version
`, nil
	case "pm2":
		return `
set -euo pipefail
if ! command -v npm >/dev/null 2>&1; then
  echo "npm is required to install pm2; install node first"
  exit 1
fi
npm install -g pm2
pm2 -v
`, nil
	case "git":
		return `
set -euo pipefail
if command -v apt-get >/dev/null 2>&1; then
  if [ "$(id -u)" -eq 0 ]; then
    apt-get update -y
    DEBIAN_FRONTEND=noninteractive apt-get install -y git
  else
    echo "git install via apt needs root"
    exit 1
  fi
elif command -v brew >/dev/null 2>&1; then
  brew install git
else
  echo "no supported package manager for git"
  exit 1
fi
git --version
`, nil
	case "yarn":
		return `
set -euo pipefail
command -v npm >/dev/null 2>&1 || { echo "npm required"; exit 1; }
npm install -g yarn
yarn -v
`, nil
	case "pnpm":
		return `
set -euo pipefail
command -v npm >/dev/null 2>&1 || { echo "npm required"; exit 1; }
npm install -g pnpm
pnpm -v
`, nil
	case "bun":
		return `
set -euo pipefail
curl -fsSL https://bun.sh/install | bash
export BUN_INSTALL="$HOME/.bun"
export PATH="$BUN_INSTALL/bin:$PATH"
bun -v
`, nil
	default:
		return "", fmt.Errorf("no installer for %s", tool)
	}
}

func uninstallScript(tool string, cred *osuser.Credential) (string, error) {
	home := "$HOME"
	if cred != nil && cred.Home != "" {
		home = shellQuote(cred.Home)
	}
	switch tool {
	case "node":
		return fmt.Sprintf(`
set -euo pipefail
export NVM_DIR=%s/.nvm
if [ -s "$NVM_DIR/nvm.sh" ]; then
  . "$NVM_DIR/nvm.sh"
  nvm deactivate 2>/dev/null || true
  nvm uninstall --lts 2>/dev/null || true
  # Drop every installed node version we can see
  for v in $(ls "$NVM_DIR/versions/node" 2>/dev/null || true); do
    nvm uninstall "$v" 2>/dev/null || true
  done
fi
rm -rf %s/.nvm
echo "node/nvm removed from account home"
`, home, home), nil
	case "go":
		return fmt.Sprintf(`
set -euo pipefail
rm -f %s/.local/bin/go %s/.local/bin/gofmt
rm -rf %s/.local/go
echo "go removed from ~/.local"
`, home, home, home), nil
	case "bun":
		return fmt.Sprintf(`
set -euo pipefail
rm -rf %s/.bun
echo "bun removed from ~/.bun"
`, home), nil
	case "pm2", "yarn", "pnpm":
		return fmt.Sprintf(`
set -euo pipefail
if ! command -v npm >/dev/null 2>&1; then
  echo "npm not found; nothing to uninstall for %s"
  exit 0
fi
npm uninstall -g %s || true
echo "%s uninstalled via npm -g"
`, tool, tool, tool), nil
	case "python":
		return `
set -euo pipefail
if command -v apt-get >/dev/null 2>&1; then
  if [ "$(id -u)" -eq 0 ]; then
    DEBIAN_FRONTEND=noninteractive apt-get remove -y python3 python3-pip python3-venv || true
    echo "attempted apt remove for python3"
    exit 0
  fi
  echo "python was likely installed system-wide; uninstall needs root"
  exit 1
elif command -v brew >/dev/null 2>&1; then
  brew uninstall python || true
  exit 0
fi
echo "no supported uninstall path for python"
exit 1
`, nil
	case "git":
		return `
set -euo pipefail
if command -v apt-get >/dev/null 2>&1; then
  if [ "$(id -u)" -eq 0 ]; then
    DEBIAN_FRONTEND=noninteractive apt-get remove -y git || true
    echo "attempted apt remove for git"
    exit 0
  fi
  echo "git was likely installed system-wide; uninstall needs root"
  exit 1
elif command -v brew >/dev/null 2>&1; then
  brew uninstall git || true
  exit 0
fi
echo "no supported uninstall path for git"
exit 1
`, nil
	default:
		return "", fmt.Errorf("no uninstaller for %s", tool)
	}
}

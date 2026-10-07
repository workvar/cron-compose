// Package hosttools detects and installs common deploy toolchains for a chosen
// OS account. Detection and installs run under that user's login shell so nvm,
// asdf, and similar PATH hooks are visible — the same problem deploy preflight
// hits when the agent process PATH is a minimal systemd default.
package hosttools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/croncompose/croncompose/agent/internal/osuser"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

const (
	detectTimeout  = 30 * time.Second
	installTimeout = 12 * time.Minute
	logCap         = 256 << 10 // 256KB
)

// KnownTools is the catalog the Tools UI shows. Order is display order.
var KnownTools = []string{
	"node", "npm", "pnpm", "yarn", "bun",
	"go", "python", "python3", "pip", "pip3",
	"pm2", "git", "docker", "make",
}

// Installable are tools the agent knows how to install for a given account.
var Installable = map[string]bool{
	"node": true, "go": true, "python": true, "pm2": true,
	"git": true, "yarn": true, "pnpm": true, "bun": true,
}

// Detect reports which known tools are on the chosen user's login PATH.
func Detect(ctx context.Context, runAs string) ([]*agentv1.ToolStatus, error) {
	cred, err := osuser.Resolve(runAs)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, detectTimeout)
	defer cancel()

	out := make([]*agentv1.ToolStatus, 0, len(KnownTools))
	for _, name := range KnownTools {
		st := &agentv1.ToolStatus{Name: name}
		path, ver, ok := probe(ctx, cred, name)
		st.Installed = ok
		st.Path = path
		st.Version = ver
		out = append(out, st)
	}
	return out, nil
}

// Install runs a curated installer for tool as runAs and returns a fresh detect
// snapshot plus captured log output.
func Install(ctx context.Context, runAs, tool string) (tools []*agentv1.ToolStatus, log string, exitCode int, err error) {
	tool = strings.ToLower(strings.TrimSpace(tool))
	if !Installable[tool] {
		return nil, "", 1, fmt.Errorf("unknown or unsupported tool %q", tool)
	}
	cred, err := osuser.Resolve(runAs)
	if err != nil {
		return nil, "", 1, err
	}
	script, err := installScript(tool, cred)
	if err != nil {
		return nil, "", 1, err
	}

	ctx, cancel := context.WithTimeout(ctx, installTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/bash", "-lc", osuser.WrapScript(script))
	cmd.Env = buildEnv(cred)
	applyCred(cmd, cred)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	runErr := cmd.Run()
	log = trimLog(buf.String())
	exitCode = 0
	if runErr != nil {
		exitCode = 1
		if ee, ok := runErr.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		}
		if ctx.Err() != nil {
			return nil, log, exitCode, fmt.Errorf("install timed out")
		}
	}

	tools, detErr := Detect(context.Background(), runAs)
	if detErr != nil && runErr == nil {
		return tools, log, exitCode, detErr
	}
	return tools, log, exitCode, runErr
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

func buildEnv(cred *osuser.Credential) []string {
	env := append([]string(nil), os.Environ()...)
	if cred != nil {
		env = append(env, cred.Env()...)
		if cred.Home != "" {
			tmp := filepath.Join(cred.Home, "tmp")
			_ = os.MkdirAll(tmp, 0o755)
			env = append(env, "TMPDIR="+tmp, "TMP="+tmp, "TEMP="+tmp)
		}
	}
	return env
}

func applyCred(cmd *exec.Cmd, cred *osuser.Credential) {
	if cred == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Credential = &syscall.Credential{
		Uid: cred.UID, Gid: cred.GID, Groups: cred.Groups,
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func trimLog(s string) string {
	if len(s) <= logCap {
		return s
	}
	return s[len(s)-logCap:]
}

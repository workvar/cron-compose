//go:build windows

package hosttools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/croncompose/croncompose/agent/internal/osuser"
	"github.com/croncompose/croncompose/agent/internal/platform"
)

// scriptCommand runs a PowerShell install/uninstall script from a temp file. The
// Windows installers are PowerShell because they drive winget and Chocolatey,
// which are the Windows package managers, rather than apt or Homebrew.
func scriptCommand(ctx context.Context, script string) (*exec.Cmd, func(), error) {
	f, err := os.CreateTemp("", "croncompose-tool-*.ps1")
	if err != nil {
		return nil, nil, err
	}
	name := f.Name()
	cleanup := func() { _ = os.Remove(name) }
	if _, err := f.WriteString(psPrelude + script); err != nil {
		f.Close()
		cleanup()
		return nil, nil, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return nil, nil, err
	}
	cmd := exec.CommandContext(ctx, "powershell.exe",
		"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", name)
	return cmd, cleanup, nil
}

// probe finds bin with the agent's extended lookup and asks it for a version.
func probe(ctx context.Context, _ *osuser.Credential, bin string) (path, version string, ok bool) {
	path, err := platform.LookPath(bin)
	if err != nil {
		return "", "", false
	}
	for _, flag := range []string{"--version", "version", "-V"} {
		out, err := exec.CommandContext(ctx, path, flag).Output()
		if err != nil {
			continue
		}
		if line := firstLine(string(out)); line != "" {
			return path, line, true
		}
	}
	return path, "", true
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// packages maps a tool to its winget id and Chocolatey id.
var packages = map[string][2]string{
	"node":   {"OpenJS.NodeJS.LTS", "nodejs-lts"},
	"go":     {"GoLang.Go", "golang"},
	"python": {"Python.Python.3.12", "python"},
	"git":    {"Git.Git", "git"},
}

func installScript(tool string, _ *osuser.Credential) (string, error) {
	switch tool {
	case "node", "go", "python", "git":
		ids := packages[tool]
		check := map[string]string{
			"node": "node -v; npm -v", "go": "go version", "python": "python --version", "git": "git --version",
		}[tool]
		return fmt.Sprintf("Install-Pkg '%s' '%s'\nRefresh-Path\n%s\n", ids[0], ids[1], check), nil
	case "pm2", "yarn", "pnpm":
		return fmt.Sprintf("Require-Npm\nnpm install -g %s\nif ($LASTEXITCODE -ne 0) { throw 'npm install -g %s failed' }\n%s -v\n",
			tool, tool, tool), nil
	case "bun":
		return "irm https://bun.sh/install.ps1 | iex\n$env:Path = \"$HOME\\.bun\\bin;$env:Path\"\nbun -v\n", nil
	default:
		return "", fmt.Errorf("no installer for %s", tool)
	}
}

func uninstallScript(tool string, _ *osuser.Credential) (string, error) {
	switch tool {
	case "node", "go", "python", "git":
		ids := packages[tool]
		return fmt.Sprintf("Uninstall-Pkg '%s' '%s'\n", ids[0], ids[1]), nil
	case "pm2", "yarn", "pnpm":
		return fmt.Sprintf("Require-Npm\nnpm uninstall -g %s\nWrite-Output '%s uninstalled via npm -g'\n", tool, tool), nil
	case "bun":
		return "Remove-Item -Recurse -Force \"$HOME\\.bun\" -ErrorAction SilentlyContinue\nWrite-Output 'bun removed from ~\\.bun'\n", nil
	default:
		return "", fmt.Errorf("no uninstaller for %s", tool)
	}
}

// psPrelude is shared by every Windows tools script. Installs are machine-wide so
// the agent service and every operator see the same toolchain. winget is tried
// first; Chocolatey is the fallback (and is bootstrapped if missing) because
// winget is not available to the LocalSystem account on most Windows Server builds.
const psPrelude = `
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor 3072
function Have($n) { [bool](Get-Command $n -ErrorAction SilentlyContinue) }
function Refresh-Path {
  $env:Path = [Environment]::GetEnvironmentVariable('Path','Machine') + ';' +
              [Environment]::GetEnvironmentVariable('Path','User') + ';' +
              "$env:APPDATA\npm;$env:ProgramData\chocolatey\bin"
}
function Ensure-Choco {
  if (Have choco) { return }
  Write-Output 'Chocolatey not found; installing it (https://chocolatey.org)'
  Invoke-Expression ((New-Object Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))
  Refresh-Path
}
function Install-Pkg($wingetId, $chocoId) {
  if (Have winget) {
    Write-Output "winget install $wingetId"
    winget install --id $wingetId -e --silent --scope machine --accept-package-agreements --accept-source-agreements --disable-interactivity
    if ($LASTEXITCODE -eq 0) { return }
    Write-Output "winget failed ($LASTEXITCODE); trying Chocolatey"
  }
  Ensure-Choco
  Write-Output "choco install $chocoId"
  choco install $chocoId -y --no-progress
  if ($LASTEXITCODE -ne 0) { throw "choco install $chocoId failed" }
}
function Uninstall-Pkg($wingetId, $chocoId) {
  if (Have choco) {
    choco uninstall $chocoId -y --no-progress
    if ($LASTEXITCODE -eq 0) { return }
  }
  if (Have winget) {
    winget uninstall --id $wingetId -e --silent --disable-interactivity
    if ($LASTEXITCODE -eq 0) { return }
  }
  throw "no package manager could uninstall $chocoId"
}
function Require-Npm {
  Refresh-Path
  if (-not (Have npm)) { throw 'npm is required; install node first' }
}
Refresh-Path
`

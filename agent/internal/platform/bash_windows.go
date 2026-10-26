//go:build windows

package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var (
	bashOnce sync.Once
	bashPath string
)

// Bash returns Git for Windows' bash.exe. Deploy, tools, and connector scripts are
// bash scripts on every platform, and Git Bash is already present wherever deploys
// work (they need git). When nothing is found it returns "bash.exe" so the exec
// error names the missing program; callers add BashHint.
func Bash() string {
	bashOnce.Do(func() { bashPath = findBash() })
	return bashPath
}

// HasBash reports whether a real (non-WSL) bash was found.
func HasBash() bool { return Bash() != "bash.exe" }

func findBash() string {
	if v := strings.TrimSpace(os.Getenv("AGENT_BASH")); v != "" && fileExists(v) {
		return v
	}
	for _, c := range bashCandidates() {
		if fileExists(c) {
			return c
		}
	}
	if p, err := exec.LookPath("bash"); err == nil && !isWSLLauncher(p) {
		return p
	}
	return "bash.exe"
}

// bashCandidates derives bash from git's own location first (git\cmd\git.exe ->
// git\bin\bash.exe), then the standard install directories.
func bashCandidates() []string {
	var out []string
	if git, err := exec.LookPath("git"); err == nil {
		root := filepath.Dir(filepath.Dir(git))
		out = append(out,
			filepath.Join(root, "bin", "bash.exe"),
			filepath.Join(root, "usr", "bin", "bash.exe"))
	}
	for _, env := range []string{"ProgramFiles", "ProgramW6432", "ProgramFiles(x86)"} {
		if base := os.Getenv(env); base != "" {
			out = append(out, filepath.Join(base, "Git", "bin", "bash.exe"))
		}
	}
	if la := os.Getenv("LOCALAPPDATA"); la != "" {
		out = append(out, filepath.Join(la, "Programs", "Git", "bin", "bash.exe"))
	}
	return out
}

// isWSLLauncher spots C:\Windows\System32\bash.exe, which starts a Linux distro
// rather than a Windows shell; scripts would run in the wrong world entirely.
func isWSLLauncher(p string) bool {
	sys := strings.ToLower(filepath.Join(os.Getenv("SystemRoot"), "System32"))
	return strings.HasPrefix(strings.ToLower(p), sys) ||
		strings.Contains(strings.ToLower(p), `\windowsapps\`)
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

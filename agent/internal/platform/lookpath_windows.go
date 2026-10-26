//go:build windows

package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// LookPath finds a program the way an operator's shell would, not just the way the
// service's PATH would. A Windows service inherits the machine PATH but not the
// per-user one, so npm globals (pm2, yarn, pnpm), Go's GOPATH bin, and bun are
// often invisible to it even though they are installed. bash and sh resolve to Git
// Bash, never to the WSL launcher.
func LookPath(name string) (string, error) {
	if base := strings.ToLower(name); base == "bash" || base == "sh" {
		if HasBash() {
			return Bash(), nil
		}
	}
	p, err := exec.LookPath(name)
	if err == nil {
		return p, nil
	}
	for _, dir := range extraBinDirs() {
		for _, ext := range []string{".exe", ".cmd", ".bat"} {
			c := filepath.Join(dir, name+ext)
			if fileExists(c) {
				return c, nil
			}
		}
	}
	return "", err
}

// ExtraBinDirs are prepended to PATH for deploy and tools commands so programs found
// by LookPath can also find their siblings (pm2.cmd needs node.exe, for example).
func ExtraBinDirs() []string {
	var out []string
	for _, d := range extraBinDirs() {
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			out = append(out, d)
		}
	}
	return out
}

func extraBinDirs() []string {
	var dirs []string
	add := func(env string, parts ...string) {
		if base := os.Getenv(env); base != "" {
			dirs = append(dirs, filepath.Join(append([]string{base}, parts...)...))
		}
	}
	add("APPDATA", "npm")
	add("ProgramFiles", "nodejs")
	add("ProgramFiles", "Git", "cmd")
	add("ProgramFiles", "Go", "bin")
	add("ProgramFiles", "Docker", "Docker", "resources", "bin")
	add("USERPROFILE", "go", "bin")
	add("USERPROFILE", ".bun", "bin")
	add("LOCALAPPDATA", "pnpm")
	add("ProgramData", "chocolatey", "bin")
	return dirs
}

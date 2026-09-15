package deploy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// Preflight catches the failures that are cheap to predict and expensive to hit
// halfway through an install: no disk, no git, no package manager, a base directory
// the agent cannot write. Each check logs its own line, so a failed deploy says
// "pnpm: not found on this server" instead of dying forty seconds into a script.
const minFreeBytes = 512 << 20 // 512MB, enough for a node_modules tree plus git objects

// preflight returns a fatal error only for conditions that guarantee failure.
// Anything it is unsure about is logged as a warning and the deploy continues.
func (m *Manager) preflight(runID, token, base, installScript, processManager string) error {
	m.phaseLine(runID, token, phasePreflight, "checking "+base)

	if err := ensureWritableDir(base); err != nil {
		return fmt.Errorf("clone path not usable: %w", err)
	}
	m.phaseLine(runID, token, phasePreflight, "clone path writable: ok")

	free, err := freeBytes(base)
	switch {
	case err != nil:
		m.phaseLine(runID, token, phasePreflight, "disk space: could not determine ("+err.Error()+")")
	case free < minFreeBytes:
		return fmt.Errorf("only %s free on %s, need at least %s", humanBytes(free), base, humanBytes(minFreeBytes))
	default:
		m.phaseLine(runID, token, phasePreflight, "disk space: "+humanBytes(free)+" free")
	}

	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is not installed on this server")
	}
	m.phaseLine(runID, token, phasePreflight, "git: ok")

	for _, bin := range requiredBinaries(installScript) {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("%s is not installed on this server, but the install script needs it", bin)
		}
		m.phaseLine(runID, token, phasePreflight, bin+": ok")
	}

	if bin := processManagerBinary(processManager); bin != "" {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("process manager %q needs %s, which is not installed on this server", processManager, bin)
		}
		m.phaseLine(runID, token, phasePreflight, bin+": ok")
	}
	return nil
}

// ensureWritableDir creates the directory if missing and proves it is writable by
// the agent's own user, which is a better test than inspecting permission bits.
func ensureWritableDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	probe := filepath.Join(dir, ".croncompose-write-test")
	f, err := os.OpenFile(probe, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_ = f.Close()
	return os.Remove(probe)
}

func freeBytes(dir string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return st.Bavail * uint64(st.Bsize), nil
}

// knownBinaries maps a token that may appear at the head of an install command to
// the executable it needs. Only well-known build tools are listed: guessing wider
// than this would start failing deploys over shell builtins and project scripts.
var knownBinaries = map[string]string{
	"npm": "npm", "pnpm": "pnpm", "yarn": "yarn", "bun": "bun", "npx": "npx",
	"node": "node", "go": "go", "cargo": "cargo", "python3": "python3", "pip": "pip",
	"bundle": "bundle", "composer": "composer", "mix": "mix", "mvn": "mvn",
	"gradle": "gradle", "docker": "docker", "make": "make",
}

// requiredBinaries pulls the tools an install script starts its commands with. It
// splits on the shell operators that begin a new command, so "pnpm install && pnpm
// run build" yields pnpm once, and "PORT=3000 node server.js" is ignored rather than
// mistaken for a binary named PORT=3000.
func requiredBinaries(script string) []string {
	if strings.TrimSpace(script) == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, part := range splitCommands(script) {
		fields := strings.Fields(part)
		// Step over leading environment assignments: in "PORT=3000 node server.js" the
		// binary is node, not PORT=3000.
		for len(fields) > 0 && strings.Contains(fields[0], "=") {
			fields = fields[1:]
		}
		if len(fields) == 0 {
			continue
		}
		head := fields[0]
		if strings.HasPrefix(head, ".") || strings.HasPrefix(head, "/") {
			continue
		}
		bin, ok := knownBinaries[head]
		if !ok || seen[bin] {
			continue
		}
		seen[bin] = true
		out = append(out, bin)
	}
	return out
}

func splitCommands(script string) []string {
	r := strings.NewReplacer("&&", "\n", "||", "\n", ";", "\n", "|", "\n")
	return strings.Split(r.Replace(script), "\n")
}

func processManagerBinary(pm string) string {
	switch pm {
	case "pm2":
		return "pm2"
	case "systemd":
		return "systemctl"
	case "docker":
		return "docker"
	default:
		return ""
	}
}

func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := uint64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(n)/float64(div), "KMGTPE"[exp])
}

package connectors

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// serviceStatus maps `systemctl is-active <unit>` onto running|stopped|unknown.
func serviceStatus(ctx context.Context, unit string) string {
	if !systemdAvailable() {
		return "unknown"
	}
	out, _ := run(ctx, "systemctl", "is-active", unit)
	switch out {
	case "active":
		return "running"
	case "inactive", "failed", "deactivating":
		return "stopped"
	default:
		return "unknown"
	}
}

// existingPaths returns the subset of paths that exist on disk.
func existingPaths(paths ...string) []string {
	out := []string{}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// writable reports whether the agent can open a path for writing. It opens O_WRONLY
// without O_TRUNC and immediately closes, so it never modifies the file; it is only used
// to set the can_edit capability hint.
func writable(path string) bool {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

// confFiles returns config files for base: a file yields itself; a directory is read one
// level deep for *.conf and extensionless entries (enabled sites). Bounded to keep
// discovery cheap.
func confFiles(base string) []string {
	info, err := os.Stat(base)
	if err != nil {
		return nil
	}
	if !info.IsDir() {
		return []string{base}
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".conf") || !strings.Contains(name, ".") {
			out = append(out, filepath.Join(base, name))
		}
		if len(out) >= 50 {
			break
		}
	}
	return out
}

// parseVersionAfterSlash pulls "1.24.0" out of "nginx version: nginx/1.24.0".
func parseVersionAfterSlash(s string) string {
	line := firstLine(s)
	if i := strings.LastIndex(line, "/"); i >= 0 {
		return strings.TrimSpace(line[i+1:])
	}
	return strings.TrimSpace(line)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[i+1:])
	}
	return s
}

// secondField returns the 2nd whitespace field of the first line, e.g. "255" from
// "systemd 255 (255.4-1)".
func secondField(s string) string {
	f := strings.Fields(firstLine(s))
	if len(f) >= 2 {
		return f[1]
	}
	if len(f) == 1 {
		return f[0]
	}
	return ""
}

func countLines(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}

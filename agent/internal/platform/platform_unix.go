//go:build !windows

package platform

import (
	"os"

	"golang.org/x/sys/unix"
)

// Bash is the shell every deploy, tools, and connector script runs under.
func Bash() string { return "/bin/bash" }

// IsElevated reports whether the agent can act as root.
func IsElevated() bool { return os.Geteuid() == 0 }

// TempDir is the shared scratch directory used when a deploy has no per-user tmp.
// It stays the literal /tmp (not $TMPDIR) to match the control plane's path rules.
func TempDir() string { return "/tmp" }

// LocalPath maps a path sent by the control plane to this host. Unix paths are
// already local.
func LocalPath(p string) string { return p }

// FreeBytes reports the bytes available to an unprivileged writer under dir.
func FreeBytes(dir string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return st.Bavail * uint64(st.Bsize), nil
}

// ExtraEnv is environment the platform needs on every child process. None on Unix.
func ExtraEnv() []string { return nil }

// PassthroughEnv lists agent environment keys a sandboxed job still inherits.
func PassthroughEnv() []string { return []string{"HOME", "LANG", "TZ"} }

// CurrentUser is the agent's account name as the environment reports it.
func CurrentUser() string { return os.Getenv("USER") }

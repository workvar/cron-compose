package deploys

import (
	"path"
	"strings"
)

// ClonePathForUser rewrites a language clone path under the chosen account's home
// when deploying as a non-root user. Root (and empty run_as) keep the global
// /opt/apps/… layout. For "pi" with home "/home/pi", "/opt/apps/node/repo" becomes
// "/home/pi/opt/apps/node/repo" — that user's own opt tree, which they can write
// without sudo and where their login PATH (nvm) applies.
func ClonePathForUser(clonePath, runAs, home string) string {
	runAs = strings.TrimSpace(runAs)
	home = strings.TrimRight(strings.TrimSpace(home), "/")
	if runAs == "" || runAs == "root" || home == "" || home == "/" {
		return clonePath
	}
	clonePath = path.Clean(clonePath)
	if clonePath == "." || clonePath == "/" {
		return path.Join(home, "opt", "apps")
	}
	// Already under this home — leave it alone.
	if clonePath == home || strings.HasPrefix(clonePath, home+"/") {
		return clonePath
	}
	if strings.HasPrefix(clonePath, "/opt/") {
		return path.Join(home, strings.TrimPrefix(clonePath, "/"))
	}
	// Fallback: park anything else under ~/opt/apps/<basename>.
	return path.Join(home, "opt", "apps", path.Base(clonePath))
}

// UserTmpDir is the per-account temp directory used for deploy scratch (incoming
// checkouts, write probes). Root keeps the system /tmp; others get ~/tmp.
func UserTmpDir(runAs, home string) string {
	runAs = strings.TrimSpace(runAs)
	home = strings.TrimRight(strings.TrimSpace(home), "/")
	if runAs == "" || runAs == "root" || home == "" {
		return "/tmp"
	}
	return path.Join(home, "tmp")
}

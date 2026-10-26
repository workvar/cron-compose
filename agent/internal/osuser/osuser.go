// Package osuser resolves a username to the credentials a child process needs to run
// as that user, and reports honestly when the agent is not allowed to do that.
//
// Dropping privileges is only possible downward: a root agent can become anyone, an
// unprivileged agent can only be itself. Everything here is written so a caller can
// tell "this user does not exist" apart from "this agent may not become that user",
// because those have very different fixes.
package osuser

import (
	"errors"
	"path/filepath"
)

// ErrNotPermitted means the target user exists but this agent cannot switch to it.
// The fix is to run the agent as root, not to change the job.
var ErrNotPermitted = errors.New("agent is not running as root, so it cannot switch users")

// fallbackShell is what a user gets when the platform's shell lookup fails. /bin/sh
// exists everywhere the agent runs and does not pretend to know more than we do.
const fallbackShell = "/bin/sh"

// Credential is everything a child process needs to run as another user.
type Credential struct {
	Username string
	UID      uint32
	GID      uint32
	Groups   []uint32
	Home     string
	Shell    string
}

// Env returns the environment overrides that make a switched-to process look like it
// belongs to that user. Without these a job running as `deploy` would still write to
// root's HOME, which is a surprising and occasionally destructive difference.
//
// Also rewrites Go cache paths under that home. A root agent unit sets
// GOCACHE=/root/.cache/go-build; inheriting that while uid is `pi` fails with
// "mkdir /root/.cache: permission denied" on every `go build` in an install script.
func (c *Credential) Env() []string {
	if c == nil {
		return nil
	}
	home := c.Home
	if home == "" {
		home = "/tmp"
	}
	gopath := filepath.Join(home, "go")
	return []string{
		"USER=" + c.Username,
		"LOGNAME=" + c.Username,
		"HOME=" + home,
		"SHELL=" + c.Shell,
		"GOPATH=" + gopath,
		"GOMODCACHE=" + filepath.Join(gopath, "pkg", "mod"),
		"GOCACHE=" + filepath.Join(home, ".cache", "go-build"),
	}
}

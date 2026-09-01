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
	"fmt"
	"os"
	"os/user"
	"strconv"
	"syscall"
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

// Resolve looks up a username. An empty name means "stay as the agent user" and
// returns a nil credential, which every caller treats as "change nothing".
func Resolve(name string) (*Credential, error) {
	if name == "" {
		return nil, nil
	}

	u, err := user.Lookup(name)
	if err != nil {
		var unknown user.UnknownUserError
		if errors.As(err, &unknown) {
			return nil, fmt.Errorf("no such user on this server: %s", name)
		}
		return nil, err
	}

	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("unusable uid for %s: %w", name, err)
	}
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("unusable gid for %s: %w", name, err)
	}

	// Already this user: nothing to switch, and no privilege needed.
	if uint32(uid) == uint32(os.Getuid()) {
		return nil, nil
	}
	if os.Geteuid() != 0 {
		return nil, fmt.Errorf("%w (wanted %s)", ErrNotPermitted, name)
	}

	cred := &Credential{
		Username: u.Username,
		UID:      uint32(uid),
		GID:      uint32(gid),
		Home:     u.HomeDir,
		Shell:    loginShell(u.Username),
	}
	cred.Groups = supplementaryGroups(u)
	return cred, nil
}

// SysProcAttr builds the process attributes that put a child under this credential.
// Nil credential returns nil, so callers can assign it unconditionally.
//
// Setsid is deliberately NOT set here: the executor wants the child in its own process
// group for cancellation, while the terminal needs a controlling tty that the pty
// package sets up itself. Each caller layers its own choice on top.
func (c *Credential) SysProcAttr() *syscall.SysProcAttr {
	if c == nil {
		return nil
	}
	return &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid:    c.UID,
			Gid:    c.GID,
			Groups: c.Groups,
		},
	}
}

// Env returns the environment overrides that make a switched-to process look like it
// belongs to that user. Without these a job running as `deploy` would still write to
// root's HOME, which is a surprising and occasionally destructive difference.
func (c *Credential) Env() []string {
	if c == nil {
		return nil
	}
	return []string{
		"USER=" + c.Username,
		"LOGNAME=" + c.Username,
		"HOME=" + c.Home,
		"SHELL=" + c.Shell,
	}
}

// supplementaryGroups collects the user's secondary groups. Best effort: if the
// lookup fails we fall back to the primary group alone, which is more restrictive
// than the user's real access rather than less.
func supplementaryGroups(u *user.User) []uint32 {
	ids, err := u.GroupIds()
	if err != nil {
		return nil
	}
	out := make([]uint32, 0, len(ids))
	for _, g := range ids {
		n, err := strconv.ParseUint(g, 10, 32)
		if err != nil {
			continue
		}
		out = append(out, uint32(n))
	}
	return out
}

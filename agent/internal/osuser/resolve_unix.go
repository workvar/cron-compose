//go:build !windows

package osuser

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

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
	if !canSwitchTo(uint32(uid)) {
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

// Apply puts cmd under cred's uid/gid. Nil cred is a no-op (run as the agent).
func Apply(cmd *exec.Cmd, cred *Credential) {
	if cred == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Credential = &syscall.Credential{
		Uid:    cred.UID,
		Gid:    cred.GID,
		Groups: cred.Groups,
	}
}

// canSwitchTo reports whether this agent process could become uid right now.
// Dropping privileges is only possible downward, so this is true only when the agent
// runs as root; a non-root agent can only ever be itself, which Resolve checks before
// calling this.
func canSwitchTo(uid uint32) bool {
	return os.Geteuid() == 0
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

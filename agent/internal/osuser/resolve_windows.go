//go:build windows

package osuser

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"syscall"

	"github.com/croncompose/croncompose/agent/internal/platform"
)

// ErrSwitchUnsupported is returned for any run-as user other than the agent's own
// account. Windows can only start a process as another user with that user's
// password (or a logon session token), neither of which the agent holds, so jobs,
// deploys, and terminals always run as the service account.
var ErrSwitchUnsupported = errors.New("the Windows agent cannot switch users; " +
	"leave run-as empty (jobs run as the agent's service account)")

// Resolve maps a run-as name to a credential. On Windows the only names accepted
// are the agent's own account (with or without its DOMAIN\ prefix), plus "root",
// "SYSTEM", and "Administrator" when the agent is elevated, since those all mean
// "the privileged account" to a control plane written for Unix. All of them return
// a nil credential: nothing to switch.
func Resolve(name string) (*Credential, error) {
	name = strings.TrimSpace(name)
	if name == "" || isSelf(name) {
		return nil, nil
	}
	if _, err := user.Lookup(name); err != nil {
		return nil, fmt.Errorf("no such user on this server: %s", name)
	}
	return nil, fmt.Errorf("%w (wanted %s)", ErrSwitchUnsupported, name)
}

// isSelf reports whether name refers to the account the agent already runs as.
func isSelf(name string) bool {
	n := strings.ToLower(name)
	if platform.IsElevated() {
		switch n {
		case "root", "system", `nt authority\system`, "administrator":
			return true
		}
	}
	cur, err := user.Current()
	if err != nil {
		return false
	}
	full := strings.ToLower(cur.Username)
	return n == full || n == shortName(full)
}

// shortName drops a DOMAIN\ prefix: "WORKGROUP\deploy" -> "deploy".
func shortName(u string) string {
	if i := strings.LastIndex(u, `\`); i >= 0 {
		return u[i+1:]
	}
	return u
}

// SysProcAttr is nil on Windows: there is never a credential to apply.
func (c *Credential) SysProcAttr() *syscall.SysProcAttr { return nil }

// Apply is a no-op on Windows; Resolve never returns a credential.
func Apply(cmd *exec.Cmd, cred *Credential) {}

// ListUsers returns the single account a Windows agent can run things as: itself.
// An elevated agent (the LocalSystem service) is reported as uid 0 with no home,
// which the control plane already treats as "root": deploy paths stay as written
// instead of being re-homed under the system profile.
func ListUsers() ([]User, error) {
	cur, err := user.Current()
	if err != nil {
		return nil, err
	}
	u := User{
		Username:  shortName(cur.Username),
		UID:       1000,
		Home:      cur.HomeDir,
		Shell:     DefaultShell(),
		Available: true,
	}
	if platform.IsElevated() {
		u.UID = 0
		u.Home = ""
	}
	return []User{u}, nil
}

// DefaultShell is the interactive shell for web terminals: PowerShell 7 when it is
// installed, Windows PowerShell otherwise. AGENT_SHELL overrides both.
func DefaultShell() string {
	if s := strings.TrimSpace(os.Getenv("AGENT_SHELL")); s != "" {
		return s
	}
	if p, err := exec.LookPath("pwsh.exe"); err == nil {
		return p
	}
	if p, err := exec.LookPath("powershell.exe"); err == nil {
		return p
	}
	return "cmd.exe"
}

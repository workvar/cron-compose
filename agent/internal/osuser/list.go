package osuser

import "os"

// User is one OS account this agent knows about, for offering a real list of "run
// as" targets instead of a free-text guess.
type User struct {
	Username string
	UID      uint32
	Home     string
	Shell    string
	// Available reports whether this agent could switch to this user right now
	// (Resolve(Username) would succeed): either the agent already is this user, or
	// the agent runs as root. False means the account exists but picking it will
	// fail until the agent on that server is (re)installed to run as root.
	Available bool
}

// minHumanUID is the lowest uid a "real" login account gets on the distros the agent
// targets (Debian, Ubuntu, RHEL, and derivatives all default new users to 1000+).
// System/service accounts below this line are not something anyone would want an
// interactive shell as, so they are left out of the list even when they have a shell.
const minHumanUID = 1000

// noInteractiveShells marks the shells that mean "this account cannot log in", the
// same signal `chsh`/`useradd -s` use. An account with one of these is a service
// account even if its uid happens to land at or above minHumanUID.
var noInteractiveShells = map[string]bool{
	"/usr/sbin/nologin": true,
	"/sbin/nologin":     true,
	"/bin/false":        true,
	"/usr/bin/false":    true,
	"":                  true,
}

// ListUsers returns the OS accounts a web terminal session could plausibly be
// switched to: root, plus every account at or above minHumanUID that has a real
// login shell. It never errors on an individual malformed entry, it just skips it;
// the caller gets a best-effort list rather than nothing.
func ListUsers() ([]User, error) {
	accounts, err := listAccounts()
	if err != nil {
		return nil, err
	}

	agentUID := uint32(os.Getuid())
	agentIsRoot := os.Geteuid() == 0

	out := make([]User, 0, len(accounts))
	for _, a := range accounts {
		if a.UID != 0 && a.UID < minHumanUID {
			continue
		}
		if a.UID != 0 && noInteractiveShells[a.Shell] {
			continue
		}
		out = append(out, User{
			Username: a.Username,
			UID:      a.UID,
			Home:     a.Home,
			Shell:    a.Shell,
			// canSwitchTo(a.UID) is equivalent to agentIsRoot (it ignores the uid),
			// but the "already this user" case needs no privilege at all.
			Available: a.UID == agentUID || agentIsRoot,
		})
	}
	return out, nil
}

// account is the platform-neutral shape listAccounts hands back before the
// human/shell filtering above is applied.
type account struct {
	Username string
	UID      uint32
	Home     string
	Shell    string
}

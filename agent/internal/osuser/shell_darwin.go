package osuser

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// loginShell asks Directory Services for the user's shell.
//
// Reading /etc/passwd, which is what the Unix build does, is wrong on macOS: that file
// holds only a handful of stub system accounts, and every account a person actually
// logs in as lives in Open Directory instead. Parsing it would silently hand every
// real user /bin/sh, so a `run_as_user` job and a web terminal would both get a shell
// that is not the one that user gets over ssh.
//
// `dscl . -read /Users/<name> UserShell` prints "UserShell: /bin/zsh". The lookup is
// bounded because it can talk to a directory server over the network on a
// domain-joined Mac, and the agent must not block a job start on that.
func loginShell(username string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "dscl", ".", "-read", "/Users/"+username, "UserShell").Output()
	if err != nil {
		return fallbackShell
	}
	_, shell, ok := strings.Cut(string(out), ":")
	if !ok {
		return fallbackShell
	}
	if shell = strings.TrimSpace(shell); shell != "" {
		return shell
	}
	return fallbackShell
}

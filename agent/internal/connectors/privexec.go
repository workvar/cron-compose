package connectors

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// The agent stays unprivileged by default. Anything that needs root goes through
// here, and only for a fixed set of binaries: an operator opts in by granting
// passwordless sudo for exactly these, e.g. in /etc/sudoers.d/croncompose:
//
//	croncompose ALL=(root) NOPASSWD: /usr/bin/systemctl, /usr/bin/systemd-analyze, /usr/sbin/nginx, /usr/bin/tee
//
// If sudo is not available or not passwordless, privileged commands simply fail and
// the operation is reported as `unauthorized` rather than half-applied.

// privBinaries is the closed allowlist. A binary absent from this map is never run
// with sudo, no matter what the control plane asks for.
var privBinaries = map[string]bool{
	"systemctl":       true,
	"systemd-analyze": true,
	"nginx":           true,
	"tee":             true,
	"cp":              true,
	"mv":              true,
	"install":         true,
	"ufw":             true,
}

// needsPriv reports whether this process must escalate to touch root-owned state.
// Running as root already means no escalation is needed.
func needsPriv() bool { return os.Geteuid() != 0 }

// canSudo reports whether passwordless sudo is usable for the given binary. `sudo -n`
// fails immediately rather than prompting, which is what we want on a headless box.
func canSudo(ctx context.Context, bin string) bool {
	if !privBinaries[bin] {
		return false
	}
	if !has("sudo") {
		return false
	}
	path, err := exec.LookPath(bin)
	if err != nil {
		return false
	}
	// `sudo -n -l <path>` asks "may I run this without a password?" and touches nothing.
	_, err = run(ctx, "sudo", "-n", "-l", path)
	return err == nil
}

// privRun runs a command, escalating with sudo only when required, permitted, and
// possible. The returned bool reports whether the command actually ran: false means
// the agent lacked the privilege, which callers map to StatusUnauthorized.
func privRun(ctx context.Context, bin string, args ...string) (out string, ran bool, err error) {
	if !needsPriv() {
		out, err = run(ctx, bin, args...)
		return out, true, err
	}
	if !canSudo(ctx, bin) {
		return "", false, nil
	}
	full := append([]string{"-n", bin}, args...)
	out, err = run(ctx, "sudo", full...)
	return out, true, err
}

// unauthorized builds the standard refusal, naming the binary so the operator knows
// exactly which sudoers line is missing.
func unauthorized(bin string) Result {
	return fail(StatusUnauthorized,
		"agent is not root and has no passwordless sudo for "+bin+
			"; grant it in /etc/sudoers.d/croncompose to manage this connector")
}

// errNoPrivilege signals that the agent is unprivileged and has no sudoers grant for
// the binary it needed. Callers map it to StatusUnauthorized rather than a generic
// failure, because the fix is an operator action, not a retry.
var errNoPrivilege = errors.New("no privilege: agent is not root and has no passwordless sudo grant")

// trimOutput keeps command output small enough to travel in a result message without
// flooding the database. Validators are chatty on failure.
func trimOutput(s string) string {
	const max = 8000
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n... (truncated)"
}

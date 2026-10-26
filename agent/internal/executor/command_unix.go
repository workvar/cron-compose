//go:build !windows

package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/croncompose/croncompose/agent/internal/osuser"
)

// defaultPath is what a job gets when neither the job's env nor the agent's own
// environment supplies one. Without a PATH the interpreter cannot find anything, and
// "command not found" for a binary that plainly exists is a miserable first experience.
const defaultPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

// prepare builds the command for one run on Unix: the interpreter directly, or
// wrapped in a systemd-run scope when limits are requested and systemd is live.
func prepare(ctx context.Context, interp string, j Job, cred *osuser.Credential) (*prepared, error) {
	p := &prepared{cleanup: func() {}}
	prog, args := buildCommand(interp, j)
	if j.limited() && prog != "systemd-run" {
		p.notes = append(p.notes, "resource limits requested but systemd-run is not available; running unlimited")
	}
	args = withUser(prog, args, cred)
	p.cmd = exec.CommandContext(ctx, prog, args...)
	// systemd-run does its own user switching via --uid; putting a credential on the
	// systemd-run process itself would stop it from being able to talk to systemd.
	if prog != "systemd-run" {
		p.cmd.SysProcAttr = cred.SysProcAttr()
	}
	return p, nil
}

// groupHandle is unused on Unix.
type groupHandle struct{}

// attach is a no-op on Unix: limits and teardown are handled by systemd-run and
// CommandContext.
func (p *prepared) attach(_ Job) (func(), []string) { return func() {}, nil }

// buildCommand returns (program, args) for cmd.Start. If the job declares any limits
// AND systemd-run is available on PATH, the script is wrapped under a transient scope
// so the kernel enforces the caps. Otherwise the interpreter runs directly.
//
// When the job also names a user, the systemd-run path carries it as --uid/--gid,
// because a transient scope started by root would otherwise run the script as root
// regardless of what the caller set on the child process.
func buildCommand(interp string, j Job) (string, []string) {
	if !j.limited() {
		return interp, []string{"-c", j.ScriptBody}
	}
	if !systemdUsable() {
		// No usable systemd: run unlimited rather than not at all. The caller records
		// the downgrade in the run log so the operator knows the caps are not in force.
		return interp, []string{"-c", j.ScriptBody}
	}
	args := []string{"--quiet", "--scope", "--collect"}
	if j.CPUQuotaPercent > 0 {
		args = append(args, "-p", fmt.Sprintf("CPUQuota=%d%%", j.CPUQuotaPercent))
	}
	if j.MemoryMaxMB > 0 {
		args = append(args, "-p", fmt.Sprintf("MemoryMax=%dM", j.MemoryMaxMB))
	}
	if j.TasksMax > 0 {
		args = append(args, "-p", fmt.Sprintf("TasksMax=%d", j.TasksMax))
	}
	if j.IOWeight > 0 {
		args = append(args, "-p", fmt.Sprintf("IOWeight=%d", j.IOWeight))
	}
	args = append(args, "--", interp, "-c", j.ScriptBody)
	return "systemd-run", args
}

// withUser adds the user-switching flags to a systemd-run invocation. Separate from
// buildCommand so the no-limits path, which switches users through the process
// credential instead, does not have to reason about it.
func withUser(prog string, args []string, cred *osuser.Credential) []string {
	if prog != "systemd-run" || cred == nil {
		return args
	}
	// Insert before the "--" separator so the flags belong to systemd-run and not to
	// the interpreter.
	sep := len(args)
	for i, a := range args {
		if a == "--" {
			sep = i
			break
		}
	}
	inject := []string{
		"--uid=" + cred.Username,
		fmt.Sprintf("--gid=%d", cred.GID),
		"-p", "WorkingDirectory=" + cred.Home,
	}
	out := make([]string, 0, len(args)+len(inject))
	out = append(out, args[:sep]...)
	out = append(out, inject...)
	out = append(out, args[sep:]...)
	return out
}

// systemdUsable reports whether systemd-run will actually work here, which is a
// stricter question than whether the binary exists.
//
// Containers routinely ship the systemd package without running systemd as PID 1. In
// that case systemd-run is on PATH and fails at runtime with "System has not been
// booted with systemd as init system", which would turn a resource limit into a broken
// job. /run/systemd/system is the same marker systemctl itself uses to decide whether
// it is talking to a live system.
func systemdUsable() bool {
	if _, err := exec.LookPath("systemd-run"); err != nil {
		return false
	}
	st, err := os.Stat("/run/systemd/system")
	return err == nil && st.IsDir()
}

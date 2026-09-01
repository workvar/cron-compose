package executor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/croncompose/croncompose/agent/internal/osuser"
)

// defaultPath is what a job gets when neither the job's env nor the agent's own
// environment supplies one. Without a PATH the interpreter cannot find anything, and
// "command not found" for a binary that plainly exists is a miserable first experience.
const defaultPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

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

// limited reports whether any resource cap is set.
func (j Job) limited() bool {
	return j.CPUQuotaPercent > 0 || j.MemoryMaxMB > 0 || j.TasksMax > 0 || j.IOWeight > 0
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

// buildEnv assembles the child's environment.
//
// The agent deliberately does NOT hand the job its own environment wholesale: an agent
// started by pm2 or systemd carries a pile of variables a job has no business seeing,
// including anything the operator exported for the agent itself. What a job gets is a
// small, predictable base plus whatever the job declares, plus the identity of the user
// it runs as.
func buildEnv(envMap map[string]string, cred *osuser.Credential) []string {
	base := map[string]string{
		"PATH": pathOrDefault(),
		"HOME": os.Getenv("HOME"),
		"LANG": os.Getenv("LANG"),
		"TZ":   os.Getenv("TZ"),
	}
	// Running as another user replaces the identity variables wholesale.
	for _, kv := range cred.Env() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			base[k] = v
		}
	}
	// The job's own env wins over everything, including PATH, so a job can pin its
	// toolchain.
	for k, v := range envMap {
		base[k] = v
	}

	out := make([]string, 0, len(base))
	for k, v := range base {
		if v == "" {
			continue
		}
		out = append(out, k+"="+v)
	}
	return out
}

func pathOrDefault() string {
	if p := os.Getenv("PATH"); p != "" {
		return p
	}
	return defaultPath
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

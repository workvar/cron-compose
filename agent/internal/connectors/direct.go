package connectors

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/croncompose/croncompose/agent/internal/osuser"
)

// splitRunAs pulls an optional "run_as:<user>" prefix out of a lifecycle payload.
// The deploy board sends it so Start/Stop hit the same pm2 daemon the deploy used,
// which is often a login user and not the agent service account.
func splitRunAs(content []byte) (user string, rest []byte) {
	const prefix = "run_as:"
	s := string(content)
	if !strings.HasPrefix(s, prefix) {
		return "", content
	}
	user = strings.TrimSpace(s[len(prefix):])
	if i := strings.IndexAny(user, "\r\n"); i >= 0 {
		user = strings.TrimSpace(user[:i])
	}
	if strings.ContainsAny(user, " \t/\\") {
		return "", content
	}
	return user, nil
}

func isHostPM(kind string) bool {
	switch kind {
	case "pm2", "systemd", "docker":
		return true
	default:
		return false
	}
}

// directLifecycle runs start/stop/pause against pm2, systemd, or docker without a
// discovered connector row. username empty means the agent user.
func directLifecycle(ctx context.Context, kind, ref, action, username string) Result {
	if !ValidAction(action) {
		return fail(StatusUnsupported, "action not allowed: "+action)
	}
	switch kind {
	case "pm2":
		return directPM2(ctx, ref, action, username)
	case "systemd":
		return directSystemd(ctx, ref, action, username)
	case "docker":
		return directDocker(ctx, ref, action, username)
	default:
		return fail(StatusUnsupported, "no connector of kind "+kind+" on this server")
	}
}

func directPM2(ctx context.Context, ref, action, username string) Result {
	if ref == "" && action != "save" && action != "startup" && action != "enable" {
		return fail(StatusFailed, "no pm2 process given")
	}
	var args []string
	switch action {
	case "save", "enable":
		args = []string{"save"}
	case "startup":
		args = []string{"startup"}
	case "pause":
		args = []string{"sendSignal", "SIGSTOP", ref}
	case "unpause":
		args = []string{"sendSignal", "SIGCONT", ref}
	case "start", "stop", "restart", "reload", "delete", "flush":
		args = []string{action, ref}
	case "disable":
		out, err := runAs(ctx, username, "pm2", "stop", ref)
		s := step("pm2 stop "+ref, err == nil, out)
		if err != nil {
			return fail(StatusFailed, "pm2 stop failed: "+trimOutput(out), s)
		}
		saveOut, err := runAs(ctx, username, "pm2", "save")
		saveStep := step("pm2 save", err == nil, saveOut)
		if err != nil {
			return fail(StatusFailed, "stopped, but pm2 save failed: "+trimOutput(saveOut), s, saveStep)
		}
		return ok("process "+ref+" stopped and removed from the boot list", s, saveStep)
	default:
		return fail(StatusUnsupported, "unknown action: "+action)
	}
	out, err := runAs(ctx, username, "pm2", args...)
	label := "pm2 " + strings.Join(args, " ")
	s := step(label, err == nil, out)
	if err != nil {
		return fail(StatusFailed, label+" failed: "+trimOutput(out), s)
	}
	return ok(label, s)
}

func directSystemd(ctx context.Context, ref, action, username string) Result {
	if ref == "" {
		return fail(StatusFailed, "no unit given")
	}
	sysAction := action
	switch action {
	case "pause":
		sysAction = "freeze"
	case "unpause":
		sysAction = "thaw"
	}
	out, err := runAs(ctx, username, "systemctl", "--user", sysAction, ref)
	s := step("systemctl --user "+sysAction+" "+ref, err == nil, out)
	if err != nil {
		return fail(StatusFailed, "systemctl "+sysAction+" failed: "+trimOutput(out), s)
	}
	return ok("unit "+ref+" "+action, s)
}

func directDocker(ctx context.Context, ref, action, username string) Result {
	if ref == "" {
		return fail(StatusFailed, "no container given")
	}
	switch action {
	case "start", "stop", "restart", "pause", "unpause":
		out, err := runAs(ctx, username, "docker", action, ref)
		s := step("docker "+action+" "+ref, err == nil, out)
		if err != nil {
			return fail(StatusFailed, "docker "+action+" failed: "+trimOutput(out), s)
		}
		return ok("container "+ref+" "+action, s)
	default:
		return fail(StatusUnsupported, "action not supported without a docker connector: "+action)
	}
}

// runAs executes name as username (empty = agent user), resolving the binary
// through that account's login shell so nvm-installed pm2 is found.
func runAs(ctx context.Context, username, name string, args ...string) (string, error) {
	cred, err := osuser.Resolve(strings.TrimSpace(username))
	if err != nil {
		return "", err
	}
	bin := name
	if len(name) > 0 && name[0] != '/' {
		look := exec.CommandContext(ctx, "/bin/bash", "-lc", "command -v "+name)
		applyUser(look, cred)
		if out, lerr := look.Output(); lerr == nil {
			if path := lastLine(string(out)); path != "" && !strings.ContainsAny(path, " \t") {
				bin = path
			}
		}
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	applyUser(cmd, cred)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err = cmd.Run()
	return strings.TrimSpace(buf.String()), err
}

func applyUser(cmd *exec.Cmd, cred *osuser.Credential) {
	if cred == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Credential = &syscall.Credential{
		Uid: cred.UID, Gid: cred.GID, Groups: cred.Groups,
	}
	cmd.Env = append(os.Environ(),
		"HOME="+cred.Home,
		"USER="+cred.Username,
		"LOGNAME="+cred.Username,
	)
}

package connectors

import "context"

// Lifecycle drives one systemd unit. Every verb in the allowlist maps to the
// identically-named systemctl subcommand, so there is no translation table to get
// wrong; the allowlist in command.go is what stops an arbitrary subcommand from
// reaching systemctl.
func (p *systemdProvider) Lifecycle(ctx context.Context, inst Instance, ref, action string) Result {
	if !ValidAction(action) {
		return fail(StatusUnsupported, "unknown action: "+action)
	}
	if ref == "" {
		return fail(StatusFailed, "no unit given")
	}
	// Detect will not have offered this instance on a host without systemd, but a
	// command can arrive against a stale instance the control plane still remembers.
	if !systemdAvailable() {
		return noSystemd(action + " " + ref)
	}
	// pause/unpause map onto cgroup freeze/thaw; systemctl has no "pause" verb.
	sysAction := action
	switch action {
	case "pause":
		sysAction = "freeze"
	case "unpause":
		sysAction = "thaw"
	}
	out, ran, err := privRun(ctx, "systemctl", sysAction, ref)
	if !ran {
		return unauthorized("systemctl")
	}
	s := step("systemctl "+sysAction+" "+ref, err == nil, out)
	if err != nil {
		return fail(StatusFailed, "systemctl "+sysAction+" failed: "+trimOutput(out), s)
	}
	verb := action + "ed"
	if action == "pause" {
		verb = "paused"
	} else if action == "unpause" {
		verb = "unpaused"
	}
	return ok("unit "+ref+" "+verb, s, step("status", true, serviceStatus(ctx, ref)))
}

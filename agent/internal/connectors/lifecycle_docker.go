package connectors

import "context"

// Lifecycle drives one container. Docker has no reload, and enable/disable is not a
// container state at all: the closest honest equivalent is the restart policy, which
// is what decides whether the container comes back after a daemon or host restart.
//
// Docker is never run through sudo here. Access to the daemon socket is a group
// membership, not a sudoers grant, and escalating to root to reach a socket the
// operator deliberately did not grant would be the wrong default.
func (p *dockerProvider) Lifecycle(ctx context.Context, inst Instance, ref, action string) Result {
	if ref == "" {
		return fail(StatusFailed, "no container given")
	}
	switch action {
	case "start", "stop", "restart":
		out, err := run(ctx, "docker", action, ref)
		s := step("docker "+action+" "+ref, err == nil, out)
		if err != nil {
			return fail(dockerStatus(out), "docker "+action+" failed: "+trimOutput(out), s)
		}
		return ok("container "+ref+" "+action+"ed", s)

	case "enable", "disable":
		policy := "always"
		if action == "disable" {
			policy = "no"
		}
		out, err := run(ctx, "docker", "update", "--restart="+policy, ref)
		s := step("docker update --restart="+policy, err == nil, out)
		if err != nil {
			return fail(dockerStatus(out), "docker update failed: "+trimOutput(out), s)
		}
		return ok("restart policy set to "+policy, s)

	case "reload":
		return fail(StatusUnsupported, "docker containers cannot be reloaded; use restart")
	}
	return fail(StatusUnsupported, "unknown action: "+action)
}

// dockerStatus distinguishes "you may not talk to the daemon" from a real failure, so
// the UI can tell the operator to add the agent user to the docker group rather than
// showing a generic error.
func dockerStatus(out string) string {
	if containsAny(out, "permission denied", "Got permission denied", "dial unix") {
		return StatusUnauthorized
	}
	return StatusFailed
}

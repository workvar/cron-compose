package connectors

import "context"

// Lifecycle drives one pm2 process. pm2 runs as the agent's own user against that
// user's daemon, so there is nothing to escalate: if the agent can see the process it
// can act on it.
//
// enable/disable map to the process list pm2 resurrects on boot. `pm2 save` after a
// stop is what actually makes the process stay down across a reboot, which is why
// disable is stop-then-save rather than a flag.
func (p *pm2Provider) Lifecycle(ctx context.Context, inst Instance, ref, action string) Result {
	if ref == "" {
		return fail(StatusFailed, "no pm2 process given")
	}
	switch action {
	case "start", "stop", "restart", "reload":
		out, err := run(ctx, "pm2", action, ref)
		s := step("pm2 "+action+" "+ref, err == nil, out)
		if err != nil {
			return fail(StatusFailed, "pm2 "+action+" failed: "+trimOutput(out), s)
		}
		return ok("process "+ref+" "+action+"ed", s)

	case "enable":
		out, err := run(ctx, "pm2", "save")
		s := step("pm2 save", err == nil, out)
		if err != nil {
			return fail(StatusFailed, "pm2 save failed: "+trimOutput(out), s)
		}
		return ok("current process list saved; it will be resurrected on boot", s)

	case "disable":
		stopOut, err := run(ctx, "pm2", "stop", ref)
		stopStep := step("pm2 stop "+ref, err == nil, stopOut)
		if err != nil {
			return fail(StatusFailed, "pm2 stop failed: "+trimOutput(stopOut), stopStep)
		}
		saveOut, err := run(ctx, "pm2", "save")
		saveStep := step("pm2 save", err == nil, saveOut)
		if err != nil {
			return fail(StatusFailed, "stopped, but pm2 save failed so it will come back on boot: "+
				trimOutput(saveOut), stopStep, saveStep)
		}
		return ok("process "+ref+" stopped and removed from the boot list", stopStep, saveStep)
	}
	return fail(StatusUnsupported, "unknown action: "+action)
}

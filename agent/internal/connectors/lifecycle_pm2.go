package connectors

import (
	"context"
	"strings"
)

// Lifecycle drives one pm2 process, or a daemon-wide action (save / startup).
//
// enable/disable keep their historical meaning (save the boot list / stop+save).
// save and startup are the explicit pm2 commands operators expect from the UI.
func (p *pm2Provider) Lifecycle(ctx context.Context, inst Instance, ref, action string) Result {
	switch action {
	case "save", "enable":
		out, err := run(ctx, "pm2", "save")
		s := step("pm2 save", err == nil, out)
		if err != nil {
			return fail(StatusFailed, "pm2 save failed: "+trimOutput(out), s)
		}
		return ok("current process list saved; it will be resurrected on boot", s)

	case "startup":
		out, err := run(ctx, "pm2", "startup")
		s := step("pm2 startup", err == nil, out)
		if err != nil {
			return fail(StatusFailed, "pm2 startup failed (may need sudo once on this host): "+trimOutput(out), s)
		}
		msg := strings.TrimSpace(out)
		if msg == "" {
			msg = "pm2 startup configured"
		}
		return ok(msg, s)
	}

	if ref == "" {
		return fail(StatusFailed, "no pm2 process given")
	}

	switch action {
	case "start", "stop", "restart", "reload", "delete":
		out, err := run(ctx, "pm2", action, ref)
		s := step("pm2 "+action+" "+ref, err == nil, out)
		if err != nil {
			return fail(StatusFailed, "pm2 "+action+" failed: "+trimOutput(out), s)
		}
		verb := action + "ed"
		if action == "delete" {
			verb = "deleted"
		}
		return ok("process "+ref+" "+verb, s)

	case "flush":
		out, err := run(ctx, "pm2", "flush", ref)
		s := step("pm2 flush "+ref, err == nil, out)
		if err != nil {
			return fail(StatusFailed, "pm2 flush failed: "+trimOutput(out), s)
		}
		return ok("logs flushed for "+ref, s)

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

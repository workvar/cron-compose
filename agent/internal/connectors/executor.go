package connectors

import (
	"context"
	"log/slog"
	"time"
)

// commandTimeout bounds a single connector command on the agent side. It is shorter
// than the control plane's wait so the agent almost always answers, even to say it
// gave up, rather than leaving the caller to time out on silence.
const commandTimeout = 25 * time.Second

// Executor runs one Command against the right provider and returns a Result. It is
// the agent-side counterpart of the control plane's SendConnectorCommand.
type Executor struct {
	log *slog.Logger
	reg *Registry
}

// NewExecutor wires an executor to a provider registry.
func NewExecutor(log *slog.Logger, reg *Registry) *Executor {
	return &Executor{log: log, reg: reg}
}

// Execute dispatches a command. It never returns an error: a failure is a Result with
// a non-success status, because that is what has to travel back over the wire.
//
// Every path is bounded by commandTimeout, so a hung `systemctl restart` cannot pin
// the goroutine forever.
func (e *Executor) Execute(ctx context.Context, cmd Command) Result {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	prov, inst, found := e.reg.Resolve(ctx, cmd.Kind, cmd.Instance)
	if !found {
		return fail(StatusUnsupported, "no connector of kind "+cmd.Kind+" on this server")
	}

	switch cmd.Op {
	case "status", "list":
		return e.list(ctx, prov, inst)

	case "lifecycle":
		actor, okAssert := prov.(Actor)
		if !okAssert {
			return fail(StatusUnsupported, cmd.Kind+" does not support lifecycle actions")
		}
		if !ValidAction(cmd.Action) {
			return fail(StatusUnsupported, "action not allowed: "+cmd.Action)
		}
		if !inst.Caps.CanLifecycle {
			return fail(StatusUnauthorized,
				"this agent cannot drive "+cmd.Kind+" (discovery reported it as not manageable)")
		}
		return actor.Lifecycle(ctx, inst, cmd.Ref, cmd.Action)

	case "read":
		cm, okAssert := prov.(ConfigManager)
		if !okAssert {
			return fail(StatusUnsupported, cmd.Kind+" does not manage config files")
		}
		return cm.ReadConfig(ctx, inst, cmd.Ref)

	case "validate":
		cm, okAssert := prov.(ConfigManager)
		if !okAssert {
			return fail(StatusUnsupported, cmd.Kind+" does not manage config files")
		}
		return cm.Validate(ctx, inst, cmd.Ref, cmd.Content)

	case "apply", "rollback":
		cm, okAssert := prov.(ConfigManager)
		if !okAssert {
			return fail(StatusUnsupported, cmd.Kind+" does not manage config files")
		}
		if !inst.Caps.CanEdit {
			return fail(StatusUnauthorized,
				"this agent cannot edit "+cmd.Kind+" config (the files are not writable and there is no sudo grant)")
		}
		// A rollback is an apply of previously-captured bytes. It skips the checksum
		// precheck on purpose: the whole point is to overwrite whatever is there now.
		if cmd.Op == "rollback" {
			cmd.BaseChecksum = ""
		}
		return applyConfig(ctx, cm, inst, cmd)
	}

	return fail(StatusUnsupported, "unknown op: "+cmd.Op)
}

// list returns the provider's current resources as a fresh read, so the UI can refresh
// one connector without waiting for the next five-minute discovery sweep.
func (e *Executor) list(ctx context.Context, prov Provider, inst Instance) Result {
	res := prov.Resources(ctx, inst)
	return Result{
		Status:  StatusSucceeded,
		Message: "listed resources",
		Steps:   []Step{step("list", true, itoa(len(res))+" resources")},
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

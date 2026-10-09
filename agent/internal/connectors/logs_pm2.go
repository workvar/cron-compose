package connectors

import (
	"context"
	"fmt"
)

// Logs returns recent output for one pm2 process. --nostream exits after the
// buffered lines so the command cannot hang the agent.
func (p *pm2Provider) Logs(ctx context.Context, inst Instance, ref string, lines int) Result {
	if ref == "" {
		return fail(StatusFailed, "no pm2 process given")
	}
	if lines <= 0 {
		lines = 200
	}
	if lines > 1000 {
		lines = 1000
	}
	out, err := run(ctx, "pm2", "logs", ref, "--lines", fmt.Sprintf("%d", lines), "--nostream", "--raw")
	s := step("pm2 logs "+ref, err == nil, trimOutput(out))
	if err != nil {
		return fail(StatusFailed, "pm2 logs failed: "+trimOutput(out), s)
	}
	return Result{
		Status:  StatusSucceeded,
		Message: "pm2 logs " + ref,
		Content: []byte(out),
		Steps:   []Step{s},
	}
}

package connectors

import (
	"context"
	"fmt"
)

// Logs returns recent container output.
func (p *dockerProvider) Logs(ctx context.Context, inst Instance, ref string, lines int) Result {
	if ref == "" {
		return fail(StatusFailed, "no container given")
	}
	if lines <= 0 {
		lines = 200
	}
	if lines > 1000 {
		lines = 1000
	}
	out, err := run(ctx, "docker", "logs", "--tail", fmt.Sprintf("%d", lines), ref)
	s := step("docker logs "+ref, err == nil, trimOutput(out))
	if err != nil {
		return fail(StatusFailed, "docker logs failed: "+trimOutput(out), s)
	}
	return Result{
		Status:  StatusSucceeded,
		Message: "docker logs " + ref,
		Content: []byte(out),
		Steps:   []Step{s},
	}
}

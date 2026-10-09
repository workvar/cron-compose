package connectors

import (
	"context"
	"fmt"
	"strings"
)

// Logs returns recent journal lines for a user unit.
func (p *systemdProvider) Logs(ctx context.Context, inst Instance, ref string, lines int) Result {
	if ref == "" {
		return fail(StatusFailed, "no systemd unit given")
	}
	if lines <= 0 {
		lines = 200
	}
	if lines > 1000 {
		lines = 1000
	}
	unit := ref
	if !strings.HasSuffix(unit, ".service") && !strings.Contains(unit, ".") {
		unit = unit + ".service"
	}
	out, err := run(ctx, "journalctl", "--user", "-u", unit, "-n", fmt.Sprintf("%d", lines), "--no-pager")
	s := step("journalctl -u "+unit, err == nil, trimOutput(out))
	if err != nil {
		// Fall back to system units when --user has nothing.
		out2, err2 := run(ctx, "journalctl", "-u", unit, "-n", fmt.Sprintf("%d", lines), "--no-pager")
		s2 := step("journalctl -u "+unit+" (system)", err2 == nil, trimOutput(out2))
		if err2 != nil {
			return fail(StatusFailed, "journalctl failed: "+trimOutput(out), s, s2)
		}
		return Result{
			Status:  StatusSucceeded,
			Message: "journalctl " + unit,
			Content: []byte(out2),
			Steps:   []Step{s, s2},
		}
	}
	return Result{
		Status:  StatusSucceeded,
		Message: "journalctl " + unit,
		Content: []byte(out),
		Steps:   []Step{s},
	}
}
